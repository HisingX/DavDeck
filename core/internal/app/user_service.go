package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
)

// UserRepository is the persistence boundary required by UserService.
type UserRepository interface {
	List(context.Context) ([]domain.User, error)
	Get(context.Context, domain.ID) (domain.User, error)
	Create(context.Context, domain.User) error
	Delete(context.Context, domain.ID) error
	SetEnabled(context.Context, domain.ID, bool, domain.Timestamp) error
	SetPasswordHash(context.Context, domain.ID, string, domain.Timestamp) error
}

type PasswordHasher interface {
	Hash(string) (string, error)
	Compare(string, string) error
}

type IDGenerator interface {
	NewID() (domain.ID, error)
}

type Clock interface {
	Now() time.Time
}

// UserService owns user validation and password hashing use cases.
type UserService struct {
	repository UserRepository
	hasher     PasswordHasher
	ids        IDGenerator
	clock      Clock
}

func NewUserService(repository UserRepository, hasher PasswordHasher, ids IDGenerator, clock Clock) *UserService {
	return &UserService{repository: repository, hasher: hasher, ids: ids, clock: clock}
}

func (s *UserService) List(ctx context.Context) ([]domain.User, error) {
	users, err := s.repository.List(ctx)
	if err != nil {
		return nil, databaseError(err)
	}
	return users, nil
}

func (s *UserService) Get(ctx context.Context, id domain.ID) (domain.User, error) {
	user, err := s.repository.Get(ctx, id)
	if err != nil {
		return domain.User{}, mapUserRepositoryError(err)
	}
	return user, nil
}

func (s *UserService) Create(ctx context.Context, username, password string) (domain.User, error) {
	if err := validatePassword(password); err != nil {
		return domain.User{}, err
	}
	id, err := s.ids.NewID()
	if err != nil {
		return domain.User{}, databaseError(fmt.Errorf("generate user id: %w", err))
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return domain.User{}, databaseError(err)
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return domain.User{}, &Error{Code: CodeInvalidPassword, Message: "Password could not be hashed", Cause: err}
	}
	user := domain.User{ID: id, Username: username, UsernameNormalized: domain.NormalizeUsername(username), PasswordHash: hash, Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	if err := user.Validate(); err != nil {
		return domain.User{}, &Error{Code: CodeInvalidUsername, Message: "Username is invalid", Cause: err}
	}
	if err := s.repository.Create(ctx, user); err != nil {
		return domain.User{}, mapUserRepositoryError(err)
	}
	return user, nil
}

func (s *UserService) Delete(ctx context.Context, id domain.ID) error {
	return mapUserRepositoryError(s.repository.Delete(ctx, id))
}

func (s *UserService) SetEnabled(ctx context.Context, id domain.ID, enabled bool) error {
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return databaseError(err)
	}
	return mapUserRepositoryError(s.repository.SetEnabled(ctx, id, enabled, stamp))
}

func (s *UserService) ChangePassword(ctx context.Context, id domain.ID, password string) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return &Error{Code: CodeInvalidPassword, Message: "Password could not be hashed", Cause: err}
	}
	stamp, err := domain.NewTimestamp(s.clock.Now())
	if err != nil {
		return databaseError(err)
	}
	return mapUserRepositoryError(s.repository.SetPasswordHash(ctx, id, hash, stamp))
}

func validatePassword(password string) error {
	length := len([]byte(password))
	if length < 8 || length > 72 {
		return &Error{Code: CodeInvalidPassword, Message: "Password must contain between 8 and 72 UTF-8 bytes"}
	}
	return nil
}

func mapUserRepositoryError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrUserNotFound):
		return &Error{Code: CodeUserNotFound, Message: "User was not found", Cause: err}
	case errors.Is(err, ErrUserAlreadyExists):
		return &Error{Code: CodeUserAlreadyExists, Message: "Username already exists", Cause: err}
	default:
		return databaseError(err)
	}
}

func databaseError(err error) error {
	return &Error{Code: CodeDatabase, Message: "Database operation failed", Cause: err}
}

// CryptoIDGenerator creates RFC 4122 version 4 UUID identifiers.
type CryptoIDGenerator struct{}

func (CryptoIDGenerator) NewID() (domain.ID, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return domain.ParseID(fmt.Sprintf("%x-%x-%x-%x-%x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]))
}

type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now().UTC() }
