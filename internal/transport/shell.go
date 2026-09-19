package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// startShell turns a connected client into a live interactive session. It is
// shared by the ssh and ssm-ssh kinds.
func startShell(
	client *ssh.Client,
	size Size,
	onOutput func([]byte),
	onExit func(error),
) (Session, error) {
	failWith := func(what string, err error) (Session, error) {
		client.Close()
		return nil, fmt.Errorf("%s: %w", what, err)
	}

	remoteSession, err := client.NewSession()
	if err != nil {
		return failWith("open ssh session have error: ", err)
	}

	standardInput, err := remoteSession.StdinPipe()
	if err != nil {
		remoteSession.Close()
		return failWith("attach stdin", err)
	}

	standardOutput, err := remoteSession.StdoutPipe()
	if err != nil {
		remoteSession.Close()
		return failWith("attach stdout", err)
	}

	standardError, err := remoteSession.StderrPipe()
	if err != nil {
		remoteSession.Close()
		return failWith("attch stderr", err)
	}

	// Without a pty the remote shell prints no prompt, no colour, and refuses
	// to run anything full-screen such as top or vim.
	terminalModes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	if err := remoteSession.RequestPty(
		"xterm-256color", int(size.Rows), int(size.Cols), terminalModes,
	); err != nil {
		remoteSession.Close()
		return failWith("request a terminal: ", err)
	}
	if err := remoteSession.Shell(); err != nil {
		remoteSession.Close()
		return failWith("start the remote shell: ", err)
	}

	session := &sshSession{
		client:        client,
		remoteSession: remoteSession,
		standardInput: standardInput,
	}

	go forwardStream(standardOutput, onOutput)
	go forwardStream(standardError, onOutput)
	go func() {
		// Wait returns when the remote shell ends, for any reason.
		exitErr := remoteSession.Wait()

		// A non-zero exit status is how a shell says goodbye after `exit 1`.
		// That is the session ending normally, not the transport failing.
		var exitStatus *ssh.ExitError
		if errors.As(exitErr, &exitStatus) {
			exitErr = nil
		}

		var noExitStatus *ssh.ExitMissingError
		if errors.As(exitErr, &noExitStatus) {
			exitErr = errors.New(
				"the server closed the connection without ending the session " +
					"- usually an idle timeout, a reboot, or network dropping")
		}

		session.Close()
		onExit(exitErr)
	}()

	return session, nil
}

func forwardStream(stream io.Reader, onOutput func([]byte)) {
	buffer := make([]byte, 32*1024)
	for {
		bytesRead, err := stream.Read(buffer)
		if bytesRead > 0 {
			// buffer is reused on the next iteration, so the callback gets its
			// own copy. Handing it buffer[:bytesRead] would let a slow consumer
			// read bytes that have already been overwritten.
			chunk := make([]byte, bytesRead)
			copy(chunk, buffer[:bytesRead])
			onOutput(chunk)
		}
		if err != nil {
			return
		}
	}
}

type sshSession struct {
	client        *ssh.Client
	remoteSession *ssh.Session
	standardInput io.WriteCloser
	closeOnce     sync.Once //// goroutine that notices the remote shell ended.
}

func (session *sshSession) Write(payload []byte) (int, error) {
	return session.standardInput.Write(payload)
}

func (session *sshSession) Resize(size Size) error {
	return session.remoteSession.WindowChange(int(size.Rows), int(size.Cols))
}

func (session *sshSession) Close() error {
	var closeErr error
	session.closeOnce.Do(func() {
		session.standardInput.Close()
		session.remoteSession.Close()
		closeErr = session.client.Close()
	})
	return closeErr
}

