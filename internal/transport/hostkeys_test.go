package transport

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// newHostKey makes a throwaway server key. Every test works in its own temp
// dir, so nothing here ever reads or writes the real ~/.ssh/known_hosts.
func newHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	key, err := ssh.NewPublicKey(public)
	if err != nil {
		t.Fatalf("wrap key: %v", err)
	}
	return key
}

var anyRemote = &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 22}

// checkHost runs the same callback a dial would, against knownHostsPath.
func checkHost(t *testing.T, knownHostsPath string, hostname string, key ssh.PublicKey) error {
	t.Helper()
	callback, err := hostKeyCallbackFor(knownHostsPath)
	if err != nil {
		t.Fatalf("hostKeyCallbackFor: %v", err)
	}
	return callback(hostname, anyRemote, key)
}

// A machine that never ran ssh has no known_hosts. That used to fail every
// connection; now each host is simply unknown, which is a question to ask.
func TestMissingKnownHostsMeansEveryHostIsUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never-created", "known_hosts")
	key := newHostKey(t)

	err := checkHost(t, path, "i-0813928dba152abd7:22", key)

	var unknown *UnknownHostKeyError
	if !errors.As(err, &unknown) {
		t.Fatalf("err = %v, want an UnknownHostKeyError", err)
	}
	if unknown.Host != "i-0813928dba152abd7" {
		t.Errorf("host = %q; port 22 is left out, the way known_hosts writes it", unknown.Host)
	}
	if !strings.Contains(err.Error(), ssh.FingerprintSHA256(key)) {
		t.Errorf("the message does not show the fingerprint: %v", err)
	}
}

// Trusting a host writes it where the ssh command looks too, and from then on
// the same key is accepted without asking.
func TestRememberedHostKeyIsTrusted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ssh", "known_hosts")
	key := newHostKey(t)

	var unknown *UnknownHostKeyError
	if err := checkHost(t, path, "build.example:2222", key); !errors.As(err, &unknown) {
		t.Fatalf("first contact: err = %v, want an UnknownHostKeyError", err)
	}
	if unknown.Host != "[build.example]:2222" {
		t.Errorf("host = %q; a non-standard port must be kept, bracketed", unknown.Host)
	}
	if got := unknown.Prompt().Host; got != "build.example" {
		t.Errorf("prompt host = %q; the brackets are for the file, not for people", got)
	}

	if err := unknown.Remember(); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if err := checkHost(t, path, "build.example:2222", key); err != nil {
		t.Errorf("after trusting it, the same key was refused: %v", err)
	}

	if runtime.GOOS != "windows" {
		for file, want := range map[string]os.FileMode{filepath.Dir(path): 0o700, path: 0o600} {
			info, err := os.Stat(file)
			if err != nil {
				t.Fatalf("stat %s: %v", file, err)
			}
			if got := info.Mode().Perm(); got != want {
				t.Errorf("%s is %o, want %o — as ssh would create it", filepath.Base(file), got, want)
			}
		}
	}
}

// A file whose last line has no newline would otherwise get the new entry
// glued onto it, and both would stop matching.
func TestRememberStartsOnAFreshLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	first, second := newHostKey(t), newHostKey(t)

	var unknown *UnknownHostKeyError
	_ = errors.As(checkHost(t, path, "first.example:22", first), &unknown)
	if err := unknown.Remember(); err != nil {
		t.Fatalf("Remember first: %v", err)
	}

	// Strip the trailing newline, the way some editors save.
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimRight(string(content), "\n")), 0o600); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	_ = errors.As(checkHost(t, path, "second.example:22", second), &unknown)
	if err := unknown.Remember(); err != nil {
		t.Fatalf("Remember second: %v", err)
	}

	for host, key := range map[string]ssh.PublicKey{"first.example:22": first, "second.example:22": second} {
		if err := checkHost(t, path, host, key); err != nil {
			t.Errorf("%s no longer matches: %v", host, err)
		}
	}
}

// A key that differs from the recorded one is the case host key checking
// exists for. It must stay a hard refusal — never something to click through.
func TestChangedHostKeyIsNeverOfferedForTrust(t *testing.T) {
	path := filepath.Join(t.TempDir(), "known_hosts")
	original, impostor := newHostKey(t), newHostKey(t)

	var unknown *UnknownHostKeyError
	_ = errors.As(checkHost(t, path, "prod.example:22", original), &unknown)
	if err := unknown.Remember(); err != nil {
		t.Fatalf("Remember: %v", err)
	}

	err := checkHost(t, path, "prod.example:22", impostor)
	if err == nil {
		t.Fatal("a different key for a known host was accepted")
	}
	if errors.As(err, &unknown) {
		t.Fatal("a changed key came back as unknown, which the app would offer to trust")
	}
	if !strings.Contains(err.Error(), "CHANGED") {
		t.Errorf("the refusal does not say the key changed: %v", err)
	}
}

// handshakeWithTestServer runs a real SSH handshake against a server living in
// this process, on a loopback port: the whole protocol, no outside network, no
// real host. Not net.Pipe — both ends of SSH write their version line first,
// and a synchronous pipe deadlocks on that.
func handshakeWithTestServer(t *testing.T, server ssh.Signer, knownHostsPath string) error {
	t.Helper()

	serverConfig := &ssh.ServerConfig{NoClientAuth: true}
	serverConfig.AddHostKey(server)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		serverEnd, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() { _ = serverEnd.Close() }()
		connection, channels, requests, err := ssh.NewServerConn(serverEnd, serverConfig)
		if err != nil {
			return
		}
		go ssh.DiscardRequests(requests)
		for channel := range channels {
			_ = channel.Reject(ssh.Prohibited, "test server")
		}
		_ = connection.Close()
	}()

	clientEnd, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = clientEnd.Close() })

	callback, err := hostKeyCallbackFor(knownHostsPath)
	if err != nil {
		t.Fatalf("hostKeyCallbackFor: %v", err)
	}
	// The name an ssm-ssh dial checks known_hosts for: the instance id.
	connection, _, _, err := handshake(clientEnd, "i-0813928dba152abd7:22", &ssh.ClientConfig{
		User:            "ec2-user",
		HostKeyCallback: callback,
	}, 5*time.Second)
	if err == nil {
		_ = connection.Close()
	}
	return err
}

// The whole round trip the trust dialog depends on, over the real protocol:
// first contact is a question carrying the key, trusting it records the key,
// and the next handshake goes straight through.
func TestHandshakeAsksAboutAnUnknownHostThenTrustsIt(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	server, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	path := filepath.Join(t.TempDir(), "known_hosts")

	err = handshakeWithTestServer(t, server, path)
	var unknown *UnknownHostKeyError
	if !errors.As(err, &unknown) {
		t.Fatalf("first handshake: err = %v, want an UnknownHostKeyError through x/crypto's wrapping", err)
	}
	if unknown.Prompt().Fingerprint != ssh.FingerprintSHA256(server.PublicKey()) {
		t.Error("the fingerprint offered is not the server's")
	}

	if err := unknown.Remember(); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if err := handshakeWithTestServer(t, server, path); err != nil {
		t.Errorf("after trusting the key, the handshake still failed: %v", err)
	}
}
