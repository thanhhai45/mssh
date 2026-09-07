package transport

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/creack/pty"
)

// startPTYProcess runs a command under a local pseudo-terminal and wires it to
// the session callbacks.
//
// It is shared by every kind that works by driving another program instead of
// speaking a protocol itself: the SSM kind drives `aws`, the ssh-config kind
// drives `ssh`.
//
// explain turns whatever the program printed on its way out into something a
// person can act on. It may return an empty string to leave the original error
// alone.
func startPTYProcess(
	command *exec.Cmd,
	size Size,
	onOutput func([]byte),
	onExit func(error),
	explain func(output string) string,
) (Session, error) {
	terminal, err := pty.StartWithSize(command, &pty.Winsize{
		Cols: size.Cols,
		Rows: size.Rows,
	})
	if err != nil {
		return nil, fmt.Errorf("start %s: %w", command.Path, err)
	}

	session := &ptySession{command: command, terminal: terminal}

	// Under a pty the child's stdout and stderr share one stream, so an error
	// arrives mixed into the terminal output. Keeping the tail lets the exit
	// handler explain what went wrong.
	recentOutput := &tailBuffer{limit: 4096}

	go forwardStream(terminal, func(chunk []byte) {
		recentOutput.append(chunk)
		onOutput(chunk)
	})

	go func() {
		waitErr := command.Wait()
		session.Close()

		if waitErr != nil && explain != nil {
			if message := strings.TrimSpace(explain(recentOutput.string())); message != "" {
				waitErr = errors.New(message)
			}
		}
		onExit(waitErr)
	}()

	return session, nil
}

// ptySession is one child process attached to a pseudo-terminal.
type ptySession struct {
	command  *exec.Cmd
	terminal *os.File

	closeOnce sync.Once
}

func (session *ptySession) Write(payload []byte) (int, error) {
	return session.terminal.Write(payload)
}

func (session *ptySession) Resize(size Size) error {
	return pty.Setsize(session.terminal, &pty.Winsize{
		Cols: size.Cols,
		Rows: size.Rows,
	})
}

func (session *ptySession) Close() error {
	var closeErr error
	session.closeOnce.Do(func() {
		// Killing the process is what ends the session. Closing only the pty
		// would leave it running, holding a remote session open and a process
		// on this machine.
		if session.command.Process != nil {
			session.command.Process.Kill()
		}
		closeErr = session.terminal.Close()
	})
	return closeErr
}
