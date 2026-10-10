package files

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/sftp"
)

// testServer runs an SFTP server inside the test, on a pair of in-memory
// pipes, with root as its working directory — what a real server would call
// the user's home. No network, no SSH, no machine of yours involved.
func testServer(t *testing.T, root string) *sftp.Client {
	t.Helper()

	clientReads, serverWrites := io.Pipe()
	serverReads, clientWrites := io.Pipe()

	server, err := sftp.NewServer(struct {
		io.Reader
		io.WriteCloser
	}{serverReads, serverWrites}, sftp.WithServerWorkingDirectory(root))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	go func() { _ = server.Serve() }()

	client, err := sftp.NewClientPipe(clientReads, clientWrites)
	if err != nil {
		t.Fatalf("NewClientPipe: %v", err)
	}
	// Server first. Closing the client waits for its reader to stop, and the
	// reader only stops when the server's end of the pipe closes — the other
	// order waits forever.
	t.Cleanup(func() {
		_ = server.Close()
		_ = client.Close()
	})
	return client
}

// tree makes files and directories under root. A name ending in / is a
// directory; anything else a file holding its own name.
func tree(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		full := filepath.Join(root, filepath.FromSlash(name))
		if strings.HasSuffix(name, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", name, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func names(listing Listing) string {
	out := make([]string, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		out = append(out, entry.Name)
	}
	return strings.Join(out, " ")
}

func TestListStartsAtHomeWithDirectoriesFirst(t *testing.T) {
	root := t.TempDir()
	tree(t, root, "zeta.txt", "Alpha.txt", "logs/", "app/", "beta.txt")
	client := testServer(t, root)

	listing, err := List(client, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	// t.TempDir can sit under a symlink (/var → /private/var on macOS); the
	// server reports the path it was given, so compare against that.
	if listing.Path != filepath.ToSlash(root) {
		t.Errorf("path = %q, want the home directory %q", listing.Path, root)
	}
	if got, want := names(listing), "app logs Alpha.txt beta.txt zeta.txt"; got != want {
		t.Errorf("order = %q, want %q", got, want)
	}
	if listing.Parent != filepath.ToSlash(filepath.Dir(root)) {
		t.Errorf("parent = %q", listing.Parent)
	}
	for _, entry := range listing.Entries {
		if entry.Path != filepath.ToSlash(filepath.Join(root, entry.Name)) {
			t.Errorf("%s has path %q; paths must be absolute", entry.Name, entry.Path)
		}
	}
}

func TestListResolvesRelativePathsFromHome(t *testing.T) {
	root := t.TempDir()
	tree(t, root, "app/config/", "app/run.sh")
	client := testServer(t, root)

	listing, err := List(client, "app")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if listing.Path != filepath.ToSlash(filepath.Join(root, "app")) {
		t.Errorf("path = %q", listing.Path)
	}
	if got := names(listing); got != "config run.sh" {
		t.Errorf("entries = %q", got)
	}
}

func TestListFollowsLinksOnce(t *testing.T) {
	root := t.TempDir()
	tree(t, root, "releases/v1/", "notes.txt")
	for link, target := range map[string]string{
		"current": filepath.Join(root, "releases", "v1"),
		"readme":  filepath.Join(root, "notes.txt"),
		"broken":  filepath.Join(root, "no-such-thing"),
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Fatalf("symlink %s: %v", link, err)
		}
	}
	client := testServer(t, root)

	listing, err := List(client, "")
	if err != nil {
		t.Fatalf("List: %v — a broken link must not fail the whole listing", err)
	}

	byName := map[string]Entry{}
	for _, entry := range listing.Entries {
		byName[entry.Name] = entry
	}
	if current := byName["current"]; !current.IsLink || !current.IsDir {
		t.Errorf("a link to a directory: IsLink=%v IsDir=%v, want both true", current.IsLink, current.IsDir)
	}
	if readme := byName["readme"]; !readme.IsLink || readme.IsDir || readme.Size != int64(len("notes.txt")) {
		t.Errorf("a link to a file: %+v", readme)
	}
	if broken := byName["broken"]; !broken.IsLink || broken.IsDir {
		t.Errorf("a broken link: %+v", broken)
	}
	// A link to a directory sorts with the directories.
	if got := names(listing); !strings.HasPrefix(got, "current releases ") {
		t.Errorf("order = %q", got)
	}
}

func TestListAtTheRootHasNoParent(t *testing.T) {
	client := testServer(t, t.TempDir())

	listing, err := List(client, "/")
	if err != nil {
		t.Fatalf("List /: %v", err)
	}
	if listing.Parent != "" {
		t.Errorf("parent of / = %q, want none", listing.Parent)
	}
}

func TestListMissingDirectoryNamesIt(t *testing.T) {
	root := t.TempDir()
	client := testServer(t, root)

	_, err := List(client, "nowhere")
	if err == nil {
		t.Fatal("listing a directory that does not exist succeeded")
	}
	if !strings.Contains(err.Error(), "nowhere") {
		t.Errorf("the error does not say which directory: %v", err)
	}
}
