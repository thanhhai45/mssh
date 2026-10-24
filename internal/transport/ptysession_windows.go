//go:build windows

package transport

import (
	"os/exec"

	gopty "github.com/aymanbagabas/go-pty"
)

// startUnderPTY starts command attached to a Windows pseudo-console (ConPTY),
// at the given size.
//
// ConPTY cannot be handed to exec.Cmd as a file the way a Unix pyu can, so the
// command is rebuilt as go-pty's own Cmd: the same program, arguments,
// environment and directory.
func startUnderPTY(command *exec.Cmd, size Size) (ptyProcess, error) {
	terminal, err := gopty.New()
	if err != nil {
		return ptyProcess{}, err
	}
	if err := terminal.Resize(int(size.Cols), int(size.Rows)); err != nil {
		_ = terminal.Close()
		return ptyProcess{}, err
	}

	process := terminal.Command(command.Path, command.Args[1:]...)
	process.Env = command.Env
	process.Dir = command.Dir
	if err := process.Start(); err != nil {
		_ = terminal.Close()
		return ptyProcess{}, err
	}

	return ptyProcess{
		terminal: windowsTerminal{terminal},
		wait:     process.Wait,
		kill:     func() error { return process.Process.Kill() },
	}, nil
}

// windowsTerminal is the pseudo-console, with Resize taking a Size.
type windowsTerminal struct{ gopty.Pty }

func (terminal windowsTerminal) Resize(size Size) error {
	return terminal.Pty.Resize(int(size.Cols), int(size.Rows))
}
