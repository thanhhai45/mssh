package store

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migration is one numbered step. The number comes from the filename prefix,
// so the order on disk is the order they run in.
type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads every file in migrations/ and returns them in order.
// It fails loudly on a badly named or missing file: a broken migration set is
// a programmer error and should stop the app at startup, not later.
func loadMigrations() ([]migration, error) {
	names, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}

	out := make([]migration, 0, len(names))
	for _, name := range names {
		base := path.Base(name)
		prefix, _, found := strings.Cut(base, "_")

		if !found {
			return nil, fmt.Errorf("migration %q must be named <number>_<description>.sql", base)
		}

		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %q has a non-numeric prefix: %w", base, err)
		}

		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", base, err)
		}

		out = append(out, migration{version: version, name: base, sql: string(body)})
	}

	slices.SortFunc(out, func(a, b migration) int { return a.version - b.version })

	// Numbering has to be 1, 2, 3… with no gaps and no duplicates. A gap means
	// a file was lost; a duplicate means two branches picked the same number.
	for i, m := range out {
		if m.version != i+1 {
			return nil, fmt.Errorf(
				"migrations must be numbered 1,2,3… without gaps; found %s at position %d",
				m.name, i+1)
		}
	}

	return out, nil
}

// migrate applies every migration newer than the version recorded in the
// database file, inside a single transaction.
func (s *Store) migrate() error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		return fmt.Errorf("no migrations found")
	}
	latest := migrations[len(migrations)-1].version

	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	if version > latest {
		return fmt.Errorf(
			"database is at schema v%d but this build only knows v%d; update mssh",
			version, latest)
	}
	if version == latest {
		return nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()

	for _, m := range migrations {
		if m.version <= version {
			continue
		}
		if _, err := tx.Exec(m.sql); err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
	}

	// PRAGMA does not accept bound parameters, so Sprintf is unavoidable.
	// Safe here because latest comes from filenames compiled into the binary.
	if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", latest)); err != nil {
		return fmt.Errorf("record schema version: %w", err)
	}

	return tx.Commit()
}
