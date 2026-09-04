package transport

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

type sshDialer struct{}

func (sshDialer) Name() string { return "SSH" }

func (sshDialer) Preflight(config Config) error {
	if config.Username == "" {
		return fmt.Errorf("This connect has no username")
	}

	switch config.AuthMethod {
	case AuthKey:
		if _, err := os.Stat(expandHome(config.KeyPath)); err != nil {
			return fmt.Errorf("Key file %s is not readable: %w", config.KeyPath, err)
		}
	case AuthAgent, "":
		if os.Getenv("SSH_AUTH_SOCK") == "" {
			return fmt.Errorf(
				"ssh-agent is not running (SSH_AUTH_SOCK is unse); start it, " +
					"or switch this connection to a key file")
		}
	case AuthPassword:
		if config.Password == "" {
			return ErrPasswordRequired
		}
	default:
		return fmt.Errorf("unknown auth method %q", config.AuthMethod)
	}
	return nil
}

func (sshDialer) Dial(
	dialContext context.Context,
	config Config,
	size Size,
	onOutput func([]byte),
	onExit func(error),
) (Session, error) {
	authMethods, releaseAuth, err := sshAuthMethods(config)
	if err != nil {
		return nil, err
	}
	defer releaseAuth()

	hostKeysCallback, err := hostKeyCallbackFor(config.KnownHostsPath)
	if err != nil {
		return nil, err
	}

	clientConfig := &ssh.ClientConfig{
		User:            config.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeysCallback,
		Timeout:         15 * time.Second,
	}

	address := net.JoinHostPort(config.Target, strconv.Itoa(config.Port))

	var tcpDialer net.Dialer
	tcpConnection, err := tcpDialer.DialContext(dialContext, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("reach %s: %w", address, err)
	}

	sshConnection, channels, requests, err := ssh.NewClientConn(tcpConnection, address, clientConfig)
	if err != nil {
		tcpConnection.Close()
		return nil, fmt.Errorf("ssh handshake with %s: %w", address, err)
	}

	client := ssh.NewClient(sshConnection, channels, requests)
	return startShell(client, size, onOutput, onExit)
}
