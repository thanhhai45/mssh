package files

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pkg/sftp"
)

// Which way a transfer goes.
const (
	Download = "download"
	Upload   = "upload"
)

// Where a transfer is.
const (
	TransferRunning   = "running"
	TransferDone      = "done"
	TransferFailed    = "failed"
	TransferCancelled = "cancelled"
)

// Transfer is one file on its way, as the frontend sees it.
type Transfer struct {
	ID          string `json:"id"`
	SessionID   string `json:"sessionId"`
	Direction   string `json:"direction"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Done        int64  `json:"done"`
	Total       int64  `json:"total"`
	State       string `json:"state"`
	Message     string `json:"message"`
	StartedAt   int64  `json:"startedAt"`
}

type job struct {
	info       Transfer
	cancel     context.CancelFunc
	lastReport time.Time
	// done is closed once the transfer has finished and cleaned up after itself.
	done chan struct{}
}

// Transfers runs file transfers in the background and reports each change.
//
// They live here, in Go, rather than in the page that started them: a
// transfer has to survive the user navigating away, the same lesson as the
// terminals, which do not belong to React either.
type Transfers struct {
	mutex    sync.Mutex
	jobs     map[string]*job
	running  sync.WaitGroup
	onChange func(Transfer)
	// interval spaces out progress reports. A large file read in 32 KB pieces
	// would otherwise send tens of thousands of events.
	interval time.Duration
	// cleanupWait bounds how long cancelling waits. Removing a partial upload
	// needs the server, and on a dead connection that call does not return
	// until the session is closed - which is what the caller is waiting to do.
	cleanupWait time.Duration
}

// NewTransfers makes a registry that calls onChange with a copy of a transfer
// whenever it starts, moves on by at least interval, or finishes. onChange
// runs on the transfer's own goroutine.
func NewTransfers(onChange func(Transfer)) *Transfers {
	return &Transfers{
		jobs:        make(map[string]*job),
		onChange:    onChange,
		interval:    100 * time.Millisecond,
		cleanupWait: 5 * time.Second,
	}
}

// Download copies a remote file to localPath. It returns at once; the copy
// runs in the background.
func (transfers *Transfers) Download(client *sftp.Client, sessionID string, remotePath string,
	localPath string) Transfer {
	return transfers.start(sessionID, Download, remotePath, localPath,
		func(ctx context.Context, report func(int64), setTotal func(int64)) error {
			return download(ctx, client, remotePath, localPath, report, setTotal)
		})
}

// Upload copies a local file into remoteDir, keeping its name, and replacing
// a file of that name if there is one. It returns at once.
func (transfers *Transfers) Upload(client *sftp.Client, sessionID string, localPath string,
	remoteDir string) Transfer {
	destination := path.Join(remoteDir, filepath.Base(localPath))
	return transfers.start(sessionID, Upload, localPath, destination,
		func(ctx context.Context, report func(int64), setTotal func(int64)) error {
			return upload(ctx, client, localPath, destination, report, setTotal)
		})
}

func (transfers *Transfers) start(
	sessionID string,
	direction string,
	source string,
	destination string,
	work func(ctx context.Context, report func(int64), setTotal func(int64)) error,
) Transfer {
	ctx, cancel := context.WithCancel(context.Background())
	info := Transfer{
		ID:          uuid.NewString(),
		SessionID:   sessionID,
		Direction:   direction,
		Name:        path.Base(filepath.ToSlash(source)),
		Source:      source,
		Destination: destination,
		State:       TransferRunning,
		StartedAt:   time.Now().Unix(),
	}

	finished := make(chan struct{})
	transfers.mutex.Lock()
	transfers.jobs[info.ID] = &job{info: info, cancel: cancel, done: finished}
	transfers.mutex.Unlock()
	transfers.notify(info)

	transfers.running.Add(1)
	go func() {
		defer transfers.running.Done()
		defer cancel()

		err := work(ctx,
			func(done int64) { transfers.progress(info.ID, done) },
			func(total int64) { transfers.setTotal(info.ID, total) })
		// Cancelled only if it stopped because of it: a cancel that arrives
		// after the last byte changes nothing, and the file is in place.
		transfers.finish(info.ID, err, err != nil && ctx.Err() != nil)
		close(finished)
	}()
	return info
}

func (transfers *Transfers) setTotal(id string, total int64) {
	transfers.mutex.Lock()
	if current := transfers.jobs[id]; current != nil {
		current.info.Total = total
	}
	transfers.mutex.Unlock()
}

func (transfers *Transfers) progress(id string, done int64) {
	transfers.mutex.Lock()
	current := transfers.jobs[id]
	if current == nil {
		transfers.mutex.Unlock()
		return
	}
	current.info.Done = done
	if time.Since(current.lastReport) < transfers.interval {
		transfers.mutex.Unlock()
		return
	}
	current.lastReport = time.Now()
	snapshot := current.info
	transfers.mutex.Unlock()

	transfers.notify(snapshot)
}

func (transfers *Transfers) finish(id string, err error, cancelled bool) {
	transfers.mutex.Lock()
	current := transfers.jobs[id]
	switch {
	case cancelled:
		current.info.State = TransferCancelled
	case err != nil:
		current.info.State = TransferFailed
		current.info.Message = err.Error()
	default:
		current.info.State = TransferDone
		current.info.Done = current.info.Total
	}
	snapshot := current.info
	transfers.mutex.Unlock()

	transfers.notify(snapshot)
}

func (transfers *Transfers) notify(info Transfer) {
	if transfers.onChange != nil {
		transfers.onChange(info)
	}
}

// Cancel stops a transfer that is still running. What it had written is
// removed; the destination is left as it was before the transfer began.
func (transfers *Transfers) Cancel(id string) {
	transfers.mutex.Lock()
	current := transfers.jobs[id]
	transfers.mutex.Unlock()
	if current != nil {
		current.cancel()
	}
}

// CancelSession stops the transfers of one session and waits for them to clean
// up after themselves. Run before that session closes, so the cleanup can still
// reach the server. It waits at most cleanupWait.
func (transfers *Transfers) CancelSession(sessionID string) {
	transfers.wait(transfers.cancelWhere(func(info Transfer) bool {
		return info.SessionID == sessionID
	}))
}

// CancelAll is CancelSession for every session, on the way out of the app.
func (transfers *Transfers) CancelAll() {
	transfers.wait(transfers.cancelWhere(func(Transfer) bool { return true }))
}

// cancelWhere cancels the matching transfers and returns what to wait on. A
// finished transfer's channel is already closed, so waiting on it costs nothing.
func (transfers *Transfers) cancelWhere(matches func(Transfer) bool) []chan struct{} {
	transfers.mutex.Lock()
	defer transfers.mutex.Unlock()

	var waiting []chan struct{}
	for _, current := range transfers.jobs {
		if matches(current.info) {
			current.cancel()
			waiting = append(waiting, current.done)
		}
	}
	return waiting
}

// wait blocks until every channel is closed, or until cleanupWait has passed.
func (transfers *Transfers) wait(waiting []chan struct{}) {
	deadline := time.NewTimer(transfers.cleanupWait)
	defer deadline.Stop()
	for _, done := range waiting {
		select {
		case <-done:
		case <-deadline.C:
			return
		}
	}
}

// Wait blocks until no transfer is running.
func (transfers *Transfers) Wait() {
	transfers.running.Wait()
}

// List returns every transfer this run has seen, newest first, so a page that
// was reloaded can show what is still going.
func (transfers *Transfers) List() []Transfer {
	transfers.mutex.Lock()
	defer transfers.mutex.Unlock()

	out := make([]Transfer, 0, len(transfers.jobs))
	for _, current := range transfers.jobs {
		out = append(out, current.info)
	}
	sort.Slice(out, func(first int, second int) bool {
		if out[first].StartedAt != out[second].StartedAt {
			return out[first].StartedAt > out[second].StartedAt
		}
		return out[first].ID < out[second].ID
	})
	return out
}

/* ---------------- the copying itself ---------------- */

// progressWriter counts what passes through it and stops the copy once the
// transfer is cancelled. Cancelling cannot interrupt a copy from outside; a
// write that refuses to continue is what makes it stop.
type progressWriter struct {
	ctx    context.Context
	out    io.Writer
	done   int64
	report func(int64)
}

func (writer *progressWriter) Write(chunk []byte) (int, error) {
	if err := writer.ctx.Err(); err != nil {
		return 0, err
	}
	written, err := writer.out.Write(chunk)
	writer.done += int64(written)
	writer.report(writer.done)
	return written, err
}

// progressReader is the same for the other direction. Size tells sftp how
// much is coming, which lets it send several pieces at once.
type progressReader struct {
	ctx    context.Context
	in     io.Reader
	size   int64
	done   int64
	report func(int64)
}

func (reader *progressReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	read, err := reader.in.Read(buffer)
	reader.done += int64(read)
	reader.report(reader.done)
	return read, err
}

func (reader *progressReader) Size() int64 { return reader.size }

// download writes to a hidden temporary file beside localPath and renames it
// into place only once every byte is there. A failure or a cancel removes the
// temporary file, so localPath is never left half written.
func download(
	ctx context.Context,
	client *sftp.Client,
	remotePath string,
	localPath string,
	report func(int64),
	setTotal func(int64),
) error {
	remote, err := client.Open(remotePath)
	if err != nil {
		return fmt.Errorf("open %s: %w", remotePath, err)
	}
	defer func() { _ = remote.Close() }()

	if info, err := remote.Stat(); err == nil {
		setTotal(info.Size())
	}

	temporary, err := os.CreateTemp(filepath.Dir(localPath), "."+filepath.Base(localPath)+".*.part")
	if err != nil {
		return fmt.Errorf("create a file beside %s: %w", localPath, err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = temporary.Close()
			_ = os.Remove(temporary.Name())
		}
	}()

	if _, err := remote.WriteTo(&progressWriter{ctx: ctx, out: temporary, report: report}); err != nil {
		return fmt.Errorf("download %s: %w", remotePath, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("write %s: %w", localPath, err)
	}
	if err := os.Rename(temporary.Name(), localPath); err != nil {
		return fmt.Errorf("save %s: %w", localPath, err)
	}
	finished = true
	return nil
}

// upload is download in the other direction: a hidden temporary file beside
// the destination on the server, renamed into place at the end and removed on
// any failure, so the server never holds a half-written file under the real
// name.
func upload(
	ctx context.Context,
	client *sftp.Client,
	localPath string,
	destination string,
	report func(int64),
	setTotal func(int64),
) error {
	local, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", localPath, err)
	}
	defer func() { _ = local.Close() }()

	info, err := local.Stat()
	if err != nil {
		return fmt.Errorf("read %s: %w", localPath, err)
	}
	setTotal(info.Size())

	temporary := path.Join(path.Dir(destination), "."+path.Base(destination)+".mssh-part")
	remote, err := client.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return fmt.Errorf("create %s: %w", temporary, err)
	}
	finished := false
	defer func() {
		if !finished {
			_ = remote.Close()
			// Best effort: if the connection is what failed, this cannot
			// reach the server either, and the hidden file stays behind.
			_ = client.Remove(temporary)
		}
	}()

	if _, err := remote.ReadFrom(&progressReader{ctx: ctx, in: local, size: info.Size(), report: report}); err != nil {
		return fmt.Errorf("upload %s: %w", localPath, err)
	}
	if err := remote.Close(); err != nil {
		return fmt.Errorf("upload %s: %w", localPath, err)
	}

	// posix-rename replaces an existing file in one step. Servers without the
	// extension get the plain SFTP rename, which refuses to overwrite, so the
	// old file is removed first.
	if err := client.PosixRename(temporary, destination); err != nil {
		if removeErr := client.Remove(destination); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("replace %s: %w", destination, removeErr)
		}
		if err := client.Rename(temporary, destination); err != nil {
			return fmt.Errorf("move %s into place: %w", destination, err)
		}
	}
	finished = true
	return nil
}
