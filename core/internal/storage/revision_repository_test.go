package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

func TestRevisionRepositoryTracksDesiredActiveAndDirtyState(t *testing.T) {
	ctx := context.Background()
	database, _, err := Open(ctx, filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewRevisionRepository(database)
	state, err := repository.State(ctx)
	if err != nil || !state.Dirty || !state.Pending {
		t.Fatalf("initial state = %#v, err = %v", state, err)
	}
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	body := []byte("{}\n")
	revision := domain.ConfigRevision{ID: "11111111-1111-4111-8111-111111111111", CreatedAt: stamp, ConfigJSON: body, ConfigHash: domain.HashConfigJSON(body), ValidationStatus: domain.RevisionValidationValid, ApplyStatus: domain.RevisionApplyNotApplied, AppVersion: "test"}
	revision, err = repository.Create(ctx, revision)
	if err != nil || revision.Number != 1 {
		t.Fatalf("revision = %#v, err = %v", revision, err)
	}
	state, _ = repository.State(ctx)
	if state.Dirty || !state.Pending || state.DesiredRevision == nil || *state.DesiredRevision != 1 || state.ActiveRevision != nil {
		t.Fatalf("desired state = %#v", state)
	}
	if err := repository.MarkApplied(ctx, revision.ID, stamp); err != nil {
		t.Fatal(err)
	}
	state, _ = repository.State(ctx)
	if state.Pending || state.ActiveRevision == nil || *state.ActiveRevision != 1 {
		t.Fatalf("active state = %#v", state)
	}
	user := domain.User{ID: "22222222-2222-4222-8222-222222222222", Username: "Alice", UsernameNormalized: "alice", PasswordHash: "hash", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	if err := NewUserRepository(database).Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	state, _ = repository.State(ctx)
	if !state.Dirty || !state.Pending {
		t.Fatalf("mutated state = %#v", state)
	}
	revisions, err := repository.List(ctx)
	if err != nil || len(revisions) != 1 || revisions[0].ApplyStatus != domain.RevisionApplyApplied {
		t.Fatalf("revisions = %#v, err = %v", revisions, err)
	}
}

func TestRevisionRepositoryRestoresDesiredAndActiveRevision(t *testing.T) {
	ctx := context.Background()
	database, _, err := Open(ctx, filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	repository := NewRevisionRepository(database)
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	body := []byte("{}\n")
	first := domain.ConfigRevision{ID: "11111111-1111-4111-8111-111111111111", CreatedAt: stamp, ConfigJSON: body, ConfigHash: domain.HashConfigJSON(body), ValidationStatus: domain.RevisionValidationValid, ApplyStatus: domain.RevisionApplyNotApplied, AppVersion: "test"}
	first, err = repository.Create(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkApplied(ctx, first.ID, stamp); err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = "22222222-2222-4222-8222-222222222222"
	second, err = repository.Create(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if err := repository.MarkRestored(ctx, first.ID, stamp); err != nil {
		t.Fatal(err)
	}
	state, err := repository.State(ctx)
	if err != nil || state.Pending || state.DesiredRevision == nil || *state.DesiredRevision != first.Number || state.ActiveRevision == nil || *state.ActiveRevision != first.Number {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
	_ = second
}

func TestSnapshotRepositoryBuildsCanonicalDomainSnapshot(t *testing.T) {
	ctx := context.Background()
	database, _, err := Open(ctx, filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	user := domain.User{ID: "11111111-1111-4111-8111-111111111111", Username: "Alice", UsernameNormalized: "alice", PasswordHash: "hash", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	share := domain.Share{ID: "22222222-2222-4222-8222-222222222222", Name: "Team", Slug: "team", Path: "/srv/team", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	if err := NewUserRepository(database).Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	if err := NewShareRepository(database).Create(ctx, share); err != nil {
		t.Fatal(err)
	}
	if err := NewPermissionRepository(database).Set(ctx, domain.SharePermission{ShareID: share.ID, UserID: user.ID, Permission: domain.PermissionRead, CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := NewSnapshotRepository(database).Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ServerSettings.PublicBasePath != "/dav" || len(snapshot.Users) != 1 || len(snapshot.Shares) != 1 || len(snapshot.Shares[0].Permissions) != 1 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}
