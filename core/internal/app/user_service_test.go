package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

const testUserID = domain.ID("11111111-1111-4111-8111-111111111111")

type memoryUsers struct{ users map[domain.ID]domain.User }

func newMemoryUsers() *memoryUsers { return &memoryUsers{users: make(map[domain.ID]domain.User)} }
func (r *memoryUsers) List(context.Context) ([]domain.User, error) {
	result := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		result = append(result, user)
	}
	return result, nil
}
func (r *memoryUsers) Get(_ context.Context, id domain.ID) (domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return domain.User{}, ErrUserNotFound
	}
	return user, nil
}
func (r *memoryUsers) Create(_ context.Context, user domain.User) error {
	for _, existing := range r.users {
		if existing.UsernameNormalized == user.UsernameNormalized {
			return ErrUserAlreadyExists
		}
	}
	r.users[user.ID] = user
	return nil
}
func (r *memoryUsers) Delete(_ context.Context, id domain.ID) error {
	if _, ok := r.users[id]; !ok {
		return ErrUserNotFound
	}
	delete(r.users, id)
	return nil
}
func (r *memoryUsers) SetEnabled(_ context.Context, id domain.ID, enabled bool, updated domain.Timestamp) error {
	user, ok := r.users[id]
	if !ok {
		return ErrUserNotFound
	}
	user.Enabled, user.UpdatedAt = enabled, updated
	r.users[id] = user
	return nil
}
func (r *memoryUsers) SetPasswordHash(_ context.Context, id domain.ID, hash string, updated domain.Timestamp) error {
	user, ok := r.users[id]
	if !ok {
		return ErrUserNotFound
	}
	user.PasswordHash, user.UpdatedAt = hash, updated
	r.users[id] = user
	return nil
}

type testHasher struct{ plaintext string }

func (h *testHasher) Hash(password string) (string, error) {
	h.plaintext = password
	return "hash:" + password, nil
}
func (*testHasher) Compare(string, string) error { return nil }

type fixedID struct{}

func (fixedID) NewID() (domain.ID, error) { return testUserID, nil }

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC) }

func TestUserServiceLifecycleHashesPasswords(t *testing.T) {
	repository, hasher := newMemoryUsers(), &testHasher{}
	service := NewUserService(repository, hasher, fixedID{}, fixedClock{})
	user, err := service.Create(context.Background(), "Alice", "correct horse")
	if err != nil {
		t.Fatal(err)
	}
	if hasher.plaintext != "correct horse" || user.PasswordHash != "hash:correct horse" {
		t.Fatalf("password was not hashed: %#v", user)
	}
	if user.Enabled != true || user.UsernameNormalized != "alice" {
		t.Fatalf("user = %#v", user)
	}
	if err := service.SetEnabled(context.Background(), testUserID, false); err != nil {
		t.Fatal(err)
	}
	if err := service.ChangePassword(context.Background(), testUserID, "another password"); err != nil {
		t.Fatal(err)
	}
	stored, _ := repository.Get(context.Background(), testUserID)
	if stored.Enabled || stored.PasswordHash != "hash:another password" {
		t.Fatalf("stored = %#v", stored)
	}
	if err := service.Delete(context.Background(), testUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(context.Background(), testUserID); !hasCode(err, CodeUserNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestUserServiceValidationAndUniqueness(t *testing.T) {
	service := NewUserService(newMemoryUsers(), &testHasher{}, fixedID{}, fixedClock{})
	if _, err := service.Create(context.Background(), "Alice", "short"); !hasCode(err, CodeInvalidPassword) {
		t.Fatalf("error = %v", err)
	}
	if _, err := service.Create(context.Background(), " Alice ", "valid password"); !hasCode(err, CodeInvalidUsername) {
		t.Fatalf("error = %v", err)
	}
	repository := newMemoryUsers()
	service = NewUserService(repository, &testHasher{}, fixedID{}, fixedClock{})
	if _, err := service.Create(context.Background(), "Alice", "valid password"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), "ALICE", "valid password"); !hasCode(err, CodeUserAlreadyExists) {
		t.Fatalf("error = %v", err)
	}
}

func TestBcryptHasherAndGeneratedID(t *testing.T) {
	hasher := BcryptHasher{}
	hash, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("plaintext was returned")
	}
	if err := hasher.Compare(hash, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if err := hasher.Compare(hash, "wrong password"); err == nil {
		t.Fatal("wrong password matched")
	}
	id, err := (CryptoIDGenerator{}).NewID()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := domain.ParseID(string(id)); err != nil {
		t.Fatalf("generated invalid id %q: %v", id, err)
	}
}

func hasCode(err error, code ErrorCode) bool {
	var applicationError *Error
	return errors.As(err, &applicationError) && applicationError.Code == code
}
