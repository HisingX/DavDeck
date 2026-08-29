package integration

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/app"
	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/domain"
	"davdeck.dev/davdeck/core/internal/storage"
)

func TestApplyWorkflowWithPinnedRuntime(t *testing.T) {
	binary := os.Getenv("DAVDECK_CADDY_BINARY")
	if binary == "" {
		t.Skip("set DAVDECK_CADDY_BINARY to run Apply integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	directory := t.TempDir()
	database, _, err := storage.Open(ctx, filepath.Join(directory, "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	httpPort := freePort(t)
	httpsPort := freePort(t)
	if _, err := database.ExecContext(ctx, `UPDATE server_settings SET http_port = ?, https_port = ?`, httpPort, httpsPort); err != nil {
		t.Fatal(err)
	}
	stamp, _ := domain.NewTimestamp(time.Now().UTC())
	hasher := app.BcryptHasher{}
	hash, err := hasher.Hash("integration password")
	if err != nil {
		t.Fatal(err)
	}
	user := domain.User{ID: "11111111-1111-4111-8111-111111111111", Username: "alice", UsernameNormalized: "alice", PasswordHash: hash, Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	share := domain.Share{ID: "22222222-2222-4222-8222-222222222222", Name: "Temporary", Slug: "temporary", Path: t.TempDir(), Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	users, shares := storage.NewUserRepository(database), storage.NewShareRepository(database)
	if err := users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	if err := shares.Create(ctx, share); err != nil {
		t.Fatal(err)
	}
	if err := storage.NewPermissionRepository(database).Set(ctx, domain.SharePermission{ShareID: share.ID, UserID: user.ID, Permission: domain.PermissionReadWrite, CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
		t.Fatal(err)
	}
	admin, err := caddyruntime.NewAdminClient("http://127.0.0.1:2019")
	if err != nil {
		t.Fatal(err)
	}
	validator := caddyruntime.BinaryValidator{BinaryPath: binary, TempDirectory: directory}
	runtime := caddyruntime.NewRuntimeManager(binary, filepath.Join(directory, "caddy.json"), validator, admin, io.Discard, io.Discard)
	defer func() {
		stop, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		if err := runtime.Stop(stop); err != nil {
			t.Errorf("stop runtime: %v", err)
		}
	}()
	service := app.NewApplyService(storage.NewSnapshotRepository(database), caddyruntime.Compiler{}, validator, runtime, storage.NewRevisionRepository(database), app.CryptoIDGenerator{}, app.SystemClock{}, "integration")
	first, err := service.Apply(ctx)
	if err != nil || first.Number != 1 || first.ApplyStatus != domain.RevisionApplyApplied {
		t.Fatalf("first = %#v, err = %v", first, err)
	}
	state, err := service.State(ctx)
	if err != nil || state.Pending {
		t.Fatalf("state = %#v, err = %v", state, err)
	}
	if err := users.SetEnabled(ctx, user.ID, false, stamp); err != nil {
		t.Fatal(err)
	}
	state, _ = service.State(ctx)
	if !state.Pending || !state.Dirty {
		t.Fatalf("dirty state = %#v", state)
	}
	second, err := service.Apply(ctx)
	if err != nil || second.Number != 2 || second.ApplyStatus != domain.RevisionApplyApplied {
		t.Fatalf("second = %#v, err = %v", second, err)
	}
	state, _ = service.State(ctx)
	if state.Pending || state.ActiveRevision == nil || *state.ActiveRevision != 2 {
		t.Fatalf("final state = %#v", state)
	}
}

func TestApplyRestoreRestoresCompleteApplicationState(t *testing.T) {
	ctx := context.Background()
	database, _, err := storage.Open(ctx, filepath.Join(t.TempDir(), "davdeck.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC))
	user := domain.User{ID: "11111111-1111-4111-8111-111111111111", Username: "alice", UsernameNormalized: "alice", PasswordHash: "hash", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	share := domain.Share{ID: "22222222-2222-4222-8222-222222222222", Name: "Temporary", Slug: "temporary", Path: t.TempDir(), Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	users := storage.NewUserRepository(database)
	if err := users.Create(ctx, user); err != nil {
		t.Fatal(err)
	}
	if err := storage.NewShareRepository(database).Create(ctx, share); err != nil {
		t.Fatal(err)
	}
	if err := storage.NewPermissionRepository(database).Set(ctx, domain.SharePermission{ShareID: share.ID, UserID: user.ID, Permission: domain.PermissionReadWrite, CreatedAt: stamp, UpdatedAt: stamp}); err != nil {
		t.Fatal(err)
	}
	runtime := &statefulTestRuntime{}
	service := app.NewApplyService(storage.NewSnapshotRepository(database), caddyruntime.Compiler{}, noOpTestValidator{}, runtime, storage.NewRevisionRepository(database), app.CryptoIDGenerator{}, fixedTestClock{value: stamp}, "integration")
	first, err := service.Apply(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bob := domain.User{ID: "33333333-3333-4333-8333-333333333333", Username: "bob", UsernameNormalized: "bob", PasswordHash: "hash-bob", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	if err := users.Create(ctx, bob); err != nil {
		t.Fatal(err)
	}
	if err := users.SetEnabled(ctx, user.ID, false, stamp); err != nil {
		t.Fatal(err)
	}
	shareRepository := storage.NewShareRepository(database)
	share.Enabled = false
	if err := shareRepository.Update(ctx, share); err != nil {
		t.Fatal(err)
	}
	second, err := service.Apply(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first.Number != 1 || second.Number != 2 {
		t.Fatalf("revisions = %#v and %#v", first, second)
	}
	if stored, err := users.Get(ctx, user.ID); err != nil || stored.Enabled {
		t.Fatalf("disabled user = %#v, err = %v", stored, err)
	}
	if _, err := service.Restore(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	restored, err := users.Get(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !restored.Enabled {
		t.Fatalf("restored user = %#v, want enabled", restored)
	}
	if _, err := users.Get(ctx, bob.ID); !errors.Is(err, app.ErrUserNotFound) {
		t.Fatalf("restored extra user error = %v, want %v", err, app.ErrUserNotFound)
	}
	restoredShare, err := shareRepository.Get(ctx, share.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !restoredShare.Enabled {
		t.Fatalf("restored share = %#v, want enabled", restoredShare)
	}
	permissions, err := storage.NewPermissionRepository(database).ListByShare(ctx, share.ID)
	if err != nil || len(permissions) != 1 || permissions[0].Permission != domain.PermissionReadWrite {
		t.Fatalf("restored permissions = %#v, err = %v", permissions, err)
	}
	state, err := service.State(ctx)
	if err != nil || state.Pending || state.ActiveRevision == nil || *state.ActiveRevision != first.Number {
		t.Fatalf("restored state = %#v, err = %v", state, err)
	}
}

type noOpTestValidator struct{}

func (noOpTestValidator) Validate(context.Context, []byte) error { return nil }

type statefulTestRuntime struct {
	state caddyruntime.RuntimeState
}

func (r *statefulTestRuntime) Start(context.Context, []byte) error {
	r.state = caddyruntime.RuntimeRunning
	return nil
}

func (r *statefulTestRuntime) Reload(context.Context, []byte) error {
	r.state = caddyruntime.RuntimeRunning
	return nil
}

func (r *statefulTestRuntime) Stop(context.Context) error {
	r.state = caddyruntime.RuntimeStopped
	return nil
}

func (r *statefulTestRuntime) Status(context.Context) caddyruntime.RuntimeState { return r.state }

type fixedTestClock struct {
	value domain.Timestamp
}

func (c fixedTestClock) Now() time.Time { return time.Time(c.value) }

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}
