package app

import (
	"context"
	"testing"

	"davdeck.dev/davdeck/core/internal/domain"
)

type memoryPermissions struct {
	values map[domain.ID]domain.SharePermission
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
func (r *memoryPermissions) Set(_ context.Context, value domain.SharePermission) error {
	r.values[value.UserID] = value
	return nil
}
func (r *memoryPermissions) Delete(_ context.Context, _ domain.ID, userID domain.ID) error {
	if _, ok := r.values[userID]; !ok {
		return ErrPermissionNotFound
	}
	delete(r.values, userID)
	return nil
}

func TestPermissionServiceUsesExplicitNoneForMissingRows(t *testing.T) {
	ctx := context.Background()
	users := newMemoryUsers()
	shares := newMemoryShares()
	permissions := &memoryPermissions{values: make(map[domain.ID]domain.SharePermission)}
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
	service := NewPermissionService(&memoryPermissions{values: make(map[domain.ID]domain.SharePermission)}, newMemoryShares(), newMemoryUsers(), fixedClock{})
	if _, err := service.List(context.Background(), testUserID); !hasCode(err, CodeShareNotFound) {
		t.Fatalf("share error = %v", err)
	}
}
