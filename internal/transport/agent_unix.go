//go:build unix

package transport

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
)

// dialAgent reaches the ssh-agent through the socket SSH_AUTH_SOCK names.
func dialAgent() (io.ReadWriteCloser, error) {
	socket := os.Getenv("SSH_AUTH_SOCK")
	if socket == "" {
		return nil, errors.New("ssh-agent is not running: SSH_AUTH_SOCK is unset")
	}
	connection, err := net.Dial("unix", socket)
	if err != nil {
		return nil, fmt.Errorf("reach ssh-agent: %w", err)
	}
	return connection, nil
}
