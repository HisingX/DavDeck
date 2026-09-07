package app

import (
	"context"
	"testing"

	"davdeck.dev/davdeck/core/internal/domain"
)

type memoryPermissions struct {
	values map[string]domain.SharePermission
}

func permissionMapKey(shareID, userID domain.ID) string {
	return string(shareID) + ":" + string(userID)
}

func (r *memoryPermissions) ListByShare(_ context.Context, shareID domain.ID) ([]domain.SharePermission, error) {
	result := make([]domain.SharePermission, 0)
	for _, value := range r.values {
		if value.ShareID == shareID {
			result = append(result, value)
		}
	}
	return result, nil
}
func (r *memoryPermissions) ListByUser(_ context.Context, userID domain.ID) ([]domain.SharePermission, error) {
	result := make([]domain.SharePermission, 0)
	for _, value := range r.values {
		if value.UserID == userID {
			result = append(result, value)
		}
	}
	return result, nil
}
func (r *memoryPermissions) ListAll(context.Context) ([]domain.SharePermission, error) {
	result := make([]domain.SharePermission, 0, len(r.values))
	for _, value := range r.values {
		result = append(result, value)
	}
	return result, nil
}
func (r *memoryPermissions) Set(_ context.Context, value domain.SharePermission) error {
	r.values[permissionMapKey(value.ShareID, value.UserID)] = value
	return nil
}
func (r *memoryPermissions) Delete(_ context.Context, shareID, userID domain.ID) error {
	key := permissionMapKey(shareID, userID)
	if _, ok := r.values[key]; !ok {
		return ErrPermissionNotFound
	}
	delete(r.values, key)
	return nil
}

func TestPermissionServiceUsesExplicitNoneForMissingRows(t *testing.T) {
	ctx := context.Background()
	users := newMemoryUsers()
	shares := newMemoryShares()
	permissions := &memoryPermissions{values: make(map[string]domain.SharePermission)}
	userService := NewUserService(users, &testHasher{}, fixedID{}, fixedClock{})
	user, err := userService.Create(ctx, "Alice", "valid password")
	if err != nil {
		t.Fatal(err)
	}
	shareService := NewShareService(shares, &fakeSharePaths{}, fixedID{}, fixedClock{})
	share, err := shareService.Create(ctx, "Team", "team", "/srv/team")
	if err != nil {
		t.Fatal(err)
	}
	service := NewPermissionService(permissions, shares, users, fixedClock{})
	entries, err := service.List(ctx, share.ID)
	if err != nil || len(entries) != 1 || entries[0].Permission != domain.PermissionNone {
		t.Fatalf("entries = %#v, err = %v", entries, err)
	}
	entry, err := service.Set(ctx, share.ID, user.ID, domain.PermissionRead)
	if err != nil || entry.Permission != domain.PermissionRead {
		t.Fatalf("entry = %#v, err = %v", entry, err)
	}
	entries, _ = service.List(ctx, share.ID)
	if entries[0].Permission != domain.PermissionRead {
		t.Fatalf("entries = %#v", entries)
	}
	entry, err = service.Set(ctx, share.ID, user.ID, domain.PermissionNone)
	if err != nil || entry.Permission != domain.PermissionNone || len(permissions.values) != 0 {
		t.Fatalf("entry = %#v, values = %#v, err = %v", entry, permissions.values, err)
	}
	if _, err := service.Set(ctx, share.ID, user.ID, domain.Permission("OWNER")); !hasCode(err, CodeInvalidPermission) {
		t.Fatalf("invalid error = %v", err)
	}
}

func TestPermissionServiceChecksReferencedEntities(t *testing.T) {
	users := newMemoryUsers()
	shares := newMemoryShares()
	service := NewPermissionService(&memoryPermissions{values: make(map[string]domain.SharePermission)}, shares, users, fixedClock{})
	if _, err := service.List(context.Background(), testUserID); !hasCode(err, CodeShareNotFound) {
		t.Fatalf("share error = %v", err)
	}
	if _, err := service.ListByUser(context.Background(), testUserID); !hasCode(err, CodeUserNotFound) {
		t.Fatalf("user error = %v", err)
	}
}

func TestPermissionServiceReturnsUserAndShareViewsWithBatchCounts(t *testing.T) {
	ctx := context.Background()
	users := newMemoryUsers()
	shares := newMemoryShares()
	permissions := &memoryPermissions{values: make(map[string]domain.SharePermission)}
	userService := NewUserService(users, &testHasher{}, fixedID{}, fixedClock{})
	user, err := userService.Create(ctx, "Alice", "valid password")
	if err != nil {
		t.Fatal(err)
	}
	stamp, _ := domain.NewTimestamp(fixedClock{}.Now())
	first := domain.Share{ID: "22222222-2222-4222-8222-222222222222", Name: "Team", Slug: "team", Path: "/srv/team", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	second := domain.Share{ID: "33333333-3333-4333-8333-333333333333", Name: "Archive", Slug: "archive", Path: "/srv/archive", Enabled: false, CreatedAt: stamp, UpdatedAt: stamp}
	shares.shares[first.ID] = first
	shares.shares[second.ID] = second
	service := NewPermissionService(permissions, shares, users, fixedClock{})
	if _, err := service.Set(ctx, first.ID, user.ID, domain.PermissionReadWrite); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Set(ctx, second.ID, user.ID, domain.PermissionNone); err != nil {
		t.Fatal(err)
	}
	userEntries, err := service.ListByUser(ctx, user.ID)
	if err != nil || len(userEntries) != 2 {
		t.Fatalf("user entries = %#v, err = %v", userEntries, err)
	}
	byShare := make(map[domain.ID]UserPermissionEntry, len(userEntries))
	for _, entry := range userEntries {
		byShare[entry.ShareID] = entry
	}
	if byShare[first.ID].Permission != domain.PermissionReadWrite || byShare[second.ID].Permission != domain.PermissionNone || byShare[second.ID].ShareEnabled {
		t.Fatalf("user entries = %#v", userEntries)
	}
	counts, err := service.AuthorizedUserCounts(ctx)
	if err != nil || counts[first.ID] != 1 || counts[second.ID] != 0 {
		t.Fatalf("counts = %#v, err = %v", counts, err)
	}
	entries, err := service.List(ctx, first.ID)
	if err != nil || len(entries) != 1 || !entries[0].UserEnabled {
		t.Fatalf("share entries = %#v, err = %v", entries, err)
	}

	summaries, err := service.SummariesByUser(ctx)
	if err != nil || len(summaries[user.ID]) != 1 || !summaries[user.ID][0].ShareEnabled {
		t.Fatalf("user summaries = %#v, err = %v", summaries, err)
	}
	if err := userService.SetEnabled(ctx, user.ID, false); err != nil {
		t.Fatal(err)
	}
	entries, err = service.List(ctx, first.ID)
	if err != nil || len(entries) != 1 || entries[0].UserEnabled {
		t.Fatalf("disabled user entry = %#v, err = %v", entries, err)
	}
}
