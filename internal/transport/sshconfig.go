package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
)

// sshConfigDialer runs the system ssh command against a Host alias.
//
// Everything is OpenSSH's business here: which user, which key, ProxyCommand,
// jump hosts, known_hosts, and its own yes/no prompt for an unknown key. mssh
// stores nothing but the alias, and asks nothing the user has already told
// ~/.ssh/config.
type sshConfigDialer struct{}

var _ Dialer = sshConfigDialer{}

func (sshConfigDialer) Name() string { return "System SSH" }

func (sshConfigDialer) Preflight(config Config) error {
	if strings.TrimSpace(config.Target) == "" {
		return fmt.Errorf("this connection has no ssh host alias")
	}

	ensureUsableEnvironment()
	if _, err := exec.LookPath("ssh"); err != nil {
		return fmt.Errorf("the ssh command is not installed, or not on PATH")
	}

	// Nothing else is checked on purpose. Whether the alias exists, whether the
	// key is right, whether the host answers — ssh reports all of that itself,
	// in the terminal, in the words the user already knows.
	return nil
}

func (sshConfigDialer) Dial(
	dialContext context.Context,
	config Config,
	size Size,
	onOutput func([]byte),
	onExit func(error),
) (Session, error) {
	alias := strings.TrimSpace(config.Target)
	controlDirectory, controlPath := sharedConnectionPath(controlPathBases())

	session := &sshConfigSession{
		alias: alias,
		// A Host block whose ProxyCommand runs aws ssm start-session needs the
		// workspace's keys exactly as much as the ssm kind does.
		environment:      awsEnvironment(pseudoTerminalEnvironment(), config.AWSCredentials, config.AWSRegion),
		controlDirectory: controlDirectory,
		controlPath:      controlPath,
	}

	command := exec.CommandContext(dialContext, "ssh", session.terminalArguments()...)
	command.Env = session.environment

	terminal, err := startPTYProcess(command, size, onOutput,
		func(exitErr error) {
			// The shell ended on its own: the shared connection, and any file
			// client on it, go with it.
			session.release()
			onExit(exitErr)
		},
		func(output string) string {
			return explainSSHCommandFailure(alias, output)
		})
	if err != nil {
		session.release()
		return nil, err
	}
	if controlPath == "" {
		// Nothing to share, so nothing to browse files over: a plain terminal,
		// which says as much when asked for files.
		return terminal, nil
	}
	session.Session = terminal
	return session, nil
}

// systemSSHOptions go first on every ssh command line this kind runs. Empty in
// the app; tests point ssh at a config file of their own with it, since ssh
// finds ~/.ssh/config through the password database rather than $HOME.
var systemSSHOptions []string

// sshConfigSession is the terminal of an ssh-config connection, plus the files
// of the machine it is logged in to.
//
// mssh holds no SSH connection here - ssh does - so files travel over ssh's
// own: the terminal's ssh is started as a ControlMaster, and the file client is
// a second ssh that runs the sftp subsystem through the master's socket. One
// login, one ProxyCommand, one tunnel, whatever ~/.ssh/config says.
//
// ControlPersist is what lets the terminal end when its shell does. Without
// it, the master is the terminal's own ssh, and it stays open as long as any
// other channel does: type exit with the Files drawer open and nothing would
// happen. With it, ssh moves the connection to a background process once it
// has logged in and carries on in the terminal as an ordinary client of it.
type sshConfigSession struct {
	// Session is the terminal: ssh under its pty.
	Session

	alias       string
	environment []string
	// controlDirectory is private to this session and holds the socket at
	// controlPath; both are removed when the session ends.
	controlDirectory string
	controlPath      string

	files       sharedSFTP
	releaseOnce sync.Once
}

// controlPersist is how long the background connection outlives its last
// client. It is told to exit when the session ends; this only bounds how long
// it lingers if mssh itself is killed first.
const controlPersist = "10"

