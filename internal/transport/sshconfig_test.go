package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestExplainSSHCommandFailure(t *testing.T) {
	tests := []struct {
		name    string
		output  string
		mustSay []string
	}{
		{
			// The advice used to be "Finder never reads ~/.zshrc". It does now,
			// so the test asserts the current claim rather than a phrase that
			// happens to survive both versions.
			name: "a ProxyCommand with no AWS credentials points at the shell environment",
			output: "Unable to locate credentials. You can configure credentials by " +
				"running \"aws configure\".",
			mustSay: []string{
				"login shell",
				"aws sts get-caller-identity",
				"~/.aws/credentials",
			},
		},
		{
			name:    "a missing profile is the same problem",
			output:  "The config profile (itviec) could not be found",
			mustSay: []string{"aws configure"},
		},
		{
			name:    "an alias ssh does not know about points at the config file",
			output:  "ssh: Could not resolve hostname prod-web-1: nodename nor servname provided",
			mustSay: []string{"~/.ssh/config", "prod-web-1"},
		},
		{
			name:    "rejected credentials point at User and IdentityFile",
			output:  "itviec-admin@prod-web-1: Permission denied (publickey).",
			mustSay: []string{"User", "IdentityFile"},
		},
		{
			name:    "anything else is passed through untouched",
			output:  "kex_exchange_identification: read: Connection reset by peer",
			mustSay: []string{"kex_exchange_identification"},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := explainSSHCommandFailure("prod-web-1", testCase.output)
			for _, phrase := range testCase.mustSay {
				if !strings.Contains(got, phrase) {
					t.Errorf("explanation does not mention %q:\n%s", phrase, got)
				}
			}
		})
	}
}

/* ---------------- sharing the connection ---------------- */

func TestTerminalArgumentsMakeAMaster(t *testing.T) {
	session := &sshConfigSession{alias: "prod-web-1", controlPath: "/tmp/mssh-1/ssh"}
	want := []string{"-o", "ControlMaster=yes", "-o", "ControlPersist=" + controlPersist,
		"-S", "/tmp/mssh-1/ssh", "prod-web-1"}
	if got := session.terminalArguments(); !slices.Equal(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	// Nowhere to put the socket: the plain command, as before.
	alone := &sshConfigSession{alias: "prod-web-1"}
	if got := alone.terminalArguments(); !slices.Equal(got, []string{"prod-web-1"}) {
		t.Errorf("without a socket: %q", got)
	}
}

func TestSFTPArgumentsGoThroughTheMaster(t *testing.T) {
	session := &sshConfigSession{alias: "prod-web-1", controlPath: "/tmp/mssh-1/ssh"}
	got := session.sftpArguments()

	for _, option := range []string{"ControlMaster=no", "BatchMode=yes", "ClearAllForwardings=yes", "RemoteCommand=none"} {
		if !slices.Contains(got, option) {
			t.Errorf("missing %s: %q", option, got)
		}
	}
	if index := slices.Index(got, "-S"); index < 0 || got[index+1] != "/tmp/mssh-1/ssh" {
		t.Errorf("not pointed at the socket: %q", got)
	}
	if tail := got[len(got)-4:]; !slices.Equal(tail, []string{"-T", "-s", "prod-web-1", "sftp"}) {
		t.Errorf("does not end by asking for the sftp subsystem: %q", got)
	}
}

// ssh expands %h, %r and friends in a control path; a literal % must survive.
func TestControlPathIsEscaped(t *testing.T) {
	if got := escapeControlPath("/tmp/100%/ssh"); got != "/tmp/100%%/ssh" {
		t.Errorf("got %q", got)
	}
}

// ssh will not start at all with a socket path too long for the system, so
// a long temporary directory gives way to the next one.
func TestSharedConnectionPathSkipsLongDirectories(t *testing.T) {
	long := filepath.Join(t.TempDir(), strings.Repeat("d", maxControlPathLength))
	if err := os.MkdirAll(long, 0o700); err != nil {
		t.Fatal(err)
	}
	short := t.TempDir()

	directory, socket := sharedConnectionPath([]string{long, short})
	if !strings.HasPrefix(directory, short) || filepath.Dir(socket) != directory {
		t.Fatalf("got %q, %q; want a socket under %q", directory, socket, short)
	}
	if len(socket) > maxControlPathLength {
		t.Errorf("socket path is %d bytes", len(socket))
	}
	if info, err := os.Stat(directory); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("the directory is not private: %v %v", info.Mode(), err)
	}
	if leftovers, _ := os.ReadDir(long); len(leftovers) != 0 {
		t.Errorf("left %d directories behind in the long one", len(leftovers))
	}

	if directory, socket := sharedConnectionPath([]string{long}); directory != "" || socket != "" {
		t.Errorf("nowhere short enough, got %q %q", directory, socket)
	}
}

/* ---------------- against the real ssh ---------------- */

// fileServer is an SSH server on a loopback port for the real ssh command: any
// login, a shell that echoes and leaves on "exit", and the sftp subsystem over
// a directory of the test's.
type fileServer struct {
	address   string
	hostKey   ssh.PublicKey
	root      string
	logins    atomic.Int32
	subsystem atomic.Int32
}

func newFileServer(t *testing.T) *fileServer {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	server := &fileServer{hostKey: signer.PublicKey(), root: t.TempDir()}
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	server.address = listener.Addr().String()

	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			go server.serve(connection, config)
		}
	}()
	return server
}