// droppedBeforeAuth reports whether the server hung up before authentication
// began, rather than refusing the credentials.
//
// The distinction is what makes retrying safe. A connection turned away by
// MaxStartups never got as far as the version exchange, so no username and no
// password ever left this machine and trying again costs the account nothing.
// A rejected password is the opposite: repeating it is how accounts get locked.
//
// x/crypto reports all of this as "EOF", because it is looking for a line
// beginning "SSH-" and sshd sends the words "Exceeded MaxStartups" instead.
func droppedBeforeAuth(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}

	lowered := strings.ToLower(err.Error())
	return strings.Contains(lowered, "eof") ||
		strings.Contains(lowered, "connection reset by peer") ||
		strings.Contains(lowered, "broken pipe") ||
		strings.Contains(lowered, "exceeded maxstartups")
}

// explainHandshakeFailure turns x/crypto's handshake errors into something that
// says what to do next.
//
// "ssh: handshake failed" covers everything from the version exchange to the
// last authentication attempt, so problems with nothing in common arrive under
// the same two words. Worse, the most common one arrives as the bare text
// "EOF", which reads like a bug in this app when it is the server hanging up.
//
// A pure function, so the mapping can be tested without a server.
func explainHandshakeFailure(address string, err error) string {
	host := address
	if hostOnly, _, splitErr := net.SplitHostPort(address); splitErr == nil {
		host = hostOnly
	}

	lowered := strings.ToLower(err.Error())

	switch {
	case droppedBeforeAuth(err):
		return fmt.Sprintf(
			"%s refused the connection before the SSH handshake started, %d times "+
				"in a row.\n\n"+
				"sshd does this when too many half-finished logins are already in "+
				"flight: its MaxStartups setting turns away a share of new "+
				"connections at random rather than queueing them, answering with "+
				"\"Exceeded MaxStartups\" instead of a version banner. On a machine "+
				"reachable from the internet that queue is usually full of bots "+
				"rather than of you, so the refusals come and go for no reason you "+
				"can see.\n\n"+
				"mssh already retried. If it keeps happening, whoever runs %s can "+
				"raise MaxStartups in /etc/ssh/sshd_config, or put fail2ban in "+
				"front of port %s to keep the bots out of the queue.",
			host, preAuthDropAttempts, host, portOf(address))

	case strings.Contains(lowered, "unable to authenticate"):
		return fmt.Sprintf(
			"%s refused the credentials.\n\n"+
				"Check the username character for character — macOS likes to "+
				"capitalise the first letter of a field, and `Deploy` is not "+
				"`deploy`. If this connection uses a password, the server may also "+
				"have password logins turned off.\n\n%s",
			host, err)

	case strings.Contains(lowered, "no common algorithm"),
		strings.Contains(lowered, "no common algo"):
		return fmt.Sprintf(
			"%s and mssh could not agree on an encryption algorithm — the server "+
				"is probably old enough that Go's SSH library no longer speaks to "+
				"it. Use the System SSH kind for this one: it runs the real ssh "+
				"command, which still supports the older algorithms.\n\n%s",
			host, err)

	default:
		return fmt.Sprintf("ssh handshake with %s: %v", address, err)
	}
}

// portOf pulls the port back out of a host:port, for printing a command the
// user can paste. An address without one falls back to the default.
func portOf(address string) string {
	if _, port, err := net.SplitHostPort(address); err == nil {
		return port
	}
	return "22"
}

// handshake runs the client handshake under a deadline.
//
// ClientConfig.Timeout only applies to ssh.Dial, which we do not use, so
// without this a hung peer blocks forever.
func handshake(
	connection net.Conn,
	address string,
	clientConfig *ssh.ClientConfig,
	limit time.Duration,
) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	// A conn that cannot take a deadline simply goes without one.
	_ = connection.SetDeadline(time.Now().Add(limit))

	sshConnection, channels, requests, err := ssh.NewClientConn(connection, address, clientConfig)
	if err != nil {
		return nil, nil, nil, err
	}

	// Clear it. The session is long-lived, and a deadline left in place would
	// kill it the moment it expires.
	_ = connection.SetDeadline(time.Time{})

	return sshConnection, channels, requests, nil
}
