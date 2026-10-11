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
	// handshakeLimit bounds the whole handshake, since ClientConfig.Timeout
	// only applies to ssh.Dial, which this file does not use.
	handshakeLimit = 15 * time.Second
)

// preAuthDropPause gives the server's startup queue a moment to drain. A
// variable only so that tests of the retry need not wait for real.
var preAuthDropPause = 1500 * time.Millisecond

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
	var (
		sshConnection ssh.Conn
		channels      <-chan ssh.NewChannel
		requests      <-chan *ssh.Request
		// Set only when the handshake itself failed, so the explanation
		// below is never applied to a cancelled dial or an unreachable host.
		handshakeErr error
	)
	err := retryPreAuthDrops(dialContext, func() (bool, error) {
		var tcpDialer net.Dialer
		tcpConnection, err := tcpDialer.DialContext(dialContext, "tcp", address)
		if err != nil {
			// A refused or unroutable address will not fix itself in 1.5s.
			return false, fmt.Errorf("reach %s: %w", address, err)
		}

		sshConnection, channels, requests, err = handshake(
			tcpConnection, address, clientConfig, handshakeLimit)
		if err != nil {
			_ = tcpConnection.Close()
			handshakeErr = err
			return droppedBeforeAuth(err), err
		}
		handshakeErr = nil
		return false, nil
	})
	if err == nil {
		return sshConnection, channels, requests, nil
	}
	if handshakeErr == nil || err != handshakeErr {
		return nil, nil, nil, err
	}
	lastErr := handshakeErr

	// A host the user has not trusted yet is a question for them, not a failure
	// to explain: hand it up as it is, so the app can recognise it and ask.
	var unknown *UnknownHostKeyError
	if errors.As(lastErr, &unknown) {
		return nil, nil, nil, unknown
	}
	return nil, nil, nil, errors.New(explainHandshakeFailure(address, lastErr))
}

// retryPreAuthDrops runs attempt until it succeeds, fails in a way attempt says
// is not worth retrying, or has been tried preAuthDropAttempts times, and
// returns the last error. Shared by ssh and ssm-ssh, so that both give a
// server's MaxStartups the same second chance.
//
// attempt decides, not this function, because what counts as a drop differs:
// through an SSM tunnel an EOF can also be AWS refusing, which no retry fixes.
func retryPreAuthDrops(dialContext context.Context, attempt func() (retry bool, err error)) error {
	var err error
	for tries := 1; tries <= preAuthDropAttempts; tries++ {
		if tries > 1 {
			select {
			case <-dialContext.Done():
				return dialContext.Err()
			case <-time.After(preAuthDropPause):
			}
		}

		var retry bool
		retry, err = attempt()
		if err == nil || !retry {
			return err
		}
	}
	return err
}
