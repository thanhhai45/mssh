//go:build windows

package transport

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// openSSHAgentPipe is where the agent that ships with Windows listens: the
// "OpenSSH Authentication Agent" service. It needs no SSH_AUTH_SOCK.
const openSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

// dialAgent reaches the ssh-agent through a named pipe.
func dialAgent() (io.ReadWriteCloser, error) {
	pipe := agentPipe(os.Getenv("SSH_AUTH_SOCK"))
	connection, err := openOverlappedPipe(pipe)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errors.New("ssh-agent is not running: start the " +
			"\"OpenSSH Authentication Agent\" service, or switch this " +
			"connection to a key file")
	}
	if err != nil {
		return nil, fmt.Errorf("reach ssh-agent: %w", err)
	}
	return connection, nil
}

// agentPipe picks the pipe to open. SSH_AUTH_SOCK wins when it names a pipe.
// as other agents set it; Git Bash sets it to a Unit-style path no Windows
// program can open, and that is passed over for the system agent.
func agentPipe(authSock string) string {
	if strings.HasPrefix(authSock, `\\.\pipe\`) {
		return authSock
	}
	return openSSHAgentPipe
}

// openOverlappedPipe opens a named pipe for overlapped I/O.
//
// x/crypto's agent client writes each request from one goroutine while
// another is already reading for the answer. On a handle opened the ordinary
// way - os.OpenFile - Windows lets one operation through at a time, so the read
// waiting for an answer holds up the write of the question, for ever. Opened
// overlapped, os.NewFile hands the handle to the runtime's poller, and the two
// proceed side by side, as on a Unix socket.
func openOverlappedPipe(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name,
		windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_OVERLAPPED, 0)
	if err != nil {
		// A PathError, like os.OpenFile's, so errors.Is still finds
		// fs.ErrNotExist when the agent is not running.
		return nil, &fs.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(handle), path), nil
}
