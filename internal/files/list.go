// Packages files is what the file browser does with a remote machine: list a
// directory, and move files to and from it. It works on an *sftp.Client and
// knows nothing of where that came from - opening one on a live session is
// the transport's job.
package files

import (
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/pkg/sftp"
)

// Entry is one item in a remote directory
type Entry struct {
	Name string `json:"name"`
	// Path is absolute, so the frontend never has to join paths itself  and
	// never with the wrong separator: remote paths are POSIX even when mssh
	// runs on Windows
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	// IsLink marks a symbolic link. IsDir then describes what it points to,
	// so a link to a directory can be opened like one.
	IsLink     bool   `json:"isLink"`
	Size       int64  `json:"size"`
	ModifiedAt int64  `json:"modifiedAt"`
	Mode       string `json:"mode"`
}

// Listing is a directory and what is in it
type Listing struct {
	Path string `json:"path"`
	// Parent is the directory one level up, or "" at the root
	Parent  string  `json:"parent"`
	Entries []Entry `json:"entries"`
}

// List reads one remote directory. An empty dir means the directory the SFTP
// session starts in - the user's home on the server. A relative one is taken
// from there too.
//
// Directories come first, the files, each sorted by name ignoring case: the
// order a file manager uses, and stable from on reac to the next.
func List(client *sftp.Client, dir string) (Listing, error) {
	home, err := client.Getwd()
	if err != nil {
		return Listing{}, fmt.Errorf("find the remote home directory: %w", err)
	}

	switch {
	case dir == "":
		dir = home
	case !path.IsAbs(dir):
		dir = path.Join(home, dir)
	default:
		dir = path.Clean(dir)
	}

	infos, err := client.ReadDir(dir)
	if err != nil {
		return Listing{}, fmt.Errorf("list %s: %w", dir, err)
	}

	entries := make([]Entry, 0, len(infos))
	for _, info := range infos {
		entries = append(entries, entryFor(client, dir, info))
	}

	sort.Slice(entries, func(first int, second int) bool {
		left, right := entries[first], entries[second]
		if left.IsDir != right.IsDir {
			return left.IsDir
		}
		return strings.ToLower(left.Name) < strings.ToLower(right.Name)
	})

	listing := Listing{Path: dir, Entries: entries}
	if dir != "/" {
		listing.Parent = path.Dir(dir)
	}
	return listing, nil
}

// entryFor describes one item. A symbolic link is followed once, so its entry
// says whether it leads to a directory; a link whose target is gone stays a
// plain link rather than failing the whole listing
func entryFor(client *sftp.Client, dir string, info os.FileInfo) Entry {
	entry := Entry{
		Name:       info.Name(),
		Path:       path.Join(dir, info.Name()),
		IsDir:      info.IsDir(),
		Size:       info.Size(),
		ModifiedAt: info.ModTime().Unix(),
		Mode:       info.Mode().String(),
	}

	if info.Mode()&os.ModeSymlink != 0 {
		entry.IsLink = true
		if target, err := client.Stat(entry.Path); err == nil {
			entry.IsDir = target.IsDir()
			if !target.IsDir() {
				entry.Size = target.Size()
			}
		}
	}
	return entry
}
