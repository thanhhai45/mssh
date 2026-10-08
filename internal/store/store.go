package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Store struct {
	db  *sql.DB
	now func() int64
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(dir, "mssh", "mssh.db"), nil
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	// Pragmas live in the DSN so the driver re-applies them to every pooled
	// connection. Setting them once with a plain Exec would only affect
	// whichever connection happened to serve that call.
	dsn := fmt.Sprintf(
		"file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)",
		path,
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	s := &Store{
		db:  db,
		now: func() int64 { return time.Now().Unix() },
	}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.seed(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := restrictToOwner(path); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// restrictToOwner makes the database readable by its owner only.
//
// The directory is 0700 already, and that is what keeps other users out. This
// is the second lock: the file now holds passwords and AWS keys, so it should
// not depend on nobody ever loosening the directory. The -wal and -shm files
// hold the same data as the main file and get the same treatment; SQLite gives
// any it creates later the main file's permissions.
//
// On Windows, Chmod can only toggle read-only and 0600 leaves the file
// writable; access there is governed by the profile directory's ACL instead.
func restrictToOwner(path string) error {
	for _, file := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Chmod(file, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("restrict database permissions: %w", err)
		}
	}
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// seed inserts a starter workspace so a fresh install is never an empty screen.
func (s *Store) seed() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&count); err != nil {
		return fmt.Errorf("count workspaces: %v", err)
	}
	if count > 0 {
		return nil
	}

	now := s.now()
	_, err := s.db.Exec(
		`INSERT INTO workspaces (id, name, color, aws_profile, aws_region, sort_order, created_at, updated_at)
		VALUES(?, ?, ?, '', '', 0, ?, ?)`, uuid.NewString(), "Default", "slate", now, now,
	)
	if err != nil {
		return fmt.Errorf("insert default workspace: %v", err)
	}
	return nil
}
