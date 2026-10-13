package transport

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

/* ---------------- Tầng 1: the arguments, no process ---------------- */

func TestSSMSessionArguments(t *testing.T) {
	cases := []struct {
		name   string
		config Config
		want   []string
	}{
		{"instance only", Config{Target: "i-0abc"},
			[]string{"ssm", "start-session", "--target", "i-0abc"}},
		{"profile and region", Config{Target: "i-0abc", AWSProfile: "prod", AWSRegion: "ap-southeast-1"},
			[]string{"ssm", "start-session", "--target", "i-0abc", "--profile", "prod", "--region", "ap-southeast-1"}},
		// Stored keys travel in the environment; a --profile would outrank them.
		{"stored keys drop the profile", Config{Target: "i-0abc", AWSProfile: "prod",
			AWSCredentials: &AWSCredentials{AccessKeyID: "AKIA", SecretAccessKey: "s"}},
			[]string{"ssm", "start-session", "--target", "i-0abc"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := ssmSessionArguments(test.config); !slices.Equal(got, test.want) {
				t.Errorf("got  %q\nwant %q", got, test.want)
			}
		})
	}
}

func TestSSMTunnelArguments(t *testing.T) {
	got := ssmTunnelArguments(Config{Target: "i-0abc", AWSRegion: "us-east-1"})
	want := []string{"ssm", "start-session", "--target", "i-0abc",
		"--document-name", "AWS-StartSSHSession", "--parameters", "portNumber=22",
		"--region", "us-east-1"}
	if !slices.Equal(got, want) {
		t.Errorf("default port:\ngot  %q\nwant %q", got, want)
	}

	got = ssmTunnelArguments(Config{Target: "i-0abc", Port: 2222})
	if !slices.Contains(got, "portNumber=2222") {
		t.Errorf("custom port missing: %q", got)
	}
}

func TestTunnelDocumentName(t *testing.T) {
	cases := map[string]string{
		"":                              "AWS-StartSSHSession",
		`{"documentName":"Custom-SSH"}`: "Custom-SSH",
		`{"documentName":""}`:           "AWS-StartSSHSession",
		`{"somethingElse":true}`:        "AWS-StartSSHSession",
		`{not json`:                     "AWS-StartSSHSession", // never an error
	}
	for extra, want := range cases {
		if got := tunnelDocumentName(extra); got != want {
			t.Errorf("tunnelDocumentName(%q) = %q, want %q", extra, got, want)
		}
	}
}

// Regression: the tunnel's name once lacked the port, and knownhosts, which
// splits it into host and port, failed on it.
func TestTunnelAddressCarriesThePort(t *testing.T) {
	if got := tunnelAddress(Config{Target: "i-0abc"}); got != "i-0abc:22" {
		t.Errorf("default: %q", got)
	}
	if got := tunnelAddress(Config{Target: "i-0abc", Port: 2222}); got != "i-0abc:2222" {
		t.Errorf("custom port: %q", got)
	}
	if got := (tunnelAddr{"i-0abc:22"}).String(); got != "i-0abc:22" {
		t.Errorf("tunnelAddr.String() = %q", got)
	}
}

/* ---------------- the fake aws ---------------- */

// fakeAWSMode, when set, makes the test binary act as aws instead of running
// tests. A two-line script named aws runs it that way: Go rather than sh, so
// the fake can pipe bytes to a TCP port without depending on nc.
const fakeAWSMode = "MSSH_FAKE_AWS"

var (
	// fakeAWSDirectory holds aws and session-manager-plugin.
	fakeAWSDirectory string
	// fakeAWSWithoutPlugin holds aws alone.
	fakeAWSWithoutPlugin string
)

// prepareFakeAWS writes the scripts once for the whole binary, as the fake
// shell does: macOS scans a new executable the first time it runs.
func prepareFakeAWS(parent string) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	script := fmt.Sprintf("#!/bin/sh\n%s=1 exec %q \"$@\"\n", fakeAWSMode, binary)

	fakeAWSDirectory = filepath.Join(parent, "aws-full")
	fakeAWSWithoutPlugin = filepath.Join(parent, "aws-only")
	for _, directory := range []string{fakeAWSDirectory, fakeAWSWithoutPlugin} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(directory, "aws"), []byte(script), 0o700); err != nil {
			return err
		}
	}
	return os.WriteFile(filepath.Join(fakeAWSDirectory, "session-manager-plugin"),
		[]byte("#!/bin/sh\nexit 0\n"), 0o700)
}

