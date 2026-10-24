//go:build windows

package transport

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

// openSSHAgentPipe is where the agent that ships with Windows listens: the
// "OpenSSH Authentication Agent" service. It needs no SSH_AUTH_SOCK.
const openSSHAgentPipe = `\\.\pipe\openssh-ssh-agent`

// dialAgent reaches the ssh-agent throught a named pipe. A named pipe opens like
// a file, and the agent protocol is one request, one answer, so a plain
// *os.File is enough.
func dialAgent() (io.ReadWriteCloser, error) {
	pipe := agentPipe(os.Getenv("SSH_AUTH_SOCK"))
	connection, err := os.OpenFile(pipe, os.O_RDWR, 0)
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
