package store

import (
	"database/sql"
	"path/filepath"
	"regexp"
	"testing"
)

// writeV1Database builds a database in the pre-password shape by running only
// the first migration — the same file that shipped when v1 was current.
func writeV1Database(t *testing.T, path string) {
	t.Helper()

	body, err := migrationsFS.ReadFile("migrations/001_initial.sql")
	if err != nil {
		t.Fatalf("read migration 001: %v", err)
	}

	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.Exec(string(body)); err != nil {
		t.Fatalf("apply migration 001: %v", err)
	}
	if _, err := db.Exec(`PRAGMA user_version = 1`); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO workspaces (id, name, color, aws_profile, aws_region, sort_order, created_at, updated_at)
		 VALUES ('w1', 'Prod', 'violet', 'prod', 'ap-southeast-1', 0, 100, 100)`,
	); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO connections (id, workspace_id, name, kind, target, port, username,
		                          auth_method, key_path, aws_profile, aws_region, extra,
		                          color, sort_order, created_at, updated_at)
		 VALUES ('c1', 'w1', 'Taptanh', 'ssh', '115.73.222.79', 22, 'thinhvu',
		         'agent', '', '', '', '{}', '', 0, 100, 100)`,
	); err != nil {
		t.Fatalf("insert connection: %v", err)
	}
}

func TestLoadMigrations(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(migrations) < 2 {
		t.Fatalf("found %d migrations, want at least 2", len(migrations))
	}
	for i, m := range migrations {
		if m.version != i+1 {
			t.Errorf("migration %s has version %d at position %d", m.name, m.version, i+1)
		}
		if m.sql == "" {
			t.Errorf("migration %s is empty", m.name)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v1.db")
	writeV1Database(t, path)

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	_ = first.Close()

	second, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer func() { _ = second.Close() }()

	if _, err := second.GetConnection("c1"); err != nil {
		t.Errorf("connection lost on the second open: %v", err)
	}
}

// dropsParentTable matches a DROP of a table that other tables cascade from.
// The \b after the name keeps connections_new — 003's temporary table — and
// connection_secrets from matching.
var dropsParentTable = regexp.MustCompile(
	`(?i)\bDROP\s+TABLE\s+(IF\s+EXISTS\s+)?(connections|workspaces)\b`)

// TestNoMigrationDropsAParentTable guards against the rebuild 003 did.
//
// With foreign keys on, DROP TABLE runs an implicit DELETE first, and the
// cascade empties every child: rebuild connections and every saved password is
// gone, with no error. PRAGMA foreign_keys cannot be switched off from inside
// a migration, because migrate runs them all in one transaction.
//
// 003 is exempt: when it ran, nothing referenced connections yet. If a rebuild
// is ever truly needed, change migrate to switch foreign keys off around it —
// see docs/CREDENTIALS.md, vòng 1 — rather than exempting the migration here.
func TestNoMigrationDropsAParentTable(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}

	for _, m := range migrations {
		if m.version <= 3 {
			continue
		}
		if found := dropsParentTable.FindString(m.sql); found != "" {
			t.Errorf("%s contains %q, which would cascade-delete every child row",
				m.name, found)
		}
	}
}
