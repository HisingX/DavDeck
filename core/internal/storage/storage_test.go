package storage

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"davdeck.dev/davdeck/core/migrations"
	_ "modernc.org/sqlite"
)

const latestSchemaVersion = 9

func TestFreshMigrationCreatesCoreSchemaAndConstraints(t *testing.T) {
	t.Parallel()
	database, version, err := Open(context.Background(), filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if version != latestSchemaVersion {
		t.Fatalf("version = %d, want %d", version, latestSchemaVersion)
	}
	assertForeignKeysEnabled(t, database)
	assertTables(t, database, "app_metadata", "schema_migrations", "users", "shares", "share_permissions", "server_settings", "tls_profiles", "config_revisions", "revision_sequence", "audit_events", "dns_provider_credentials", "dns_provider_secrets")

	const now = "2026-08-20T00:00:00Z"
	if _, err := database.Exec("INSERT INTO users(id, username, username_normalized, password_hash, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "user-1", "Alice", "alice", "hash", 1, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO users(id, username, username_normalized, password_hash, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "user-2", "Alice 2", "alice", "hash", 1, now, now); err == nil {
		t.Fatal("expected normalized username uniqueness violation")
	}
	if _, err := database.Exec("INSERT INTO shares(id, name, slug, path, enabled, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)", "share-1", "Photos", "photos", "/srv/photos", 1, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO share_permissions(share_id, user_id, permission, created_at, updated_at) VALUES (?, ?, ?, ?, ?)", "share-1", "user-1", "READ", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec("INSERT INTO share_permissions(share_id, user_id, permission, created_at, updated_at) VALUES (?, ?, ?, ?, ?)", "missing", "user-1", "READ", now, now); err == nil {
		t.Fatal("expected foreign key violation")
	}
	if _, err := database.Exec("INSERT INTO share_permissions(share_id, user_id, permission, created_at, updated_at) VALUES (?, ?, ?, ?, ?)", "share-1", "user-1", "WRITE", now, now); err == nil {
		t.Fatal("expected permission check constraint violation")
	}
	if _, err := database.Exec("INSERT INTO dns_provider_credentials(id, name, provider, created_at, updated_at) VALUES (?, ?, ?, ?, ?)", "dns-legacy", "DNSPod legacy", "dnspod", now, now); err != nil {
		t.Fatalf("expected legacy DNSPod provider to be accepted: %v", err)
	}
	assertMigrationHistory(t, database)
}

func TestMigrationUpgradesExistingBootstrapDatabase(t *testing.T) {
	t.Parallel()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	bootstrapFiles := migrationSubset(t, "0001_bootstrap.sql")
	version, err := RunMigrations(context.Background(), database, bootstrapFiles)
	if err != nil || version != 1 {
		t.Fatalf("bootstrap migration: version=%d error=%v", version, err)
	}
	version, err = RunMigrations(context.Background(), database, migrations.Files)
	if err != nil || version != latestSchemaVersion {
		t.Fatalf("upgrade migration: version=%d error=%v", version, err)
	}
	assertTables(t, database, "users", "shares", "config_revisions")
	assertMigrationHistory(t, database)
}

func TestMigrationUpgradesEveryHistoricalSchemaAndPreservesMetadata(t *testing.T) {
	for startingVersion := 1; startingVersion < latestSchemaVersion; startingVersion++ {
		t.Run("from_v"+strconv.Itoa(startingVersion), func(t *testing.T) {
			database, err := sql.Open("sqlite", ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer database.Close()
			initial := migrationPrefix(t, startingVersion)
			if version, err := RunMigrations(context.Background(), database, initial); err != nil || version != startingVersion {
				t.Fatalf("initial migration version=%d error=%v", version, err)
			}
			marker := "2026-08-20T00:00:0" + strconv.Itoa(startingVersion) + "Z"
			if _, err := database.Exec(`UPDATE app_metadata SET created_at = ? WHERE singleton = 1`, marker); err != nil {
				t.Fatal(err)
			}
			if version, err := RunMigrations(context.Background(), database, migrations.Files); err != nil || version != latestSchemaVersion {
				t.Fatalf("upgrade version=%d error=%v", version, err)
			}
			var actual string
			if err := database.QueryRow(`SELECT created_at FROM app_metadata WHERE singleton = 1`).Scan(&actual); err != nil || actual != marker {
				t.Fatalf("preserved marker=%q error=%v", actual, err)
			}
			assertMigrationHistory(t, database)
			if version, err := RunMigrations(context.Background(), database, migrations.Files); err != nil || version != latestSchemaVersion {
				t.Fatalf("idempotent rerun version=%d error=%v", version, err)
			}
		})
	}
}

func TestRuntimeStateMigrationPreservesExistingSettings(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	firstThree := migrationSubset(t, "0001_bootstrap.sql", "0002_migration_integrity.sql", "0003_core_schema.sql")
	if version, err := RunMigrations(context.Background(), database, firstThree); err != nil || version != 3 {
		t.Fatalf("version = %d, err = %v", version, err)
	}
	const stamp = "2026-08-20T00:00:00Z"
	if _, err := database.Exec(`INSERT INTO server_settings(id, public_base_path, http_port, https_port, runtime_mode, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, "11111111-1111-4111-8111-111111111111", "/custom", 9080, 9443, "service", stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if version, err := RunMigrations(context.Background(), database, migrations.Files); err != nil || version != latestSchemaVersion {
		t.Fatalf("version = %d, err = %v", version, err)
	}
	var basePath string
	var count int
	if err := database.QueryRow(`SELECT COUNT(*), MIN(public_base_path) FROM server_settings`).Scan(&count, &basePath); err != nil {
		t.Fatal(err)
	}
	if count != 1 || basePath != "/custom" {
		t.Fatalf("settings count = %d, base path = %q", count, basePath)
	}
}

func TestMigrationIntegrityRejectsModifiedAppliedMigration(t *testing.T) {
	t.Parallel()
	database, _, err := Open(context.Background(), filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	altered := migrationSubset(t, "0001_bootstrap.sql", "0002_migration_integrity.sql", "0003_core_schema.sql", "0004_runtime_state.sql", "0005_tls_runtime_dirty.sql", "0006_revision_sequence.sql", "0007_revision_state_snapshot.sql", "0008_dns_provider_credentials.sql", "0009_dnspod_token_provider.sql")
	altered["0003_core_schema.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE changed (id INTEGER);\n")}
	if _, err := RunMigrations(context.Background(), database, altered); err == nil || !strings.Contains(err.Error(), "integrity check failed") {
		t.Fatalf("error = %v, want migration integrity failure", err)
	}
}

func TestMigrationRejectsDuplicateVersions(t *testing.T) {
	t.Parallel()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	files := fstest.MapFS{
		"0001_first.sql":  &fstest.MapFile{Data: []byte("CREATE TABLE first_table (id INTEGER);\n")},
		"0001_second.sql": &fstest.MapFile{Data: []byte("CREATE TABLE second_table (id INTEGER);\n")},
	}
	if _, err := RunMigrations(context.Background(), database, files); err == nil || !strings.Contains(err.Error(), "duplicate migration version") {
		t.Fatalf("error = %v, want duplicate version failure", err)
	}
}

func TestMigrationRejectsVersionGaps(t *testing.T) {
	t.Parallel()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	files := fstest.MapFS{"0002_skipped.sql": &fstest.MapFile{Data: []byte("CREATE TABLE skipped_table (id INTEGER);\n")}}
	if _, err := RunMigrations(context.Background(), database, files); err == nil || !strings.Contains(err.Error(), "contiguous") {
		t.Fatalf("error = %v, want contiguous version failure", err)
	}
}

func TestFailedMigrationRollsBackHistory(t *testing.T) {
	t.Parallel()
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	files := fstest.MapFS{"0001_broken.sql": &fstest.MapFile{Data: []byte("CREATE TABLE partial (id INTEGER); INVALID SQL;")}}
	if _, err := RunMigrations(context.Background(), database, files); err == nil {
		t.Fatal("expected migration failure")
	}
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("migration history count = %d, want 0", count)
	}
	var partialTable int
	if err := database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'partial'").Scan(&partialTable); err != nil {
		t.Fatal(err)
	}
	if partialTable != 0 {
		t.Fatal("failed migration left a partial table")
	}
}

func TestOpenDoesNotRecreateCorruptDatabase(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "davdeck.db")
	original := []byte("not a sqlite database")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open(context.Background(), path); err == nil {
		t.Fatal("expected corrupt database error")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != string(original) {
		t.Fatal("corrupt database contents changed")
	}
}

func assertForeignKeysEnabled(t *testing.T, database *sql.DB) {
	t.Helper()
	var enabled int
	if err := database.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatal(err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", enabled)
	}
}

func assertTables(t *testing.T, database *sql.DB, names ...string) {
	t.Helper()
	for _, name := range names {
		var count int
		if err := database.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("table %q does not exist", name)
		}
	}
}

func assertMigrationHistory(t *testing.T, database *sql.DB) {
	t.Helper()
	var count int
	if err := database.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE name IS NOT NULL AND checksum IS NOT NULL").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != latestSchemaVersion {
		t.Fatalf("integrity migration count = %d, want %d", count, latestSchemaVersion)
	}
}

func migrationSubset(t *testing.T, names ...string) fstest.MapFS {
	t.Helper()
	result := make(fstest.MapFS, len(names))
	for _, name := range names {
		body, err := fs.ReadFile(migrations.Files, name)
		if err != nil {
			t.Fatal(err)
		}
		result[name] = &fstest.MapFile{Data: body}
	}
	return result
}

func migrationPrefix(t *testing.T, count int) fstest.MapFS {
	t.Helper()
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	result := make(fstest.MapFS, count)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, _ := strings.Cut(entry.Name(), "_")
		version, err := strconv.Atoi(prefix)
		if err != nil {
			t.Fatal(err)
		}
		if version <= count {
			body, err := fs.ReadFile(migrations.Files, entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			result[entry.Name()] = &fstest.MapFile{Data: body}
		}
	}
	return result
}