// fakeAWS is what the test asked the fake to do. Every field reaches the
// child through the environment.
type fakeAWS struct {
	// stsFailure makes `sts get-caller-identity` fail with this output.
	stsFailure string
	// sessionFailure makes an interactive start-session print this and fail.
	sessionFailure string
	// drops is how many tunnels hang up at once, as sshd does to a connection
	// MaxStartups turns away, before one goes through to sshd.
	drops int
	// tunnelStderr is what a dropped tunnel prints on stderr first.
	tunnelStderr string
	// sshd is where a tunnel that goes through connects.
	sshd string
	// silent makes a tunnel that never says a word: aws stuck, or a network
	// that swallows everything.
	silent bool
	// noise is what a tunnel that goes through prints on stderr first, as
	// the real one prints warnings.
	noise string
}

// useFakeAWS puts the fake first on PATH and returns the files it reports in.
func useFakeAWS(t *testing.T, fake fakeAWS, withPlugin bool) (argsLog string, pidFile string) {
	t.Helper()
	// Preflight would otherwise adopt the login shell's PATH, and with it a
	// real aws or session-manager-plugin. Marking that done keeps the test
	// to the PATH it sets.
	adoptPathOnce.Do(func() {})

	directory := t.TempDir()
	argsLog = filepath.Join(directory, "args.log")
	pidFile = filepath.Join(directory, "tunnel.pid")

	if withPlugin {
		t.Setenv("PATH", fakeAWSDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	} else {
		t.Setenv("PATH", fakeAWSWithoutPlugin)
	}
	t.Setenv("MSSH_FAKE_AWS_LOG", argsLog)
	t.Setenv("MSSH_FAKE_AWS_COUNT", filepath.Join(directory, "count"))
	t.Setenv("MSSH_FAKE_AWS_PID", pidFile)
	t.Setenv("MSSH_FAKE_AWS_STS_FAILURE", fake.stsFailure)
	t.Setenv("MSSH_FAKE_AWS_SESSION_FAILURE", fake.sessionFailure)
	t.Setenv("MSSH_FAKE_AWS_DROPS", strconv.Itoa(fake.drops))
	t.Setenv("MSSH_FAKE_AWS_TUNNEL_STDERR", fake.tunnelStderr)
	t.Setenv("MSSH_FAKE_SSHD", fake.sshd)
	t.Setenv("MSSH_FAKE_AWS_SILENT", map[bool]string{true: "1"}[fake.silent])
	t.Setenv("MSSH_FAKE_AWS_NOISE", fake.noise)
	return argsLog, pidFile
}

// calls reads back every command line the fake was run with.
func calls(t *testing.T, argsLog string) []string {
	t.Helper()
	content, err := os.ReadFile(argsLog)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read args log: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(content)), "\n")
}

