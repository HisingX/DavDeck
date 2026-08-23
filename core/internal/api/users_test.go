package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiUserRepository struct{ user *domain.User }

func (r *apiUserRepository) List(context.Context) ([]domain.User, error) {
	if r.user == nil {
		return nil, nil
	}
	return []domain.User{*r.user}, nil
}
func (r *apiUserRepository) Get(_ context.Context, id domain.ID) (domain.User, error) {
	if r.user == nil || r.user.ID != id {
		return domain.User{}, app.ErrUserNotFound
	}
	return *r.user, nil
}
func (r *apiUserRepository) Create(_ context.Context, user domain.User) error {
	if r.user != nil {
		return app.ErrUserAlreadyExists
	}
	r.user = &user
	return nil
}
func (r *apiUserRepository) Delete(_ context.Context, id domain.ID) error {
	if r.user == nil || r.user.ID != id {
		return app.ErrUserNotFound
	}
	r.user = nil
	return nil
}
func (r *apiUserRepository) SetEnabled(_ context.Context, id domain.ID, enabled bool, updated domain.Timestamp) error {
	if r.user == nil || r.user.ID != id {
		return app.ErrUserNotFound
	}
	r.user.Enabled, r.user.UpdatedAt = enabled, updated
	return nil
}
func (r *apiUserRepository) SetPasswordHash(_ context.Context, id domain.ID, hash string, updated domain.Timestamp) error {
	if r.user == nil || r.user.ID != id {
		return app.ErrUserNotFound
	}
	r.user.PasswordHash, r.user.UpdatedAt = hash, updated
	return nil
}

type apiHasher struct{}

func (apiHasher) Hash(password string) (string, error) { return "SECRET_HASH:" + password, nil }
func (apiHasher) Compare(string, string) error         { return nil }

type apiID struct{}

func (apiID) NewID() (domain.ID, error) { return "11111111-1111-4111-8111-111111111111", nil }

type apiClock struct{}

func (apiClock) Now() time.Time { return time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC) }

func TestUserAPICompleteLifecycleAndHidesHash(t *testing.T) {
	repository := &apiUserRepository{}
	service := app.NewUserService(repository, apiHasher{}, apiID{}, apiClock{})
	runtime := &apiApply{}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{Name: "DavDeck"}, nil, WithUserService(service), WithApplyService(runtime))
	if err != nil {
		t.Fatal(err)
	}
	create := apiRequest(t, server, http.MethodPost, "/api/v1/users", `{"username":"Alice","password":"valid password"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", create.Code, create.Body.String())
	}
	if strings.Contains(create.Body.String(), "password") || strings.Contains(create.Body.String(), "SECRET_HASH") {
		t.Fatalf("secret leaked: %s", create.Body.String())
	}
	list := apiRequest(t, server, http.MethodGet, "/api/v1/users", "")
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), "SECRET_HASH") {
		t.Fatalf("list = %d: %s", list.Code, list.Body.String())
	}
	patch := apiRequest(t, server, http.MethodPatch, "/api/v1/users/11111111-1111-4111-8111-111111111111", `{"enabled":false}`)
	if patch.Code != http.StatusOK || !strings.Contains(patch.Body.String(), `"enabled":false`) {
		t.Fatalf("patch = %d: %s", patch.Code, patch.Body.String())
	}
	password := apiRequest(t, server, http.MethodPost, "/api/v1/users/11111111-1111-4111-8111-111111111111/password", `{"password":"another valid password"}`)
	if password.Code != http.StatusOK || strings.Contains(password.Body.String(), "another valid password") {
		t.Fatalf("password = %d: %s", password.Code, password.Body.String())
	}
	deleted := apiRequest(t, server, http.MethodDelete, "/api/v1/users/11111111-1111-4111-8111-111111111111", "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete = %d: %s", deleted.Code, deleted.Body.String())
	}
	missing := apiRequest(t, server, http.MethodGet, "/api/v1/users/11111111-1111-4111-8111-111111111111", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing = %d: %s", missing.Code, missing.Body.String())
	}
	if runtime.calls != 4 {
		t.Fatalf("automatic apply calls = %d, want 4", runtime.calls)
	}
}

func TestUserAPIRejectsMalformedAndDuplicateRequests(t *testing.T) {
	repository := &apiUserRepository{}
	service := app.NewUserService(repository, apiHasher{}, apiID{}, apiClock{})
	runtime := &apiApply{}
	server, _ := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithUserService(service), WithApplyService(runtime))
	invalid := apiRequest(t, server, http.MethodPost, "/api/v1/users", `{"username":"Alice","password":"short","extra":true}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid = %d: %s", invalid.Code, invalid.Body.String())
	}
	first := apiRequest(t, server, http.MethodPost, "/api/v1/users", `{"username":"Alice","password":"valid password"}`)
	if first.Code != http.StatusCreated {
		t.Fatal(first.Body.String())
	}
	duplicate := apiRequest(t, server, http.MethodPost, "/api/v1/users", `{"username":"Alice","password":"valid password"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate = %d: %s", duplicate.Code, duplicate.Body.String())
	}
	var envelope Envelope
	if err := json.NewDecoder(duplicate.Body).Decode(&envelope); err != nil || envelope.Error == nil || envelope.Error.Code != "USER_ALREADY_EXISTS" {
		t.Fatalf("envelope = %#v, err = %v", envelope, err)
	}
	if runtime.calls != 1 {
		t.Fatalf("automatic apply calls = %d, want 1", runtime.calls)
	}
}

func TestUserAPIPreservesDesiredStateWhenAutomaticApplyFails(t *testing.T) {
	repository := &apiUserRepository{}
	service := app.NewUserService(repository, apiHasher{}, apiID{}, apiClock{})
	runtime := &apiApply{err: &app.Error{Code: app.CodeCaddyValidateFailed, Message: "Caddy rejected the generated configuration"}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithUserService(service), WithApplyService(runtime))
	if err != nil {
		t.Fatal(err)
	}

	response := apiRequest(t, server, http.MethodPost, "/api/v1/users", `{"username":"Alice","password":"valid password"}`)
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), "CADDY_VALIDATE_FAILED") {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
	if repository.user == nil || repository.user.Username != "Alice" {
		t.Fatalf("desired user was not retained: %#v", repository.user)
	}
	if runtime.calls != 1 {
		t.Fatalf("automatic apply calls = %d, want 1", runtime.calls)
	}
}

func apiRequest(t *testing.T, server *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer secret")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	return response
}
