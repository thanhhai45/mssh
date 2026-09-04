package transport

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// ssmDialer opens a session by running `aws ssm start-session` under a local
// pseudo-terminal.
//
// Nothing in this file handles credentials. SSO refresh, MFA and assume-role
// are the AWS CLI's business, and mssh never sees a secret. That is a feature,
// not a shortcut.
type ssmDialer struct{}

// Compile-time check, reported here rather than wherever For() assigns it.
var _ Dialer = ssmDialer{}

func (ssmDialer) Name() string { return "AWS SSM" }

func (ssmDialer) Preflight(config Config) error {
	if config.Target == "" {
		return fmt.Errorf("this connection has no instance id")
	}
	if err := requireAWSTools(); err != nil {
		return err
	}
	return checkAWSCredentials(config.AWSProfile, config.AWSRegion)
}

func (ssmDialer) Dial(
	dialContext context.Context,
	config Config,
	size Size,
	onOutput func([]byte),
	onExit func(error),
) (Session, error) {
	arguments := append(
		[]string{"ssm", "start-session", "--target", config.Target},
		awsFlags(config.AWSProfile, config.AWSRegion)...,
	)

	command := exec.CommandContext(dialContext, "aws", arguments...)
	command.Env = os.Environ()

	// session-manager-plugin looks at whether its stdin is a terminal to decide
	// on raw mode, and reads the window size from it. Give it a plain pipe and
	// you get a shell with no prompt, a broken Ctrl-C and wrong line wrapping.
	terminal, err := pty.StartWithSize(command, &pty.Winsize{
		Cols: size.Cols,
		Rows: size.Rows,
	})
	if err != nil {
		return nil, fmt.Errorf("start `aws ssm start-session`: %w", err)
	}

	session := &ssmSession{command: command, terminal: terminal}

	// Under a pty the child's stdout and stderr share one stream, so an AWS
	// error arrives mixed into the terminal output. Keeping the tail lets the
	// exit handler explain what went wrong.
	recentOutput := &tailBuffer{limit: 4096}

	go forwardStream(terminal, func(chunk []byte) {
		recentOutput.append(chunk)
		onOutput(chunk)
	})

	go func() {
		waitErr := command.Wait()
		session.Close()

		if waitErr != nil {
			waitErr = errors.New(explainSSMFailure(config.Target, recentOutput.string()))
		}
		onExit(waitErr)
	}()

	return session, nil
}

type ssmSession struct {
	command  *exec.Cmd
	terminal *os.File

	closeOnce sync.Once
}

func (session *ssmSession) Write(payload []byte) (int, error) {
	return session.terminal.Write(payload)
}

func (session *ssmSession) Resize(size Size) error {
	return pty.Setsize(session.terminal, &pty.Winsize{
		Cols: size.Cols,
		Rows: size.Rows,
	})
}

func (session *ssmSession) Close() error {
	var closeErr error
	session.closeOnce.Do(func() {
		// Killing the aws process is what ends the SSM session. Closing only
		// the pty would leave it running, holding an AWS session open and a
		// process on this machine.
		if session.command.Process != nil {
			session.command.Process.Kill()
		}
		closeErr = session.terminal.Close()
	})
	return closeErr
}

// tailBuffer keeps the last few kilobytes written through it, so a process that
// dies early can be explained using whatever it printed on the way out.
type tailBuffer struct {
	mutex sync.Mutex
	data  []byte
	limit int
}

func (buffer *tailBuffer) append(chunk []byte) {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()

	buffer.data = append(buffer.data, chunk...)
	if len(buffer.data) > buffer.limit {
		buffer.data = buffer.data[len(buffer.data)-buffer.limit:]
	}
}

func (buffer *tailBuffer) string() string {
	buffer.mutex.Lock()
	defer buffer.mutex.Unlock()
	return string(buffer.data)
}

func (buffer *tailBuffer) Write(chunk []byte) (int, error) {
	buffer.append(chunk)
	return len(chunk), nil
}