// runFakeAWS is the fake aws itself, running in a child process.
func runFakeAWS(arguments []string) int {
	if file, err := os.OpenFile(os.Getenv("MSSH_FAKE_AWS_LOG"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600); err == nil {
		_, _ = fmt.Fprintln(file, strings.Join(arguments, " "))
		_ = file.Close()
	}

	command := strings.Join(arguments[:min(2, len(arguments))], " ")
	switch {
	case command == "sts get-caller-identity":
		if failure := os.Getenv("MSSH_FAKE_AWS_STS_FAILURE"); failure != "" {
			_, _ = fmt.Fprintln(os.Stderr, failure)
			return 255
		}
		fmt.Println(`{"Arn":"arn:aws:iam::111122223333:user/test"}`)
		return 0
	case command == "ssm start-session" && slices.Contains(arguments, "--document-name"):
		return fakeTunnel()
	case command == "ssm start-session":
		return fakeInteractiveSession()
	}
	_, _ = fmt.Fprintln(os.Stderr, "fake aws: unexpected arguments", arguments)
	return 2
}

func fakeInteractiveSession() int {
	if failure := os.Getenv("MSSH_FAKE_AWS_SESSION_FAILURE"); failure != "" {
		fmt.Print(failure)
		return 255
	}
	fmt.Print("fake-ssm$ ")
	_, _ = io.Copy(os.Stdout, os.Stdin)
	return 0
}

func fakeTunnel() int {
	if os.Getenv("MSSH_FAKE_AWS_SILENT") != "" {
		_ = os.WriteFile(os.Getenv("MSSH_FAKE_AWS_PID"), []byte(strconv.Itoa(os.Getpid())), 0o600)
		_, _ = io.Copy(io.Discard, os.Stdin)
		return 0
	}
	drops, _ := strconv.Atoi(os.Getenv("MSSH_FAKE_AWS_DROPS"))
	if countTunnel() <= drops {
		if said := os.Getenv("MSSH_FAKE_AWS_TUNNEL_STDERR"); said != "" {
			_, _ = fmt.Fprintln(os.Stderr, said)
		}
		return 0
	}

	if noise := os.Getenv("MSSH_FAKE_AWS_NOISE"); noise != "" {
		_, _ = fmt.Fprintln(os.Stderr, noise)
	}
	connection, err := net.Dial("tcp", os.Getenv("MSSH_FAKE_SSHD"))
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "fake tunnel:", err)
		return 1
	}
	_ = os.WriteFile(os.Getenv("MSSH_FAKE_AWS_PID"), []byte(strconv.Itoa(os.Getpid())), 0o600)
	go func() { _, _ = io.Copy(connection, os.Stdin) }()
	_, _ = io.Copy(os.Stdout, connection)
	return 0
}

// countTunnel numbers the tunnels opened in one test. They are opened one
// after another, so a file is enough.
func countTunnel() int {
	path := os.Getenv("MSSH_FAKE_AWS_COUNT")
	content, _ := os.ReadFile(path)
	count, _ := strconv.Atoi(strings.TrimSpace(string(content)))
	count++
	_ = os.WriteFile(path, []byte(strconv.Itoa(count)), 0o600)
	return count
}

/* ---------------- an sshd inside the test ---------------- */

// fakeSSHD is an SSH server on a loopback port: any login, a session that
// grants a pty and a shell, and a shell that echoes what it is sent. Behind
// the fake tunnel it is the whole ssm-ssh path except AWS itself.
func fakeSSHD(t *testing.T) (address string, hostKey ssh.PublicKey) {
	t.Helper()
	address, hostKey, _ = fakeSSHDDropping(t, 0)
	return address, hostKey
}

// fakeSSHDDropping is fakeSSHD that hangs up on its first drops connections
// before saying anything, as sshd does when MaxStartups turns one away.
// accepted counts every connection it took.
func fakeSSHDDropping(t *testing.T, drops int) (address string, hostKey ssh.PublicKey, accepted *atomic.Int32) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	accepted = &atomic.Int32{}
	go func() {
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			if int(accepted.Add(1)) <= drops {
				_ = connection.Close()
				continue
			}
			go serveFakeSSH(connection, config)
		}
	}()
	return listener.Addr().String(), signer.PublicKey(), accepted
}

func serveFakeSSH(connection net.Conn, config *ssh.ServerConfig) {
	defer func() { _ = connection.Close() }()
	server, channels, requests, err := ssh.NewServerConn(connection, config)
	if err != nil {
		return
	}
	defer func() { _ = server.Close() }()
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
				switch request.Type {
				case "pty-req", "window-change", "env":
					_ = request.Reply(true, nil)
				case "shell":
					_ = request.Reply(true, nil)
					go func() { _, _ = io.Copy(channel, channel) }()
				default:
					_ = request.Reply(false, nil)
				}
			}
		}()
	}
}

// trustedKnownHosts writes a known_hosts that trusts key for the instance.
func trustedKnownHosts(t *testing.T, instance string, key ssh.PublicKey) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(instance + ":22")}, key)
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("known_hosts: %v", err)
	}
	return path
}

