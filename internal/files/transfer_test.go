package files

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// recorder keeps every report a Transfers makes, and can act on one as it
// arrives — which is how a test cancels a transfer partway through.
type recorder struct {
	mutex   sync.Mutex
	seen    []Transfer
	onEvent func(Transfer)
}

func (r *recorder) record(info Transfer) {
	r.mutex.Lock()
	r.seen = append(r.seen, info)
	act := r.onEvent
	r.mutex.Unlock()
	if act != nil {
		act(info)
	}
}

func (r *recorder) last() Transfer {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.seen[len(r.seen)-1]
}

func newTransfers() (*Transfers, *recorder) {
	r := &recorder{}
	transfers := NewTransfers(r.record)
	transfers.interval = 0 // report every piece, so a test can stop one partway
	return transfers, r
}

func randomBytes(t *testing.T, size int) []byte {
	t.Helper()
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("random: %v", err)
	}
	return data
}

// leftovers lists the hidden temporary files a transfer might leave behind.
func leftovers(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var found []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".part") || strings.HasSuffix(entry.Name(), ".mssh-part") {
			found = append(found, entry.Name())
		}
	}
	return found
}

func TestDownload(t *testing.T) {
	remote, local := t.TempDir(), t.TempDir()
	content := randomBytes(t, 300*1024)
	if err := os.WriteFile(filepath.Join(remote, "backup.tar"), content, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	client := testServer(t, remote)
	transfers, events := newTransfers()

	destination := filepath.Join(local, "backup.tar")
	transfers.Download(client, "s1", filepath.ToSlash(filepath.Join(remote, "backup.tar")), destination)
	transfers.Wait()

	final := events.last()
	if final.State != TransferDone || final.Done != int64(len(content)) || final.Total != int64(len(content)) {
		t.Fatalf("final report = %+v", final)
	}
	got, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("downloaded file differs (err %v)", err)
	}
	if found := leftovers(t, local); len(found) != 0 {
		t.Errorf("temporary files left behind: %v", found)
	}
}

func TestUploadReplacesAnExistingFile(t *testing.T) {
	remote, local := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(remote, "app.conf"), []byte("old"), 0o644); err != nil {
		t.Fatalf("seed remote: %v", err)
	}
	content := randomBytes(t, 200*1024)
	source := filepath.Join(local, "app.conf")
	if err := os.WriteFile(source, content, 0o644); err != nil {
		t.Fatalf("seed local: %v", err)
	}
	client := testServer(t, remote)
	transfers, events := newTransfers()

	started := transfers.Upload(client, "s1", source, filepath.ToSlash(remote))
	transfers.Wait()

	if final := events.last(); final.State != TransferDone {
		t.Fatalf("final report = %+v", final)
	}
	if started.Destination != filepath.ToSlash(filepath.Join(remote, "app.conf")) {
		t.Errorf("destination = %q", started.Destination)
	}
	got, err := os.ReadFile(filepath.Join(remote, "app.conf"))
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("the remote file was not replaced (err %v)", err)
	}
	if found := leftovers(t, remote); len(found) != 0 {
		t.Errorf("temporary files left on the server: %v", found)
	}
}

// Cancelled partway, a download leaves nothing behind: no half file under the
// real name, and no temporary one either.
func TestCancelledDownloadLeavesNothing(t *testing.T) {
	remote, local := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(remote, "big.bin"), randomBytes(t, 8*1024*1024), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	client := testServer(t, remote)
	transfers, events := newTransfers()
	events.onEvent = func(info Transfer) {
		if info.State == TransferRunning && info.Done > 0 {
			transfers.Cancel(info.ID)
		}
	}

	destination := filepath.Join(local, "big.bin")
	transfers.Download(client, "s1", filepath.ToSlash(filepath.Join(remote, "big.bin")), destination)
	transfers.Wait()

	if final := events.last(); final.State != TransferCancelled {
		t.Fatalf("final state = %q, want cancelled", final.State)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Errorf("a half-downloaded file exists under the real name (err %v)", err)
	}
	if found := leftovers(t, local); len(found) != 0 {
		t.Errorf("temporary files left behind: %v", found)
	}
}

// The same on the server: a cancelled upload leaves the old file as it was and
// no hidden part file beside it.
func TestCancelledUploadLeavesTheServerAsItWas(t *testing.T) {
	remote, local := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(remote, "big.bin"), []byte("previous version"), 0o644); err != nil {
		t.Fatalf("seed remote: %v", err)
	}
	source := filepath.Join(local, "big.bin")
	if err := os.WriteFile(source, randomBytes(t, 8*1024*1024), 0o644); err != nil {
		t.Fatalf("seed local: %v", err)
	}
	client := testServer(t, remote)
	transfers, events := newTransfers()
	events.onEvent = func(info Transfer) {
		if info.State == TransferRunning && info.Done > 0 {
			transfers.Cancel(info.ID)
		}
	}

	transfers.Upload(client, "s1", source, filepath.ToSlash(remote))
	transfers.Wait()

	if final := events.last(); final.State != TransferCancelled {
		t.Fatalf("final state = %q, want cancelled", final.State)
	}
	if got, _ := os.ReadFile(filepath.Join(remote, "big.bin")); string(got) != "previous version" {
		t.Errorf("the server's file was changed by a cancelled upload")
	}
	if found := leftovers(t, remote); len(found) != 0 {
		t.Errorf("temporary files left on the server: %v", found)
	}
}

