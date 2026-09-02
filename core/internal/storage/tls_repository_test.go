package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

func TestSQLiteTLSRepositoryUpsertsSingleProfileAndMarksRuntimeDirty(t *testing.T) {
	ctx := context.Background()
	database, _, err := Open(ctx, filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewTLSRepository(database)
	if _, found, err := repository.Get(ctx); err != nil || found {
		t.Fatalf("found = %t, err = %v", found, err)
	}
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	profile := domain.TLSProfile{ID: "11111111-1111-4111-8111-111111111111", Mode: domain.TLSModeInternal, Hostname: "dav.local", Challenge: domain.TLSChallengeAuto, CreatedAt: stamp, UpdatedAt: stamp}
	if _, err := database.Exec(`UPDATE runtime_state SET dirty = 0 WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(ctx, profile); err != nil {
		t.Fatal(err)
	}
	stored, found, err := repository.Get(ctx)
	if err != nil || !found || stored != profile {
		t.Fatalf("stored = %#v, found = %t, err = %v", stored, found, err)
	}
	var dirty, count int
	if err := database.QueryRow(`SELECT dirty FROM runtime_state WHERE id = 1`).Scan(&dirty); err != nil || dirty != 1 {
		t.Fatalf("dirty = %d, err = %v", dirty, err)
	}
	updated, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC))
	profile.Mode, profile.Hostname, profile.UpdatedAt = domain.TLSModeAutomatic, "dav.example.com", updated
	if err := repository.Save(ctx, profile); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRow(`SELECT COUNT(*) FROM tls_profiles`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count = %d, err = %v", count, err)
	}
	if err := repository.Delete(ctx); err != nil {
		t.Fatal(err)
	}
	if _, found, err := repository.Get(ctx); err != nil || found {
		t.Fatalf("after delete: found = %t, err = %v", found, err)
	}
}
