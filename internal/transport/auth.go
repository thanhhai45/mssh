package transport

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// sshAuthMethods returns the methods to offer, plus a release function the
// caller must run once the handshake is over.
func sshAuthMethods(config Config) ([]ssh.AuthMethod, func(), error) {
	nothingToRelease := func() {}

	switch config.AuthMethod {
	case AuthAgent, "":
		return agentAuthMethods()
	case AuthKey:
		methods, err := keyAuthMethods(config.KeyPath)
		return methods, nothingToRelease, err
	case AuthPassword:
		if config.Password == "" {
			return nil, nothingToRelease, ErrPasswordRequired
		}
		return passwordAuthMethods(config.Password), nothingToRelease, nil
	default:
		return nil, nothingToRelease, fmt.Errorf("unknown auth method %q", config.AuthMethod)
	}
}

func agentAuthMethods() ([]ssh.AuthMethod, func(), error) {
	nothingToRelease := func() {}

	agentSocket := os.Getenv("SSH_AUTH_SOCK")
	if agentSocket == "" {
		return nil, nothingToRelease, fmt.Errorf("ssh-agent is not running: SSH_AUTH_SOCK is unset")
	}

	agentConnection, err := net.Dial("unix", agentSocket)
	if err != nil {
		return nil, nothingToRelease, fmt.Errorf("reach ssh-agent: %w", err)
	}

	// The signers keep talking to the agent over this socket during the
	// handshake, so it cannot be closed until Dial is done with them.
	agentClient := agent.NewClient(agentConnection)
	methods := []ssh.AuthMethod{ssh.PublicKeysCallback(agentClient.Signers)}

	return methods, func() { agentConnection.Close() }, nil
}

func keyAuthMethods(keyPath string) ([]ssh.AuthMethod, error) {
	resolvePath := expandHome(keyPath)

	keyBytes, err := os.ReadFile(resolvePath)
	if err != nil {
		return nil, fmt.Errorf("read key %s: %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		var passphraseMissing *ssh.PassphraseMissingError
		if errors.As(err, &passphraseMissing) {
			return nil, fmt.Errorf(
				"key %s is protected by a passphrase; load it with `ssh-add %s` "+
					"and switch this connection to the SSH agent", keyPath, keyPath)
		}
		return nil, fmt.Errorf("parse key %s: %w", keyPath, err)
	}

	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

func passwordAuthMethods(password string) []ssh.AuthMethod {
	answerEveryQuestion := func(
		name string, instruction string, questions []string, echos []bool,
	) ([]string, error) {
		answers := make([]string, len(questions))
		for index := range questions {
			answers[index] = password
		}
		return answers, nil
	}
	return []ssh.AuthMethod{
		ssh.Password(password),
		ssh.KeyboardInteractive(answerEveryQuestion),
	}
}

/* ---------------- host keys ---------------- */
func hostKeyCallbackFor(knownHostsPath string) (ssh.HostKeyCallback, error) {
	if knownHostsPath == "" {
		homeDirectory, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("locate your home directory: %w", err)
		}
		knownHostsPath = filepath.Join(homeDirectory, ".ssh", "known_hosts")
	}

	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf(
			"read %s: %w - connect once with the ssh command first so the host "+
				"key gets recorded", knownHostsPath, err)
	}

	return callback, nil
}

func expandHome(rawPath string) string {
	if !strings.HasPrefix(rawPath, "-/") {
		return rawPath
	}
	homeDirectory, err := os.UserHomeDir()
	if err != nil {
		return rawPath
	}
	return filepath.Join(homeDirectory, rawPath[2:])
}
