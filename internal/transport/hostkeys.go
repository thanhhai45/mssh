package transport

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// UnknownHostKeyError is a server this machine has never connected to.
//
// It carries the key the server presented, so the user can be shown its
// fingerprint and asked, the way the ssh command asks. It also means the key
// that gets recorded is exactly this one: the frontend only ever answers yes
// or no, and never sends a key back.
//
// A *changed* key is a different error and is never offered for trust: that
// is the one case where accepting blindly would defeat host key checking.
type UnknownHostKeyError struct {
	// Host is the name as known_hosts writes it: "host", or "[host]:port"
	// when the port is not 22.
	Host           string
	Key            ssh.PublicKey
	KnownHostsPath string
}

func (unknown *UnknownHostKeyError) Error() string {
	return fmt.Sprintf("%s has never been connected to from this machine (%s key %s)",
		unknown.displayHost(), unknown.Key.Type(), ssh.FingerprintSHA256(unknown.Key))
}

// displayHost drops the brackets and port known_hosts adds for a non-standard
// port, for messages meant to be read.
func (unknown *UnknownHostKeyError) displayHost() string {
	host := unknown.Host
	if withoutPort, _, err := net.SplitHostPort(host); err == nil {
		host = withoutPort
	}
	return host
}

// HostKeyPrompt is what the trust dialog shows. Plain strings only — the key
// itself stays in Go.
type HostKeyPrompt struct {
	Host           string `json:"host"`
	KeyType        string `json:"keyType"`
	Fingerprint    string `json:"fingerprint"`
	KnownHostsPath string `json:"knownHostsPath"`
}

func (unknown *UnknownHostKeyError) Prompt() HostKeyPrompt {
	return HostKeyPrompt{
		Host:           unknown.displayHost(),
		KeyType:        unknown.Key.Type(),
		Fingerprint:    ssh.FingerprintSHA256(unknown.Key),
		KnownHostsPath: unknown.KnownHostsPath,
	}
}

// Remember appends the key to known_hosts, the file the ssh command uses too,
// so trusting a host here also means ssh will not ask about it again.
//
// The directory and file are created owner-only when missing, as ssh creates
// them. A file whose last line has no newline gets one first; otherwise the
// new entry would be glued onto the old one and both would stop matching.
func (unknown *UnknownHostKeyError) Remember() error {
	if err := os.MkdirAll(filepath.Dir(unknown.KnownHostsPath), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(unknown.KnownHostsPath), err)
	}

	existing, err := os.ReadFile(unknown.KnownHostsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", unknown.KnownHostsPath, err)
	}

	file, err := os.OpenFile(unknown.KnownHostsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", unknown.KnownHostsPath, err)
	}

	var line bytes.Buffer
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n")) {
		line.WriteString("\n")
	}
	line.WriteString(knownhosts.Line([]string{unknown.Host}, unknown.Key))
	line.WriteString("\n")

	if _, err := file.Write(line.Bytes()); err != nil {
		_ = file.Close()
		return fmt.Errorf("write %s: %w", unknown.KnownHostsPath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("write %s: %w", unknown.KnownHostsPath, err)
	}
	return nil
}
