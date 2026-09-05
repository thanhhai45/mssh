package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
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
