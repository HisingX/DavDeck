// Package storage contains the SQLite persistence adapter and migration runner.
package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"davdeck.dev/davdeck/core/migrations"
	_ "modernc.org/sqlite"
)

const migrationTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
)`

const migrationIntegrityVersion = 2

type migration struct {
	version  int
	name     string
	checksum string
	body     []byte
}

// Open creates or opens the authoritative SQLite database and applies pending
// migrations. A migration failure is returned without recreating the database.
func Open(ctx context.Context, path string) (*sql.DB, int, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, 0, fmt.Errorf("create database directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return nil, 0, fmt.Errorf("secure database directory: %w", err)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, 0, fmt.Errorf("database path must be a regular file")
		}
	} else if !os.IsNotExist(err) {
		return nil, 0, fmt.Errorf("inspect database path: %w", err)
	}
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, 0, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, 0, fmt.Errorf("ping database: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, 0, fmt.Errorf("secure database file: %w", err)
	}
	var foreignKeys int
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		db.Close()
		if err != nil {
			return nil, 0, fmt.Errorf("verify foreign keys: %w", err)
		}
		return nil, 0, fmt.Errorf("verify foreign keys: pragma is disabled")
	}
	version, err := RunMigrations(ctx, db, migrations.Files)
	if err != nil {
		db.Close()
		return nil, 0, fmt.Errorf("run migrations: %w", err)
	}
	return db, version, nil
}

// RunMigrations applies numbered .sql files in ascending order, one transaction
// per migration, and returns the current schema version.
func RunMigrations(ctx context.Context, db *sql.DB, files fs.FS) (int, error) {
	if _, err := db.ExecContext(ctx, migrationTable); err != nil {
		return 0, fmt.Errorf("initialize migration history: %w", err)
	}
	migrations, err := readMigrations(files)
	if err != nil {
		return 0, err
	}
	current := 0
	if err := db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&current); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	integrityAvailable, err := migrationIntegrityAvailable(ctx, db)
	if err != nil {
		return current, err
	}
	if current >= migrationIntegrityVersion && !integrityAvailable {
		return current, fmt.Errorf("migration history is missing integrity columns")
	}
	if integrityAvailable {
		if err := validateMigrationHistory(ctx, db, migrations, current); err != nil {
			return current, err
		}
	}
	for _, item := range migrations {
		if item.version <= current {
			continue
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return current, fmt.Errorf("begin migration %s: %w", item.name, err)
		}
		if _, err = tx.ExecContext(ctx, string(item.body)); err == nil {
			integrityAvailable, err = migrationIntegrityAvailableTx(ctx, tx)
			if err == nil && item.version >= migrationIntegrityVersion && !integrityAvailable {
				err = fmt.Errorf("migration %s did not enable migration integrity", item.name)
			}
			if err == nil {
				err = recordMigration(ctx, tx, item, integrityAvailable)
			}
		}
		if err != nil {
			_ = tx.Rollback()
			return current, fmt.Errorf("apply migration %s: %w", item.name, err)
		}
		if err := tx.Commit(); err != nil {
			return current, fmt.Errorf("commit migration %s: %w", item.name, err)
		}
		current = item.version
	}
	if integrityAvailable, err = migrationIntegrityAvailable(ctx, db); err != nil {
		return current, err
	} else if integrityAvailable {
		if err := validateMigrationHistory(ctx, db, migrations, current); err != nil {
			return current, err
		}
	}
	return current, nil
}

func readMigrations(files fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var result []migration
	seen := make(map[int]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil || version < 1 {
			return nil, fmt.Errorf("invalid migration filename %q", entry.Name())
		}
		if previous, exists := seen[version]; exists {
			return nil, fmt.Errorf("duplicate migration version %d in %q and %q", version, previous, entry.Name())
		}
		body, err := fs.ReadFile(files, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		sum := sha256.Sum256(body)
		seen[version] = entry.Name()
		result = append(result, migration{version: version, name: entry.Name(), checksum: hex.EncodeToString(sum[:]), body: body})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("no migrations found")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	for index, item := range result {
		if item.version != index+1 {
			return nil, fmt.Errorf("migration versions must be contiguous starting at 1")
		}
	}
	return result, nil
}

func migrationIntegrityAvailable(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('schema_migrations') WHERE name IN ('name', 'checksum')").Scan(&count); err != nil {
		return false, fmt.Errorf("inspect migration history: %w", err)
	}
	return count == 2, nil
}

func migrationIntegrityAvailableTx(ctx context.Context, tx *sql.Tx) (bool, error) {
	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('schema_migrations') WHERE name IN ('name', 'checksum')").Scan(&count); err != nil {
		return false, fmt.Errorf("inspect migration history: %w", err)
	}
	return count == 2, nil
}

func recordMigration(ctx context.Context, tx *sql.Tx, item migration, integrityAvailable bool) error {
	if integrityAvailable {
		_, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at, name, checksum) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'), ?, ?)", item.version, item.name, item.checksum)
		return err
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, applied_at) VALUES (?, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))", item.version)
	return err
}

func validateMigrationHistory(ctx context.Context, db *sql.DB, migrations []migration, current int) error {
	known := make(map[int]migration, len(migrations))
	for _, item := range migrations {
		known[item.version] = item
	}
	rows, err := db.QueryContext(ctx, "SELECT version, name, checksum FROM schema_migrations ORDER BY version")
	if err != nil {
		return fmt.Errorf("read migration integrity: %w", err)
	}
	defer rows.Close()
	applied := make(map[int]struct{}, current)
	for rows.Next() {
		var version int
		var name, checksum sql.NullString
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			return fmt.Errorf("scan migration integrity: %w", err)
		}
		item, exists := known[version]
		if !exists {
			return fmt.Errorf("database contains unknown migration version %d", version)
		}
		if !name.Valid || !checksum.Valid || name.String != item.name || checksum.String != item.checksum {
			return fmt.Errorf("migration integrity check failed for version %d", version)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate migration integrity: %w", err)
	}
	for version := 1; version <= current; version++ {
		if _, exists := applied[version]; !exists {
			return fmt.Errorf("migration history is missing version %d", version)
		}
	}
	return nil
}