// terminalArguments is the command line for the interactive ssh: the alias,
// and, when there is somewhere to put the socket, the options that make it a
// master other ssh commands can share.
func (session *sshConfigSession) terminalArguments() []string {
	arguments := slices.Clone(systemSSHOptions)
	if session.controlPath != "" {
		arguments = append(arguments,
			"-o", "ControlMaster=yes",
			"-o", "ControlPersist="+controlPersist,
			"-S", escapeControlPath(session.controlPath))
	}
	return append(arguments, session.alias)
}

// clientArguments are the options every other ssh this session runs shares:
// connect through the master and never become one, and leave behind what the
// Host block asks of an interactive login.
func (session *sshConfigSession) clientArguments() []string {
	return append(slices.Clone(systemSSHOptions),
		"-S", escapeControlPath(session.controlPath),
		"-o", "ControlMaster=no",
		// The master already set up the block's LocalForwards; asking again
		// through it only fails with "address already in use".
		"-o", "ClearAllForwardings=yes",
		"-o", "PermitLocalCommand=no",
	)
}

// sftpArguments is the command line for the file client: the sftp subsystem,
// with no terminal, and never a prompt - there is no one to answer it.
func (session *sshConfigSession) sftpArguments() []string {
	return append(session.clientArguments(),
		"-o", "BatchMode=yes",
		// A RemoteCommand in the Host block would collide with the subsystem.
		"-o", "RemoteCommand=none",
		"-T", "-s", session.alias, "sftp")
}

// SFTP returns the session's file client, opening it on the first call.
func (session *sshConfigSession) SFTP() (*sftp.Client, error) {
	return session.files.get(func() (*sftp.Client, error) {
		return openSystemSFTP(session.sftpArguments(), session.environment)
	})
}

func (session *sshConfigSession) Close() error {
	// The file client first: its channel would keep the master busy.
	session.files.close()
	closeErr := session.Session.Close()
	session.release()
	return closeErr
}

// release closes the file client, stops the shared connection and removes its
// socket. It runs once, from Close or from the end of the shell, whichever
// comes first.
func (session *sshConfigSession) release() {
	session.releaseOnce.Do(func() {
		session.files.close()
		if session.controlPath == "" {
			return
		}
		stopMaster(session.clientArguments(), session.alias, session.environment)
		_ = os.RemoveAll(session.controlDirectory)
	})
}

// stopMaster asks the background connection to exit. It may well be gone
// already - it leaves on its own once idle - so its answer does not matter.
func stopMaster(clientArguments []string, alias string, environment []string) {
	stopContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	command := exec.CommandContext(stopContext, "ssh", append(clientArguments, "-O", "exit", alias)...)
	command.Env = environment
	_ = command.Run()
}

// openSystemSFTP runs ssh with the given arguments and speaks SFTP over its
// standard input and output. A variable so that tests can count the opens.
var openSystemSFTP = func(arguments []string, environment []string) (*sftp.Client, error) {
	command := exec.Command("ssh", arguments...)
	command.Env = environment
	complaints := &tailBuffer{limit: 4096}
	command.Stderr = complaints

	input, err := command.StdinPipe()
	if err != nil {
		return nil, err
	}
	// A pipe of our own rather than StdoutPipe, which Wait closes: Wait runs
	// as soon as ssh starts, while the client is still reading.
	output, outputWriter, err := os.Pipe()
	if err != nil {
		_ = input.Close()
		return nil, err
	}
	command.Stdout = outputWriter
	if err := command.Start(); err != nil {
		_ = input.Close()
		_ = output.Close()
		_ = outputWriter.Close()
		return nil, fmt.Errorf("start ssh: %w", err)
	}
	_ = outputWriter.Close()

	exited := make(chan struct{})
	go func() {
		_ = command.Wait()
		close(exited)
	}()

	pipe := &processPipe{input: input, output: output, process: command.Process, exited: exited}
	client, err := sftp.NewClientPipe(output, pipe)
	if err != nil {
		_ = pipe.Close()
		// ssh's own words beat "EOF": the master is gone, the host refused, ...
		if message := lastLines(complaints.string(), 3); message != "" {
			return nil, errors.New(message)
		}
		return nil, err
	}
	return client, nil
}

