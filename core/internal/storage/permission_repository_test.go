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

func TestSQLitePermissionRepositorySetUpdateDelete(t *testing.T) {
	ctx := context.Background()
	database, _, err := Open(ctx, filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	updated, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC))
	user := domain.User{ID: "11111111-1111-4111-8111-111111111111", Username: "Alice", UsernameNormalized: "alice", PasswordHash: "hash", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	if err := NewUserRepository(database).Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	share := domain.Share{ID: "22222222-2222-4222-8222-222222222222", Name: "Team", Slug: "team", Path: "/srv/team", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	if err := NewShareRepository(database).Create(ctx, share); err != nil {
		t.Fatal(err)
	}
	repository := NewPermissionRepository(database)
	value := domain.SharePermission{ShareID: share.ID, UserID: user.ID, Permission: domain.PermissionRead, CreatedAt: stamp, UpdatedAt: stamp}
	if err := repository.Set(ctx, value); err != nil {
		t.Fatal(err)
	}
	value.Permission, value.UpdatedAt = domain.PermissionReadWrite, updated
	if err := repository.Set(ctx, value); err != nil {
		t.Fatal(err)
	}
	values, err := repository.ListByShare(ctx, share.ID)
	if err != nil || len(values) != 1 || values[0].Permission != domain.PermissionReadWrite || values[0].CreatedAt.String() != stamp.String() {
		t.Fatalf("values = %#v, err = %v", values, err)
	}
	if err := repository.Delete(ctx, share.ID, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := repository.Delete(ctx, share.ID, user.ID); !errors.Is(err, app.ErrPermissionNotFound) {
		t.Fatalf("delete error = %v", err)
	}
}
