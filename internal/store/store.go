package store

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

// schemaVersion is the version that schema.sql produces. Bump it and add a
// migration step in migrate() whenever schema.sql changes.
const schemaVersion = 1

// Store owns the SQLite connection pool. It is the only thing in the app that
// talks to the database.
type Store struct {
	db *sql.DB

	now func() int64
}

// DefaultPath returns the per-user database location, e.g.
// ~/Library/Application Support/mssh/mssh.db on macOS.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config dir: %w", err)
	}
	return filepath.Join(dir, "mssh", "mssh.db"), nil
}

// Open creates the database if it does not exist, applies migrations, and
// seeds a default workspace on first run.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
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
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	s := &Store{
		db:  db,
		now: func() int64 { return time.Now().Unix() },
	}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	if err := s.seed(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the connection pool.
func (s *Store) Close() error {
	return s.db.Close()
}

// migrate brings the database up to schemaVersion, recording progress in
// SQLite's built-in user_version field.
func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	switch {
	case version == schemaVersion:
		return nil
	case version > schemaVersion:
		return fmt.Errorf(
			"database is schema v%d but this build only understands v%d; update mssh",
			version, schemaVersion,
		)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()

	if version < 1 {
		if _, err := tx.Exec(schemaSQL); err != nil {
			return fmt.Errorf("apply schema v1: %w", err)
		}
	}
	// Future migrations go here, each guarded by `if version < N`.

	// PRAGMA does not accept bound parameters, so Sprintf is unavoidable.
	// Safe here because schemaVersion is a compile-time constant.
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion)); err != nil {
		return fmt.Errorf("record schema version: %w", err)
	}

	return tx.Commit()
}

// seed inserts a starter workspace so a fresh install is never an empty screen.
func (s *Store) seed() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&count); err != nil {
		return fmt.Errorf("count workspaces: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := s.now()
	_, err := s.db.Exec(
		`INSERT INTO workspaces (id, name, color, aws_profile, aws_region, sort_order, created_at, updated_at)
		 VALUES (?, ?, ?, '', '', 0, ?, ?)`,
		uuid.NewString(), "Default", "slate", now, now,
	)
	if err != nil {
		return fmt.Errorf("seed default workspace: %w", err)
	}
	return nil
}