/* ---------------- collecting what a session does ---------------- */

type sessionLog struct {
	mutex  sync.Mutex
	output bytes.Buffer
	exited chan error
}

func newSessionLog() *sessionLog { return &sessionLog{exited: make(chan error, 1)} }

func (log *sessionLog) onOutput(chunk []byte) {
	log.mutex.Lock()
	defer log.mutex.Unlock()
	log.output.Write(chunk)
}

func (log *sessionLog) onExit(err error) { log.exited <- err }

// waitFor waits until the output contains text.
func (log *sessionLog) waitFor(t *testing.T, text string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		log.mutex.Lock()
		found := strings.Contains(log.output.String(), text)
		log.mutex.Unlock()
		if found {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	log.mutex.Lock()
	defer log.mutex.Unlock()
	t.Fatalf("output never contained %q; got %q", text, log.output.String())
}

func (log *sessionLog) waitExit(t *testing.T) error {
	t.Helper()
	select {
	case err := <-log.exited:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("the session never reported its end")
		return nil
	}
}

/* ---------------- Tầng 2: kind ssm against the fake ---------------- */

func TestSSMSessionRunsAWSUnderAPTY(t *testing.T) {
	argsLog, _ := useFakeAWS(t, fakeAWS{}, true)
	config := Config{Target: "i-0test", AWSProfile: "dev", AWSRegion: "ap-southeast-1"}
	log := newSessionLog()

	session, err := ssmDialer{}.Dial(context.Background(), config, Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = session.Close() }()

	log.waitFor(t, "fake-ssm$")
	if _, err := session.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	log.waitFor(t, "hello")
	if err := session.Resize(Size{Cols: 120, Rows: 40}); err != nil {
		t.Errorf("Resize: %v", err)
	}

	// What reached aws, compared with what Tầng 1 says it should be.
	if got, want := calls(t, argsLog), strings.Join(ssmSessionArguments(config), " "); !slices.Contains(got, want) {
		t.Errorf("aws was run with %q, want %q", got, want)
	}
}

// Regression for closedOnPurpose: closing is not a failure.
func TestSSMCloseIsNotAnError(t *testing.T) {
	useFakeAWS(t, fakeAWS{}, true)
	log := newSessionLog()

	session, err := ssmDialer{}.Dial(context.Background(), Config{Target: "i-0test"}, Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	log.waitFor(t, "fake-ssm$")
	_ = session.Close()

	if err := log.waitExit(t); err != nil {
		t.Errorf("onExit after Close = %v, want nil", err)
	}
}

// Regression for lastLines: the reason, not 4 KB of scrollback.
func TestSSMFailureIsExplained(t *testing.T) {
	noise := strings.Repeat("Starting session with SessionId: noise\n", 120)
	useFakeAWS(t, fakeAWS{sessionFailure: noise +
		"An error occurred (TargetNotConnected) when calling the StartSession operation: i-0test is not connected.\n"}, true)
	log := newSessionLog()

	if _, err := (ssmDialer{}).Dial(context.Background(), Config{Target: "i-0test"}, Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit); err != nil {
		t.Fatalf("Dial: %v", err)
	}
	err := log.waitExit(t)
	if err == nil || !strings.Contains(err.Error(), "not reachable through Session Manager") {
		t.Fatalf("onExit = %v, want the TargetNotConnected explanation", err)
	}
	if len(err.Error()) > 400 {
		t.Errorf("the explanation carries the scrollback: %d bytes", len(err.Error()))
	}
}

// A failure nothing recognises still comes back short: its last lines.
func TestSSMUnknownFailureKeepsTheLastLines(t *testing.T) {
	noise := strings.Repeat("Starting session with SessionId: noise\n", 120)
	useFakeAWS(t, fakeAWS{sessionFailure: noise + "something nobody has seen before\n"}, true)
	log := newSessionLog()

	if _, err := (ssmDialer{}).Dial(context.Background(), Config{Target: "i-0test"}, Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit); err != nil {
		t.Fatalf("Dial: %v", err)
	}
	err := log.waitExit(t)
	if err == nil || !strings.Contains(err.Error(), "something nobody has seen before") {
		t.Fatalf("onExit = %v, want the last line kept", err)
	}
	if lines := strings.Count(err.Error(), "\n") + 1; lines > 3 {
		t.Errorf("onExit carries %d lines of scrollback, want at most 3", lines)
	}
}

func TestPreflightNamesTheMissingPlugin(t *testing.T) {
	useFakeAWS(t, fakeAWS{}, false)

	err := ssmDialer{}.Preflight(Config{Target: "i-0test"})
	if err == nil || !strings.Contains(err.Error(), "brew install --cask session-manager-plugin") {
		t.Errorf("Preflight = %v, want the install command", err)
	}
}

func TestPreflightOnAnExpiredSession(t *testing.T) {
	useFakeAWS(t, fakeAWS{stsFailure: "An error occurred (ExpiredToken) when calling the " +
		"GetCallerIdentity operation: The security token included in the request is expired"}, true)

	err := ssmDialer{}.Preflight(Config{Target: "i-0test", AWSProfile: "dev"})
	if err == nil || !strings.Contains(err.Error(), "aws sso login --profile dev") {
		t.Errorf("Preflight = %v, want the login command", err)
	}
}

/* ---------------- ssm-ssh through the fake tunnel ---------------- */

func dialSSMSSH(t *testing.T, fake fakeAWS) (Session, *sessionLog, string, string, error) {
	t.Helper()
	address, hostKey := fakeSSHD(t)
	return dialSSMSSHWith(t, fake, address, trustedKnownHosts(t, "i-0test", hostKey))
}

// dialSSMSSHWith dials through the fake tunnel to sshd, checking host keys
// against knownHostsPath.
func dialSSMSSHWith(t *testing.T, fake fakeAWS, sshd string, knownHostsPath string) (Session, *sessionLog, string, string, error) {
	t.Helper()
	config, argsLog, pidFile := prepareSSMSSH(t, fake, sshd, knownHostsPath)
	log := newSessionLog()
	session, err := ssmSSHDialer{}.Dial(context.Background(), config, Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit)
	return session, log, argsLog, pidFile, err
}

// prepareSSMSSH sets up the fake aws and returns the config to dial with.
// Apart from the dial itself because it changes the environment, which only
// the test's own goroutine may do.
func prepareSSMSSH(t *testing.T, fake fakeAWS, sshd string, knownHostsPath string) (config Config, argsLog string, pidFile string) {
	t.Helper()
	fake.sshd = sshd
	argsLog, pidFile = useFakeAWS(t, fake, true)

	config = Config{
		Kind:           KindSSMSSH,
		Target:         "i-0test",
		Username:       "ec2-user",
		AuthMethod:     AuthPassword,
		Password:       "unused: the test server takes anyone",
		KnownHostsPath: knownHostsPath,
	}
	// The pause is for a real server's queue; the fake has none.
	original := preAuthDropPause
	preAuthDropPause = 10 * time.Millisecond
	t.Cleanup(func() { preAuthDropPause = original })
	return config, argsLog, pidFile
}

func tunnelsOpened(t *testing.T, argsLog string) int {
	count := 0
	for _, line := range calls(t, argsLog) {
		if strings.Contains(line, "--document-name") {
			count++
		}
	}
	return count
}

// The whole ssm-ssh path but AWS: tunnel, handshake, known_hosts, pty, shell.
func TestSSMSSHThroughTheTunnel(t *testing.T) {
	session, log, _, pidFile, err := dialSSMSSH(t, fakeAWS{})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if _, err := session.Write([]byte("hello through the tunnel\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	log.waitFor(t, "hello through the tunnel")
	if err := session.Resize(Size{Cols: 100, Rows: 30}); err != nil {
		t.Errorf("Resize: %v", err)
	}

	content, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("tunnel pid: %v", err)
	}
	pid, _ := strconv.Atoi(string(content))
	_ = session.Close()

	// Closing the session ends the tunnel process too.
	if processRunning(pid) {
		t.Errorf("the tunnel process %d is still running after Close", pid)
	}
}

// MaxStartups through the tunnel: the first connection is turned away before
// authentication, the second goes through.
func TestSSMSSHRetriesADropBeforeAuth(t *testing.T) {
	session, _, argsLog, _, err := dialSSMSSH(t, fakeAWS{drops: 1})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	_ = session.Close()
	if opened := tunnelsOpened(t, argsLog); opened != 2 {
		t.Errorf("opened %d tunnels, want 2", opened)
	}
}

// AWS naming the problem means the tunnel failed, not sshd: one attempt, and
// the error says why.
func TestSSMSSHDoesNotRetryWhatAWSRefused(t *testing.T) {
	_, _, argsLog, _, err := dialSSMSSH(t, fakeAWS{drops: 10,
		tunnelStderr: "An error occurred (TargetNotConnected) when calling the StartSession operation"})
	if err == nil || !strings.Contains(err.Error(), "not reachable through Session Manager") {
		t.Fatalf("Dial = %v, want the TargetNotConnected explanation", err)
	}
	if opened := tunnelsOpened(t, argsLog); opened != 1 {
		t.Errorf("opened %d tunnels, want 1", opened)
	}
}

func TestSSMSSHGivesUpAfterThreeDrops(t *testing.T) {
	_, _, argsLog, _, err := dialSSMSSH(t, fakeAWS{drops: 10})
	if err == nil {
		t.Fatal("Dial succeeded through a tunnel that always hangs up")
	}
	if opened := tunnelsOpened(t, argsLog); opened != preAuthDropAttempts {
		t.Errorf("opened %d tunnels, want %d", opened, preAuthDropAttempts)
	}
}

// processAlive reports whether the process in pidFile is still running.
func processAlive(t *testing.T, pidFile string) bool {
	t.Helper()
	content, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatalf("tunnel pid: %v", err)
	}
	pid, _ := strconv.Atoi(string(content))
	return processRunning(pid)
}

// A tunnel that never answers must not hang the dial: the handshake deadline
// on processConn's pipes is what ends it, and the stuck aws goes with it.
func TestSSMSSHGivesUpOnASilentTunnel(t *testing.T) {
	original := tunnelHandshakeLimit
	tunnelHandshakeLimit = 300 * time.Millisecond
	t.Cleanup(func() { tunnelHandshakeLimit = original })

	address, hostKey := fakeSSHD(t)
	config, argsLog, pidFile := prepareSSMSSH(t, fakeAWS{silent: true}, address,
		trustedKnownHosts(t, "i-0test", hostKey))

	done := make(chan error, 1)
	go func() {
		_, err := ssmSSHDialer{}.Dial(context.Background(), config, Size{Cols: 80, Rows: 24},
			func([]byte) {}, func(error) {})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "timeout") {
			t.Fatalf("Dial = %v, want a timeout", err)
		}
		if processAlive(t, pidFile) {
			t.Error("the stuck tunnel process is still running")
		}
		// A timeout is not a drop before authentication: no retry.
		if opened := tunnelsOpened(t, argsLog); opened != 1 {
			t.Errorf("opened %d tunnels, want 1", opened)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Dial is still waiting on a tunnel that never answers")
	}
}

// First contact through the tunnel is a question for the user, not a failure:
// the app finds the key in the error and offers to trust it.
func TestSSMSSHAsksAboutAnUnknownHost(t *testing.T) {
	address, hostKey := fakeSSHD(t)
	empty := filepath.Join(t.TempDir(), "known_hosts")

	_, _, argsLog, _, err := dialSSMSSHWith(t, fakeAWS{}, address, empty)
	var unknown *UnknownHostKeyError
	if !errors.As(err, &unknown) {
		t.Fatalf("Dial = %v, want an UnknownHostKeyError the app can find", err)
	}
	if unknown.Host != "i-0test" || ssh.FingerprintSHA256(unknown.Key) != ssh.FingerprintSHA256(hostKey) {
		t.Errorf("asks about %s %s, want i-0test %s", unknown.Host,
			ssh.FingerprintSHA256(unknown.Key), ssh.FingerprintSHA256(hostKey))
	}
	if !strings.Contains(err.Error(), "has never been connected to") {
		t.Errorf("message = %q", err)
	}
	if opened := tunnelsOpened(t, argsLog); opened != 1 {
		t.Errorf("opened %d tunnels, want 1", opened)
	}
}

// A key that changed is never offered for trust, and never retried.
func TestSSMSSHRefusesAChangedHostKey(t *testing.T) {
	address, _ := fakeSSHD(t)
	_, otherKey := fakeSSHD(t)

	_, _, argsLog, _, err := dialSSMSSHWith(t, fakeAWS{}, address, trustedKnownHosts(t, "i-0test", otherKey))
	if err == nil || !strings.Contains(err.Error(), "has CHANGED") {
		t.Fatalf("Dial = %v, want the changed-key warning", err)
	}
	var unknown *UnknownHostKeyError
	if errors.As(err, &unknown) {
		t.Error("a changed key was offered for trust")
	}
	if opened := tunnelsOpened(t, argsLog); opened != 1 {
		t.Errorf("opened %d tunnels, want 1", opened)
	}
}

// AWS prints warnings on stderr. Kept out of the byte stream, they cost
// nothing; mixed into it, they would break the handshake at once.
func TestSSMSSHKeepsTunnelStderrOutOfTheStream(t *testing.T) {
	session, log, _, _, err := dialSSMSSH(t, fakeAWS{noise: "SSH-2.0-not-a-server\nWarning: something harmless\n"})
	if err != nil {
		t.Fatalf("Dial with a chatty tunnel: %v", err)
	}
	defer func() { _ = session.Close() }()
	if _, err := session.Write([]byte("still clean\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	log.waitFor(t, "still clean")
}

/* ---------------- kind ssh: the same retry, over TCP ---------------- */

func dialSSH(t *testing.T, drops int) (Session, *atomic.Int32, error) {
	t.Helper()
	address, hostKey, accepted := fakeSSHDDropping(t, drops)
	host, portText, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(portText)

	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	line := knownhosts.Line([]string{knownhosts.Normalize(address)}, hostKey)
	if err := os.WriteFile(knownHostsPath, []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("known_hosts: %v", err)
	}

	original := preAuthDropPause
	preAuthDropPause = 10 * time.Millisecond
	t.Cleanup(func() { preAuthDropPause = original })

	log := newSessionLog()
	session, err := sshDialer{}.Dial(context.Background(), Config{
		Kind: KindSSH, Target: host, Port: port, Username: "deploy",
		AuthMethod: AuthPassword, Password: "unused", KnownHostsPath: knownHostsPath,
	}, Size{Cols: 80, Rows: 24}, log.onOutput, log.onExit)
	return session, accepted, err
}

func TestSSHRetriesADropBeforeAuth(t *testing.T) {
	session, accepted, err := dialSSH(t, 1)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	_ = session.Close()
	if got := accepted.Load(); got != 2 {
		t.Errorf("connected %d times, want 2", got)
	}
}

func TestSSHGivesUpAfterThreeDropsAndSaysWhy(t *testing.T) {
	_, accepted, err := dialSSH(t, 10)
	if err == nil || !strings.Contains(err.Error(), "before the SSH handshake started") {
		t.Fatalf("Dial = %v, want the MaxStartups explanation", err)
	}
	if got := accepted.Load(); got != preAuthDropAttempts {
		t.Errorf("connected %d times, want %d", got, preAuthDropAttempts)
	}
}

// An address nothing listens on is not retried, and is not dressed up as a
// handshake problem.
func TestSSHUnreachableIsReportedAsIs(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	host, portText, _ := net.SplitHostPort(address)
	port, _ := strconv.Atoi(portText)

	_, err = sshDialer{}.Dial(context.Background(), Config{
		Kind: KindSSH, Target: host, Port: port, Username: "deploy",
		AuthMethod: AuthPassword, Password: "unused",
		KnownHostsPath: filepath.Join(t.TempDir(), "known_hosts"),
	}, Size{Cols: 80, Rows: 24}, func([]byte) {}, func(error) {})
	if err == nil || !strings.HasPrefix(err.Error(), "reach "+address) {
		t.Errorf("Dial = %v, want it to start with %q", err, "reach "+address)
	}
}