// processPipe is the writing end of an SFTP client that talks to a child
// process. Closing it ends the process: politely first, by closing its input,
// then by force, since the client's Close waits until the output ends.
type processPipe struct {
	input     io.WriteCloser
	output    *os.File
	process   *os.Process
	exited    <-chan struct{}
	closeOnce sync.Once
}

// processPipeGrace is how long the child has to leave after its input closes.
const processPipeGrace = 2 * time.Second

func (pipe *processPipe) Write(payload []byte) (int, error) {
	return pipe.input.Write(payload)
}

func (pipe *processPipe) Close() error {
	pipe.closeOnce.Do(func() {
		_ = pipe.input.Close()
		select {
		case <-pipe.exited:
		case <-time.After(processPipeGrace):
			_ = pipe.process.Kill()
			<-pipe.exited
		}
		_ = pipe.output.Close()
	})
	return nil
}

// maxControlPathLength leaves room for what ssh adds while it creates the
// socket - a dot and sixteen random characters - inside the 104 bytes macOS
// allows a socket path. ssh refuses to start at all when the path is too long.
const maxControlPathLength = 104 - 1 - 17

// controlPathBases are the directories tried for the socket, in order. The
// usual temporary directory is private to the user on macOS but can be long;
// /tmp is short everywhere it exists.
func controlPathBases() []string {
	if runtime.GOOS == "windows" {
		// Windows' ssh cannot share a connection at all.
		return nil
	}
	return []string{os.TempDir(), "/tmp"}
}

// sharedConnectionPath makes a directory only this user can enter and returns
// it with the socket path inside it, in the first base where the path is short
// enough. Both are empty when there is nowhere: the session then simply goes
// without files.
func sharedConnectionPath(bases []string) (directory string, socket string) {
	for _, base := range bases {
		created, err := os.MkdirTemp(base, "mssh-")
		if err != nil {
			continue
		}
		socket := filepath.Join(created, "ssh")
		if len(socket) <= maxControlPathLength {
			return created, socket
		}
		_ = os.Remove(created)
	}
	return "", ""
}

// escapeControlPath protects a path from ssh's %-token expansion.
func escapeControlPath(path string) string {
	return strings.ReplaceAll(path, "%", "%%")
}

// explainSSHCommandFailure adds the one piece of context ssh cannot know: that
// its settings came from ~/.ssh/config rather than from this app.
func explainSSHCommandFailure(alias string, output string) string {
	lowered := strings.ToLower(output)

	switch {
	// A ProxyCommand that shells out to the AWS CLI fails here, and the reason
	// is not something the user can guess: they can see the exports in their
	// shell profile, so "no credentials" reads like a bug in this app.
	case strings.Contains(lowered, "unable to locate credentials"),
		strings.Contains(lowered, "credentials not found"),
		strings.Contains(lowered, "the config profile") && strings.Contains(lowered, "could not be found"):
		return fmt.Sprintf(
			"the ProxyCommand for %q could not find AWS credentials.\n\n"+
				"mssh runs your login shell at startup and hands its environment "+
				"to ssh, so exports in ~/.zshrc should reach the ProxyCommand. "+
				"Check that `aws sts get-caller-identity` works in a terminal: if "+
				"it does, run `aws configure` so the credentials live in "+
				"~/.aws/credentials, which any process can read however it was "+
				"started.", alias)

	case strings.Contains(lowered, "could not resolve hostname"):
		return fmt.Sprintf(
			"ssh does not know how to reach %q — check that ~/.ssh/config has a "+
				"Host block matching it", alias)

	case strings.Contains(lowered, "permission denied"):
		return fmt.Sprintf(
			"%s refused the credentials ssh offered — check User and IdentityFile "+
				"in its Host block in ~/.ssh/config", alias)

	case strings.Contains(lowered, "closed by remote host"),
		strings.Contains(lowered, "connection reset byt peer"),
		strings.Contains(lowered, "broken pipe"),
		strings.Contains(lowered, "not responding"):
		return fmt.Sprintf("%s closed the connection - usually an idle timeout, a reboot, or the network dropping", alias)
	default:
		// ssh's own message is usually the clearest thing available.
		return lastLines(output, 3)
	}
}
