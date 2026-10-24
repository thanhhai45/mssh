//go:build unix

package transport

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// startUnderPTY starts command with a new pseudo-terminal as its stdin stdout
// and stderr, at the given size.
func startUnderPTY(command *exec.Cmd, size Size) (ptyProcess, error) {
	terminal, err := pty.StartWithSize(command, &pty.Winsize{
		Cols: size.Cols,
		Rows: size.Rows,
	})
	if err != nil {
		return ptyProcess{}, err
	}
	return ptyProcess{
		terminal: unixTerminal{terminal},
		wait:     command.Wait,
		kill:     func() error { return command.Process.Kill() },
	}, nil
}

// unixTerminal is the master end of th pty, which a file already is.
type unixTerminal struct{ *os.File }

func (terminal unixTerminal) Resize(size Size) error {
	return pty.Setsize(terminal.File, &pty.Winsize{
		Cols: size.Cols,
		Rows: size.Rows,
	})
}
