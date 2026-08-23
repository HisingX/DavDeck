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

func TestSQLiteUserRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	database, _, err := Open(ctx, filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewUserRepository(database)
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	updated, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC))
	user := domain.User{ID: "11111111-1111-4111-8111-111111111111", Username: "Alice", UsernameNormalized: "alice", PasswordHash: "$2a$hash", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	if err := repository.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	if err := repository.Create(ctx, domain.User{ID: "22222222-2222-4222-8222-222222222222", Username: "ALICE", UsernameNormalized: "alice", PasswordHash: "$2a$other", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}); !errors.Is(err, app.ErrUserAlreadyExists) {
		t.Fatalf("duplicate error = %v", err)
	}
	users, err := repository.List(ctx)
	if err != nil || len(users) != 1 || users[0].PasswordHash != "$2a$hash" {
		t.Fatalf("users = %#v, err = %v", users, err)
	}
	if err := repository.SetEnabled(ctx, user.ID, false, updated); err != nil {
		t.Fatal(err)
	}
	if err := repository.SetPasswordHash(ctx, user.ID, "$2a$new", updated); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.Get(ctx, user.ID)
	if err != nil || stored.Enabled || stored.PasswordHash != "$2a$new" || stored.UpdatedAt.String() != updated.String() {
		t.Fatalf("stored = %#v, err = %v", stored, err)
	}
	if err := repository.Delete(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Get(ctx, user.ID); !errors.Is(err, app.ErrUserNotFound) {
		t.Fatalf("missing error = %v", err)
	}
	if err := repository.Delete(ctx, user.ID); !errors.Is(err, app.ErrUserNotFound) {
		t.Fatalf("second delete error = %v", err)
	}
}