func (server *fileServer) serve(connection net.Conn, config *ssh.ServerConfig) {
	defer func() { _ = connection.Close() }()
	conn, channels, requests, err := ssh.NewServerConn(connection, config)
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()
	server.logins.Add(1)
	go ssh.DiscardRequests(requests)

	for incoming := range channels {
		if incoming.ChannelType() != "session" {
			_ = incoming.Reject(ssh.UnknownChannelType, "sessions only")
			continue
		}
		channel, channelRequests, err := incoming.Accept()
		if err != nil {
			continue
		}
		go func() {
			for request := range channelRequests {
				switch {
				case request.Type == "shell":
					_ = request.Reply(true, nil)
					go echoUntilExit(channel)
				case request.Type == "subsystem" && string(request.Payload[4:]) == "sftp":
					_ = request.Reply(true, nil)
					server.subsystem.Add(1)
					go func() {
						files, err := sftp.NewServer(channel, sftp.WithServerWorkingDirectory(server.root))
						if err == nil {
							_ = files.Serve()
						}
						_ = channel.Close()
					}()
				case request.Type == "pty-req", request.Type == "window-change", request.Type == "env":
					_ = request.Reply(true, nil)
				default:
					_ = request.Reply(false, nil)
				}
			}
		}()
	}
}

// echoUntilExit is the whole shell: a prompt, every byte sent back, and the
// end of the session once a line says exit.
func echoUntilExit(channel ssh.Channel) {
	_, _ = channel.Write([]byte("fake-shell$ "))
	var line []byte
	buffer := make([]byte, 1024)
	for {
		count, err := channel.Read(buffer)
		_, _ = channel.Write(buffer[:count])
		line = append(line, buffer[:count]...)
		if strings.Contains(string(line), "exit") || err != nil {
			break
		}
	}
	_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
	_ = channel.Close()
}

// useSystemSSH points the real ssh at server under the alias "fake", through
// a config file of the test's own. Skipped where there is no ssh to run.
func useSystemSSH(t *testing.T, server *fileServer) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows' ssh cannot share a connection")
	}
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("no ssh command on this machine")
	}
	host, port, _ := net.SplitHostPort(server.address)
	directory := t.TempDir()
	knownHosts := filepath.Join(directory, "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(server.address)}, server.hostKey)
	if err := os.WriteFile(knownHosts, []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(directory, "config")
	config := fmt.Sprintf("Host fake\n  HostName %s\n  Port %s\n  User tester\n"+
		"  UserKnownHostsFile %s\n  StrictHostKeyChecking yes\n  IdentitiesOnly yes\n"+
		"  IdentityFile none\n  IdentityAgent none\n", host, port, knownHosts)
	if err := os.WriteFile(configFile, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	original := systemSSHOptions
	systemSSHOptions = []string{"-F", configFile}
	t.Cleanup(func() { systemSSHOptions = original })
}

func dialSystemSSH(t *testing.T) (*sshConfigSession, *sessionLog) {
	t.Helper()
	log := newSessionLog()
	session, err := (sshConfigDialer{}).Dial(context.Background(), Config{Target: "fake"},
		Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	shared, ok := session.(*sshConfigSession)
	if !ok {
		t.Fatalf("Dial returned %T, which cannot browse files", session)
	}
	t.Cleanup(func() { _ = shared.Close() })
	log.waitFor(t, "fake-shell$ ")
	return shared, log
}

// Files go over the terminal's own connection: the server sees one login.
func TestSystemSSHBrowsesFilesOverTheSameLogin(t *testing.T) {
	server := newFileServer(t)
	useSystemSSH(t, server)
	if err := os.WriteFile(filepath.Join(server.root, "notes.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	session, _ := dialSystemSSH(t)

	client, err := session.SFTP()
	if err != nil {
		t.Fatalf("SFTP: %v", err)
	}
	entries, err := client.ReadDir(server.root)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "notes.txt" {
		t.Errorf("listed %v, want notes.txt", entries)
	}
	if again, _ := session.SFTP(); again != client {
		t.Error("the second call opened another client")
	}
	if logins := server.logins.Load(); logins != 1 {
		t.Errorf("the server saw %d logins, want 1", logins)
	}
}

// Typing exit ends the session at once, even with a file client open on the
// connection, and takes the client and the socket with it.
func TestSystemSSHExitEndsTheSessionWithFilesOpen(t *testing.T) {
	server := newFileServer(t)
	useSystemSSH(t, server)
	session, log := dialSystemSSH(t)

	client, err := session.SFTP()
	if err != nil {
		t.Fatalf("SFTP: %v", err)
	}
	if _, err := session.Write([]byte("exit\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := log.waitExit(t); err != nil {
		t.Errorf("onExit = %v, want a clean end", err)
	}

	if _, err := client.Getwd(); err == nil {
		t.Error("the file client still works after the shell ended")
	}
	if _, err := session.SFTP(); !errors.Is(err, errSessionClosed) {
		t.Errorf("SFTP after the end: %v, want errSessionClosed", err)
	}
	if _, err := os.Stat(session.controlDirectory); !os.IsNotExist(err) {
		t.Errorf("the socket directory is still there: %v", err)
	}
}

// Closing from mssh stops the background connection too.
func TestSystemSSHCloseStopsTheSharedConnection(t *testing.T) {
	server := newFileServer(t)
	useSystemSSH(t, server)
	session, log := dialSystemSSH(t)
	if _, err := session.SFTP(); err != nil {
		t.Fatalf("SFTP: %v", err)
	}

	if err := session.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if err := log.waitExit(t); err != nil {
		t.Errorf("onExit after Close = %v, want nil", err)
	}
	check := exec.Command("ssh", append(session.clientArguments(), "-O", "check", "fake")...)
	if output, err := check.CombinedOutput(); err == nil {
		t.Errorf("the shared connection is still running: %s", output)
	}
	if _, err := os.Stat(session.controlDirectory); !os.IsNotExist(err) {
		t.Errorf("the socket directory is still there: %v", err)
	}
}
