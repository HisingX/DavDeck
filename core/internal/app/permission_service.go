package app

import (
	"context"
	"errors"

	"davdeck.dev/davdeck/core/internal/domain"
)

var ErrPermissionNotFound = errors.New("permission not found")

type PermissionRepository interface {
	ListByShare(context.Context, domain.ID) ([]domain.SharePermission, error)
	Set(context.Context, domain.SharePermission) error
	Delete(context.Context, domain.ID, domain.ID) error
}

type PermissionEntry struct {
	ShareID    domain.ID         `json:"share_id"`
	UserID     domain.ID         `json:"user_id"`
	Username   string            `json:"username"`
	Permission domain.Permission `json:"permission"`
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
		entries = append(entries, PermissionEntry{ShareID: shareID, UserID: user.ID, Username: user.Username, Permission: permission})
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
		return PermissionEntry{ShareID: shareID, UserID: userID, Username: user.Username, Permission: permission}, nil
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
	return PermissionEntry{ShareID: shareID, UserID: userID, Username: user.Username, Permission: permission}, nil
}
