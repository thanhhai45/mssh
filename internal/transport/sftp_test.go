package transport

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// clientEnd is the client's side of the pipes. Closing it closes both
// directions, as closing an SSH channel does: without that, a client closed
// while its server is still running waits forever for the server to hang up.
type clientEnd struct {
	*io.PipeWriter
	reads *io.PipeReader
}

func (end clientEnd) Close() error {
	_ = end.reads.Close()
	return end.PipeWriter.Close()
}

// pipeClient is an SFTP client talking to a server inside the test, over
// in-memory pipes: a real client, with no SSH connection under it.
func pipeClient(t *testing.T) *sftp.Client {
	t.Helper()
	serverReads, clientWrites := io.Pipe()
	clientReads, serverWrites := io.Pipe()

	server, err := sftp.NewServer(struct {
		io.Reader
		io.WriteCloser
	}{serverReads, serverWrites}, sftp.WithServerWorkingDirectory(t.TempDir()))
	if err != nil {
		t.Fatalf("sftp server: %v", err)
	}
	go func() { _ = server.Serve() }()

	client, err := sftp.NewClientPipe(clientReads, clientEnd{clientWrites, clientReads})
	if err != nil {
		t.Fatalf("sftp client: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})
	return client
}

// opener stands in for openSFTP and counts its calls. Each call gets a fresh
// client; gate, when set, holds every call until it is closed.
type opener struct {
	t      *testing.T
	mutex  sync.Mutex
	calls  int
	opened []*sftp.Client
	err    error
	inside chan struct{}
	gate   chan struct{}
}

func useOpener(t *testing.T, fake *opener) {
	t.Helper()
	fake.t = t
	original := openSFTP
	openSFTP = fake.open
	t.Cleanup(func() { openSFTP = original })
}

func (fake *opener) open(*ssh.Client) (*sftp.Client, error) {
	fake.mutex.Lock()
	fake.calls++
	fake.mutex.Unlock()

	if fake.inside != nil {
		fake.inside <- struct{}{}
	}
	if fake.gate != nil {
		<-fake.gate
	}
	if fake.err != nil {
		return nil, fake.err
	}

	client := pipeClient(fake.t)
	fake.mutex.Lock()
	fake.opened = append(fake.opened, client)
	fake.mutex.Unlock()
	return client, nil
}

func (fake *opener) callCount() int {
	fake.mutex.Lock()
	defer fake.mutex.Unlock()
	return fake.calls
}

// works reports whether a client still answers: a closed one does not.
func works(client *sftp.Client) bool {
	_, err := client.Getwd()
	return err == nil
}

// Every listing and transfer of a session shares one SFTP channel.
func TestSFTPIsOpenedOnceAndShared(t *testing.T) {
	fake := &opener{}
	useOpener(t, fake)
	session := &sshSession{}

	first, err := session.SFTP()
	if err != nil {
		t.Fatalf("first SFTP: %v", err)
	}
	second, err := session.SFTP()
	if err != nil {
		t.Fatalf("second SFTP: %v", err)
	}
	if first != second {
		t.Error("the second call opened another client instead of sharing the first")
	}
	if calls := fake.callCount(); calls != 1 {
		t.Errorf("opened %d times, want 1", calls)
	}
}

// Two first calls at once may both open one; only one may survive.
func TestTwoFirstCallsKeepOneClient(t *testing.T) {
	fake := &opener{inside: make(chan struct{}, 2), gate: make(chan struct{})}
	useOpener(t, fake)
	session := &sshSession{}

	results := make(chan *sftp.Client, 2)
	for range 2 {
		go func() {
			client, err := session.SFTP()
			if err != nil {
				t.Errorf("SFTP: %v", err)
			}
			results <- client
		}()
	}
	<-fake.inside
	<-fake.inside
	close(fake.gate)
	first, second := <-results, <-results

	if first == nil || first != second {
		t.Fatal("the two callers got different clients")
	}
	for _, client := range fake.opened {
		if client != first && works(client) {
			t.Error("the client that lost the race was left open")
		}
	}
}

// Closing the session closes its file client, and no new one opens after.
func TestCloseClosesTheFileClient(t *testing.T) {
	fake := &opener{}
	useOpener(t, fake)
	session := &sshSession{}

	client, err := session.SFTP()
	if err != nil {
		t.Fatalf("SFTP: %v", err)
	}
	session.closeSFTP()

	if works(client) {
		t.Error("the file client still works after the session closed")
	}
	if _, err := session.SFTP(); !errors.Is(err, errSessionClosed) {
		t.Errorf("SFTP after close: %v, want errSessionClosed", err)
	}
	if calls := fake.callCount(); calls != 1 {
		t.Errorf("a closed session opened another client (%d opens)", calls)
	}
}

// A client still opening when the session closes must not outlive it.
func TestAClientOpenedDuringCloseIsClosed(t *testing.T) {
	fake := &opener{inside: make(chan struct{}, 1), gate: make(chan struct{})}
	useOpener(t, fake)
	session := &sshSession{}

	done := make(chan error, 1)
	go func() {
		_, err := session.SFTP()
		done <- err
	}()
	<-fake.inside
	session.closeSFTP()
	close(fake.gate)

	if err := <-done; !errors.Is(err, errSessionClosed) {
		t.Errorf("SFTP during close: %v, want errSessionClosed", err)
	}
	for _, client := range fake.opened {
		if works(client) {
			t.Error("a client opened during close was left open")
		}
	}
}

// A failure says so, and is not remembered: the next call tries again.
func TestAFailedOpenIsReportedAndRetried(t *testing.T) {
	fake := &opener{err: errors.New("subsystem request failed")}
	useOpener(t, fake)
	session := &sshSession{}

	_, err := session.SFTP()
	if err == nil || !strings.Contains(err.Error(), "open sftp: subsystem request failed") {
		t.Fatalf("SFTP: %v", err)
	}
	fake.err = nil
	if _, err := session.SFTP(); err != nil {
		t.Errorf("second SFTP: %v", err)
	}
	if calls := fake.callCount(); calls != 2 {
		t.Errorf("opened %d times, want 2", calls)
	}
}

// The kinds with a connection to share have files; the others do not.
func TestWhichSessionsBrowseFiles(t *testing.T) {
	var withConnection Session = &sshSession{}
	if _, ok := withConnection.(FileBrowser); !ok {
		t.Error("an ssh session cannot browse files")
	}
	var program Session = &ptySession{}
	if _, ok := program.(FileBrowser); ok {
		t.Error("a program session claims to browse files")
	}
}