func TestFailuresSayWhatAndLeaveNothing(t *testing.T) {
	remote, local := t.TempDir(), t.TempDir()
	client := testServer(t, remote)
	transfers, events := newTransfers()

	transfers.Download(client, "s1", "/no/such/file", filepath.Join(local, "file"))
	transfers.Wait()
	if final := events.last(); final.State != TransferFailed || !strings.Contains(final.Message, "/no/such/file") {
		t.Errorf("download of a missing file: %+v", final)
	}
	if entries, _ := os.ReadDir(local); len(entries) != 0 {
		t.Errorf("a failed download left files: %v", entries)
	}

	source := filepath.Join(local, "notes.txt")
	if err := os.WriteFile(source, []byte("hello"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	transfers.Upload(client, "s1", source, filepath.ToSlash(filepath.Join(remote, "missing-dir")))
	transfers.Wait()
	if final := events.last(); final.State != TransferFailed {
		t.Errorf("upload into a missing directory: %+v", final)
	}
	if entries, _ := os.ReadDir(remote); len(entries) != 0 {
		t.Errorf("a failed upload left files on the server: %v", entries)
	}
}

func TestListShowsEveryTransferNewestFirst(t *testing.T) {
	remote, local := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(remote, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	client := testServer(t, remote)
	transfers, _ := newTransfers()

	first := transfers.Download(client, "s1", filepath.ToSlash(filepath.Join(remote, "a.txt")), filepath.Join(local, "a.txt"))
	transfers.Wait()

	listed := transfers.List()
	if len(listed) != 1 || listed[0].ID != first.ID || listed[0].State != TransferDone {
		t.Errorf("List = %+v", listed)
	}
}

// holdAtFirstProgress stops the first transfer of each named session at its
// first progress report, until release is closed, so a test can act while
// those transfers are certainly partway through. reached receives each
// session's id as it is held.
func holdAtFirstProgress(events *recorder, sessions ...string) (reached chan string, release chan struct{}) {
	reached = make(chan string, len(sessions))
	release = make(chan struct{})
	var mutex sync.Mutex
	waiting := map[string]bool{}
	for _, session := range sessions {
		waiting[session] = true
	}
	events.onEvent = func(info Transfer) {
		if info.State != TransferRunning || info.Done == 0 {
			return
		}
		mutex.Lock()
		hold := waiting[info.SessionID]
		delete(waiting, info.SessionID)
		mutex.Unlock()
		if hold {
			reached <- info.SessionID
			<-release
		}
	}
	return reached, release
}

func stateOf(transfers *Transfers, id string) string {
	for _, info := range transfers.List() {
		if info.ID == id {
			return info.State
		}
	}
	return ""
}

// Closing one tab must not stop what another tab is moving. Both transfers are
// held partway, so a CancelSession that matched too much would catch the other.
func TestCancelSessionStopsOnlyThatSession(t *testing.T) {
	remote, local := t.TempDir(), t.TempDir()
	for _, name := range []string{"one.bin", "two.bin"} {
		if err := os.WriteFile(filepath.Join(remote, name), randomBytes(t, 8*1024*1024), 0o644); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	client := testServer(t, remote)
	transfers, events := newTransfers()
	// s1 is held, so its cleanup cannot finish while CancelSession waits: this
	// also checks that the wait gives up instead of hanging.
	transfers.cleanupWait = 50 * time.Millisecond
	reached, release := holdAtFirstProgress(events, "s1", "s2")

	one := transfers.Download(client, "s1", filepath.ToSlash(filepath.Join(remote, "one.bin")), filepath.Join(local, "one.bin"))
	two := transfers.Download(client, "s2", filepath.ToSlash(filepath.Join(remote, "two.bin")), filepath.Join(local, "two.bin"))
	<-reached
	<-reached

	began := time.Now()
	transfers.CancelSession("s1")
	if waited := time.Since(began); waited > 2*time.Second {
		t.Errorf("CancelSession waited %v on a transfer that could not finish", waited)
	}
	close(release)
	transfers.Wait()

	if state := stateOf(transfers, one.ID); state != TransferCancelled {
		t.Errorf("s1's transfer: state %q, want cancelled", state)
	}
	if state := stateOf(transfers, two.ID); state != TransferDone {
		t.Errorf("s2's transfer: state %q, want done", state)
	}
	if _, err := os.Stat(filepath.Join(local, "one.bin")); !os.IsNotExist(err) {
		t.Errorf("the cancelled download exists under its real name (err %v)", err)
	}
}

// Cancelling returns only once the cleanup is done - checked straight after,
// without Wait - because the caller closes the session next, and after that
// the server cannot be reached to remove the partial upload.
func TestCancelWaitsForTheCleanup(t *testing.T) {
	remote, local := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(remote, "big.bin"), []byte("previous version"), 0o644); err != nil {
		t.Fatalf("seed remote: %v", err)
	}
	source := filepath.Join(local, "big.bin")
	if err := os.WriteFile(source, randomBytes(t, 8*1024*1024), 0o644); err != nil {
		t.Fatalf("seed local: %v", err)
	}
	client := testServer(t, remote)
	transfers, events := newTransfers()
	reached, release := holdAtFirstProgress(events, "s1")

	started := transfers.Upload(client, "s1", source, filepath.ToSlash(remote))
	<-reached
	// CancelSession in two steps, so the transfer is released only after it
	// has been cancelled: the wait then has real cleanup to wait for.
	waiting := transfers.cancelWhere(func(info Transfer) bool { return info.SessionID == "s1" })
	close(release)
	transfers.wait(waiting)

	if state := stateOf(transfers, started.ID); state != TransferCancelled {
		t.Errorf("state %q, want cancelled", state)
	}
	if found := leftovers(t, remote); len(found) != 0 {
		t.Errorf("cancel returned before the cleanup: %v still on the server", found)
	}
	if got, _ := os.ReadFile(filepath.Join(remote, "big.bin")); string(got) != "previous version" {
		t.Errorf("the server's file was changed by a cancelled upload")
	}
}
