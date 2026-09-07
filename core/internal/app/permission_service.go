package app

import (
	"context"
	"errors"

	"davdeck.dev/davdeck/core/internal/domain"
)

var ErrPermissionNotFound = errors.New("permission not found")

type PermissionRepository interface {
	ListByShare(context.Context, domain.ID) ([]domain.SharePermission, error)
	ListByUser(context.Context, domain.ID) ([]domain.SharePermission, error)
	ListAll(context.Context) ([]domain.SharePermission, error)
	Set(context.Context, domain.SharePermission) error
	Delete(context.Context, domain.ID, domain.ID) error
}

type PermissionEntry struct {
	ShareID     domain.ID         `json:"share_id"`
	UserID      domain.ID         `json:"user_id"`
	Username    string            `json:"username"`
	UserEnabled bool              `json:"user_enabled"`
	Permission  domain.Permission `json:"permission"`
}

// UserPermissionEntry is the user-centric view of the same per-share ACL.
// Missing rows are returned explicitly as NONE so the GUI can configure every
// share without needing another endpoint.
type UserPermissionEntry struct {
	ShareID      domain.ID         `json:"share_id"`
	ShareName    string            `json:"share_name"`
	ShareSlug    string            `json:"share_slug"`
	ShareEnabled bool              `json:"share_enabled"`
	Permission   domain.Permission `json:"permission"`
}

// UserPermissionSummary is the compact authorized-only data used by the user
// list. It is produced in one batch rather than one query per user.
type UserPermissionSummary struct {
	ShareID      domain.ID         `json:"share_id"`
	ShareName    string            `json:"share_name"`
	ShareSlug    string            `json:"share_slug"`
	ShareEnabled bool              `json:"share_enabled"`
	Permission   domain.Permission `json:"permission"`
}

type PermissionService struct {
	repository PermissionRepository
	shares     ShareRepository
	users      UserRepository
	clock      Clock
}

func NewPermissionService(repository PermissionRepository, shares ShareRepository, users UserRepository, clock Clock) *PermissionService {
	return &PermissionService{repository: repository, shares: shares, users: users, clock: clock}
}

func (s *PermissionService) List(ctx context.Context, shareID domain.ID) ([]PermissionEntry, error) {
	if _, err := s.shares.Get(ctx, shareID); err != nil {
		return nil, mapShareError(err)
	}
	users, err := s.users.List(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	permissions, err := s.repository.ListByShare(ctx, shareID)
	if err != nil {
		return nil, databaseError(err)
	}
	byUser := make(map[domain.ID]domain.Permission, len(permissions))
	for _, permission := range permissions {
		byUser[permission.UserID] = permission.Permission
	}
	entries := make([]PermissionEntry, 0, len(users))
	for _, user := range users {
		permission := byUser[user.ID]
		if permission == "" {
			permission = domain.PermissionNone
		}
		entries = append(entries, PermissionEntry{ShareID: shareID, UserID: user.ID, Username: user.Username, UserEnabled: user.Enabled, Permission: permission})
	}
	return entries, nil
}

func (s *PermissionService) Set(ctx context.Context, shareID, userID domain.ID, permission domain.Permission) (PermissionEntry, error) {
	if !permission.Valid() {
		return PermissionEntry{}, &Error{Code: CodeInvalidPermission, Message: "Permission must be NONE, READ, or READ_WRITE"}
	}
	if _, err := s.shares.Get(ctx, shareID); err != nil {
		return PermissionEntry{}, mapShareError(err)
	}
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		return PermissionEntry{}, mapUserRepositoryError(err)
	}
	if permission == domain.PermissionNone {
		err := s.repository.Delete(ctx, shareID, userID)
		if err != nil && !errors.Is(err, ErrPermissionNotFound) {
			return PermissionEntry{}, databaseError(err)
		}
		return PermissionEntry{ShareID: shareID, UserID: userID, Username: user.Username, UserEnabled: user.Enabled, Permission: permission}, nil
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return PermissionEntry{}, databaseError(err)
	}
	value := domain.SharePermission{ShareID: shareID, UserID: userID, Permission: permission, CreatedAt: stamp, UpdatedAt: stamp}
	if err := value.Validate(); err != nil {
		return PermissionEntry{}, &Error{Code: CodeInvalidPermission, Message: "Permission is invalid", Cause: err}
	}
	if err := s.repository.Set(ctx, value); err != nil {
		return PermissionEntry{}, databaseError(err)
	}
	return PermissionEntry{ShareID: shareID, UserID: userID, Username: user.Username, UserEnabled: user.Enabled, Permission: permission}, nil
}

// ListByUser returns every share for a user, filling absent ACL rows with NONE.
func (s *PermissionService) ListByUser(ctx context.Context, userID domain.ID) ([]UserPermissionEntry, error) {
	if _, err := s.users.Get(ctx, userID); err != nil {
		return nil, mapUserRepositoryError(err)
	}
	shares, err := s.shares.List(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	permissions, err := s.repository.ListByUser(ctx, userID)
	if err != nil {
		return nil, databaseError(err)
	}
	byShare := make(map[domain.ID]domain.Permission, len(permissions))
	for _, permission := range permissions {
		byShare[permission.ShareID] = permission.Permission
	}
	entries := make([]UserPermissionEntry, 0, len(shares))
	for _, share := range shares {
		permission := byShare[share.ID]
		if permission == "" {
			permission = domain.PermissionNone
		}
		entries = append(entries, UserPermissionEntry{
			ShareID: share.ID, ShareName: share.Name, ShareSlug: share.Slug,
			ShareEnabled: share.Enabled, Permission: permission,
		})
	}
	return entries, nil
}

// AuthorizedUserCounts returns configured non-NONE ACL counts in one query.
// Disabled users are intentionally included: the number represents configured
// access, not currently active logins.
func (s *PermissionService) AuthorizedUserCounts(ctx context.Context) (map[domain.ID]int, error) {
	permissions, err := s.repository.ListAll(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	counts := make(map[domain.ID]int)
	for _, permission := range permissions {
		if permission.Permission != domain.PermissionNone {
			counts[permission.ShareID]++
		}
	}
	return counts, nil
}

// SummariesByUser returns authorized-only share chips for every user using the
// same batch permission read as AuthorizedUserCounts.
func (s *PermissionService) SummariesByUser(ctx context.Context) (map[domain.ID][]UserPermissionSummary, error) {
	shares, err := s.shares.List(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	permissions, err := s.repository.ListAll(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	shareByID := make(map[domain.ID]domain.Share, len(shares))
	for _, share := range shares {
		shareByID[share.ID] = share
	}
	result := make(map[domain.ID][]UserPermissionSummary)
	for _, permission := range permissions {
		if permission.Permission == domain.PermissionNone {
			continue
		}
		share, ok := shareByID[permission.ShareID]
		if !ok {
			continue
		}
		result[permission.UserID] = append(result[permission.UserID], UserPermissionSummary{
			ShareID: share.ID, ShareName: share.Name, ShareSlug: share.Slug,
			ShareEnabled: share.Enabled, Permission: permission.Permission,
		})
	}
	return result, nil
}
