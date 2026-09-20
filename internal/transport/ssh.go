package transport

import (
	"context"
	"errors"
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

	sshConnection, channels, requests, err := dialAndHandshake(
		dialContext, address, clientConfig)
	if err != nil {
		return nil, err
	}

	client := ssh.NewClient(sshConnection, channels, requests)
	return startShell(client, size, onOutput, onExit)
}

const (
	// preAuthDropAttempts is how many times a connection turned away before
	// authentication is tried again.
	preAuthDropAttempts = 3
	// preAuthDropPause gives the server's startup queue a moment to drain.
	preAuthDropPause = 1500 * time.Millisecond
	// handshakeLimit bounds the whole handshake, since ClientConfig.Timeout
	// only applies to ssh.Dial, which this file does not use.
	handshakeLimit = 15 * time.Second
)

// dialAndHandshake opens the TCP connection and runs the SSH handshake, trying
// again while the server hangs up before authentication.
//
// The retry exists because sshd's MaxStartups refuses a *random share* of new
// connections whenever its queue of unauthenticated logins is full — so on a
// server exposed to the internet, roughly a third of attempts can fail for
// reasons that have nothing to do with the user, and succeed a second later.
// Only pre-authentication drops are retried; see droppedBeforeAuth.
func dialAndHandshake(
	dialContext context.Context,
	address string,
	clientConfig *ssh.ClientConfig,
) (ssh.Conn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	var lastErr error

	for attempt := 1; attempt <= preAuthDropAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-dialContext.Done():
				return nil, nil, nil, dialContext.Err()
			case <-time.After(preAuthDropPause):
			}
		}

		var tcpDialer net.Dialer
		tcpConnection, err := tcpDialer.DialContext(dialContext, "tcp", address)
		if err != nil {
			// A refused or unroutable address will not fix itself in 1.5s.
			return nil, nil, nil, fmt.Errorf("reach %s: %w", address, err)
		}

		sshConnection, channels, requests, err := handshake(
			tcpConnection, address, clientConfig, handshakeLimit)
		if err == nil {
			return sshConnection, channels, requests, nil
		}

		_ = tcpConnection.Close()
		lastErr = err

		if !droppedBeforeAuth(err) {
			break
		}
	}

	return nil, nil, nil, errors.New(explainHandshakeFailure(address, lastErr))
}
