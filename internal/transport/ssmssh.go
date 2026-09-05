package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// defaultSSHDocument is the SSM document that turns a session into a plain
// byte pipe suitable for carrying SSH, instead of an interactive shell.
const defaultSSHDocument = "AWS-StartSSHSession"

// ssmSSHDialer runs a real SSH handshake through an SSM tunnel, so the session
// lands as the user's own account rather than ssm-user, and scp keeps working.
type ssmSSHDialer struct{}

var _ Dialer = ssmSSHDialer{}

func (ssmSSHDialer) Name() string { return "SSH over SSM" }

func (ssmSSHDialer) Preflight(config Config) error {
	if config.Target == "" {
		return fmt.Errorf("this connection has no instance id")
	}
	if err := requireAWSTools(); err != nil {
		return err
	}
	if err := checkAWSCredentials(config.AWSProfile, config.AWSRegion); err != nil {
		return err
	}
	// The SSH half has its own requirements: a username, and usable
	// credentials. Reusing the ssh dialer's checks keeps them in one place.
	return sshDialer{}.Preflight(config)
}

func (ssmSSHDialer) Dial(
	dialContext context.Context,
	config Config,
	size Size,
	onOutput func([]byte),
	onExit func(error),
) (Session, error) {
	tunnel, err := startSSMTunnel(dialContext, config)
	if err != nil {
		return nil, err
	}

	authMethods, releaseAuth, err := sshAuthMethods(config)
	if err != nil {
		tunnel.Close()
		return nil, err
	}
	defer releaseAuth()

	hostKeyCallback, err := hostKeyCallbackFor(config.KnownHostsPath)
	if err != nil {
		tunnel.Close()
		return nil, err
	}

	clientConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         30 * time.Second,
	}

	// The instance id is what ssh sees as the host name here, so that is the
	// name known_hosts is searched for. It has to carry a port: knownhosts
	// runs SplitHostPort on this too.
	sshConnection, channels, requests, err := handshake(
		tunnel, tunnel.address, clientConfig, 30*time.Second)
	if err != nil {
		// Whatever AWS printed on stderr is reported alongside the ssh error
		// rather than instead of it: stderr also carries harmless warnings, and
		// those must not be mistaken for the reason the handshake failed.
		awsComplaint := strings.TrimSpace(tunnel.problems.string())
		tunnel.Close()

		if awsComplaint != "" {
			return nil, fmt.Errorf(
				"ssh handshake through the SSM tunnel to %s: %w\n\nthe tunnel also said: %s",
				config.Target, err, explainSSMFailure(config.Target, awsComplaint))
		}
		return nil, fmt.Errorf("ssh handshake through the SSM tunnel to %s: %w",
			config.Target, err)
	}

	client := ssh.NewClient(sshConnection, channels, requests)

	// If the tunnel dies mid-session, ssh only sees EOF. The reason is in the
	// tunnel's stderr, so attach it on the way out.
	return startShell(client, size, onOutput, func(exitErr error) {
		if exitErr != nil {
			if complaint := strings.TrimSpace(tunnel.problems.string()); complaint != "" {
				exitErr = fmt.Errorf("%w — the tunnel said: %s",
					exitErr, explainSSMFailure(config.Target, complaint))
			}
		}
		onExit(exitErr)
	})
}

// tunnelOptions are the rarely-changed settings kept in the connection's extra
// JSON rather than given a column of their own.
type tunnelOptions struct {
	DocumentName string `json:"documentName"`
}

// startSSMTunnel launches `aws ssm start-session` as a pure byte pipe.
func startSSMTunnel(dialContext context.Context, config Config) (*processConn, error) {
	portNumber := config.Port
	if portNumber == 0 {
		portNumber = 22
	}

	documentName := defaultSSHDocument
	if config.Extra != "" {
		var options tunnelOptions
		// A malformed extra should not stop a connection: fall back to the
		// default rather than failing on something the user cannot see.
		if err := json.Unmarshal([]byte(config.Extra), &options); err == nil {
			if options.DocumentName != "" {
				documentName = options.DocumentName
			}
		}
	}

	arguments := append([]string{
		"ssm", "start-session",
		"--target", config.Target,
		"--document-name", documentName,
		"--parameters", fmt.Sprintf("portNumber=%d", portNumber),
	}, awsFlags(config.AWSProfile, config.AWSRegion)...)

	command := exec.CommandContext(dialContext, "aws", arguments...)
	command.Env = os.Environ()

	// No pty here, unlike the plain ssm kind. This process is carrying the SSH
	// wire protocol, and a terminal would translate newlines and interpret
	// control characters — corrupting it.
	writer, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("attach tunnel stdin: %w", err)
	}
	reader, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("attach tunnel stdout: %w", err)
	}

	// stderr is kept separate on purpose: it carries AWS's error messages, and
	// must never reach the SSH byte stream.
	problems := &tailBuffer{limit: 4096}
	command.Stderr = problems

	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start the SSM tunnel: %w", err)
	}

	return &processConn{
		command:  command,
		writer:   writer,
		reader:   reader,
		problems: problems,
		address:  net.JoinHostPort(config.Target, strconv.Itoa(portNumber)),
	}, nil
}

// processConn presents a child process's stdin and stdout as a net.Conn, so an
// SSH handshake can run over them. It is the Go equivalent of ssh's
// ProxyCommand.
type processConn struct {
	command   *exec.Cmd
	writer    io.WriteCloser
	reader    io.ReadCloser
	problems  *tailBuffer
	address   string
	closeOnce sync.Once
}

func (connection *processConn) Read(payload []byte) (int, error) {
	return connection.reader.Read(payload)
}

func (connection *processConn) Write(payload []byte) (int, error) {
	return connection.writer.Write(payload)
}

func (connection *processConn) Close() error {
	var closeErr error
	connection.closeOnce.Do(func() {
		connection.writer.Close()
		connection.reader.Close()
		if connection.command.Process != nil {
			connection.command.Process.Kill()
		}
		// Wait reaps the process and waits for the goroutine copying stderr.
		// After Kill it always reports "signal: killed", which is the expected
		// outcome here rather than a failure worth passing on.
		waitErr := connection.command.Wait()
		var exited *exec.ExitError
		if waitErr != nil && !errors.As(waitErr, &exited) {
			closeErr = waitErr
		}
	})
	return closeErr
}

// The rest of net.Conn has no meaning for a pair of pipes. Satisfying an
// interface does not oblige every method to do something.
func (connection *processConn) LocalAddr() net.Addr  { return tunnelAddr{connection.address} }
func (connection *processConn) RemoteAddr() net.Addr { return tunnelAddr{connection.address} }

// StdinPipe and StdoutPipe hand back *os.File, and those support deadlines on
// pipes — so these are real implementations, not stubs. The handshake relies
// on them to avoid hanging forever on a tunnel that goes quiet.
func (connection *processConn) SetDeadline(deadline time.Time) error {
	if err := connection.SetReadDeadline(deadline); err != nil {
		return err
	}
	return connection.SetWriteDeadline(deadline)
}

func (connection *processConn) SetReadDeadline(deadline time.Time) error {
	if file, ok := connection.reader.(*os.File); ok {
		return file.SetReadDeadline(deadline)
	}
	return nil
}

func (connection *processConn) SetWriteDeadline(deadline time.Time) error {
	if file, ok := connection.writer.(*os.File); ok {
		return file.SetWriteDeadline(deadline)
	}
	return nil
}

type tunnelAddr struct{ address string }

func (address tunnelAddr) Network() string { return "ssm" }
func (address tunnelAddr) String() string  { return address.address }
