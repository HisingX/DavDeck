package storage

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

func TestSQLiteShareRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	database, _, err := Open(ctx, filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewShareRepository(database)
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	updated, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC))
	share := domain.Share{ID: "11111111-1111-4111-8111-111111111111", Name: "Documents", Slug: "documents", Path: "/srv/documents", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	if err := repository.Create(ctx, share); err != nil {
		t.Fatal(err)
	}
	duplicate := share
	duplicate.ID = "22222222-2222-4222-8222-222222222222"
	if err := repository.Create(ctx, duplicate); !errors.Is(err, app.ErrShareAlreadyExists) {
		t.Fatalf("duplicate error = %v", err)
	}
	shares, err := repository.List(ctx)
	if err != nil || len(shares) != 1 {
		t.Fatalf("shares = %#v, err = %v", shares, err)
	}
	share.Name, share.Path, share.Enabled, share.UpdatedAt = "Team", "/srv/team", false, updated
	if err := repository.Update(ctx, share); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.Get(ctx, share.ID)
	if err != nil || stored.Name != "Team" || stored.Enabled {
		t.Fatalf("stored = %#v, err = %v", stored, err)
	}
	if err := repository.Delete(ctx, share.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Get(ctx, share.ID); !errors.Is(err, app.ErrShareNotFound) {
		t.Fatalf("missing error = %v", err)
	}
}
