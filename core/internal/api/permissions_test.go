package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiPermissions struct {
	permission domain.Permission
	counts     map[domain.ID]int
	summaries  map[domain.ID][]app.UserPermissionSummary
}

func (p *apiPermissions) List(_ context.Context, shareID domain.ID) ([]app.PermissionEntry, error) {
	permission := p.permission
	if permission == "" {
		permission = domain.PermissionNone
	}
	return []app.PermissionEntry{{ShareID: shareID, UserID: "11111111-1111-4111-8111-111111111111", Username: "Alice", UserEnabled: true, Permission: permission}}, nil
}
func (p *apiPermissions) Set(_ context.Context, shareID, userID domain.ID, permission domain.Permission) (app.PermissionEntry, bool, error) {
	if !permission.Valid() {
		return app.PermissionEntry{}, false, &app.Error{Code: app.CodeInvalidPermission, Message: "Permission must be NONE, READ, or READ_WRITE"}
	}
	changed := p.permission != permission
	p.permission = permission
	return app.PermissionEntry{ShareID: shareID, UserID: userID, Username: "Alice", UserEnabled: true, Permission: permission}, changed, nil
}

func (p *apiPermissions) ListByUser(context.Context, domain.ID) ([]app.UserPermissionEntry, error) {
	return []app.UserPermissionEntry{{
		ShareID: "22222222-2222-4222-8222-222222222222", ShareName: "Team",
		ShareSlug: "team", ShareEnabled: false, Permission: domain.PermissionRead,
	}}, nil
}

func (p *apiPermissions) AuthorizedUserCounts(context.Context) (map[domain.ID]int, error) {
	return p.counts, nil
}

func (p *apiPermissions) SummariesByUser(context.Context) (map[domain.ID][]app.UserPermissionSummary, error) {
	if p.summaries != nil {
		return p.summaries, nil
	}
	return map[domain.ID][]app.UserPermissionSummary{}, nil
}

func TestPermissionAPIListsExplicitNoneAndSetsEnum(t *testing.T) {
	permissions := &apiPermissions{}
	runtime := &apiApply{}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithShareService(&apiShares{}), WithPermissionService(permissions), WithApplyService(runtime))
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/shares/22222222-2222-4222-8222-222222222222/permissions"
	listed := apiRequest(t, server, http.MethodGet, path, "")
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), `"permission":"NONE"`) {
		t.Fatalf("list = %d: %s", listed.Code, listed.Body.String())
	}
	set := apiRequest(t, server, http.MethodPut, path+"/11111111-1111-4111-8111-111111111111", `{"permission":"READ_WRITE"}`)
	if set.Code != http.StatusOK || permissions.permission != domain.PermissionReadWrite {
		t.Fatalf("set = %d: %s", set.Code, set.Body.String())
	}
	repeated := apiRequest(t, server, http.MethodPut, path+"/11111111-1111-4111-8111-111111111111", `{"permission":"READ_WRITE"}`)
	if repeated.Code != http.StatusOK || runtime.calls != 1 {
		t.Fatalf("repeated set = %d: %s, automatic apply calls = %d", repeated.Code, repeated.Body.String(), runtime.calls)
	}
	invalid := apiRequest(t, server, http.MethodPut, path+"/11111111-1111-4111-8111-111111111111", `{"permission":"OWNER"}`)
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "INVALID_PERMISSION") {
		t.Fatalf("invalid = %d: %s", invalid.Code, invalid.Body.String())
	}
	if runtime.calls != 1 {
		t.Fatalf("automatic apply calls = %d, want 1", runtime.calls)
	}
}

func TestUserPermissionAPIListsTheUserPerspective(t *testing.T) {
	stamp, _ := domain.NewTimestamp(apiClock{}.Now())
	repository := &apiUserRepository{user: &domain.User{
		ID:                 "11111111-1111-4111-8111-111111111111",
		Username:           "Alice",
		UsernameNormalized: "alice",
		PasswordHash:       "hash",
		Enabled:            false,
		CreatedAt:          stamp,
		UpdatedAt:          stamp,
	}}
	users := app.NewUserService(repository, apiHasher{}, apiID{}, apiClock{})
	permissions := &apiPermissions{}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithUserService(users), WithPermissionService(permissions))
	if err != nil {
		t.Fatal(err)
	}
	response := apiRequest(t, server, http.MethodGet, "/api/v1/users/11111111-1111-4111-8111-111111111111/permissions", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"share_name":"Team"`) {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
}

func TestUserAPIIncludesShareStateInPermissionSummary(t *testing.T) {
	stamp, _ := domain.NewTimestamp(apiClock{}.Now())
	userID := domain.ID("11111111-1111-4111-8111-111111111111")
	repository := &apiUserRepository{user: &domain.User{
		ID: userID, Username: "Alice", UsernameNormalized: "alice",
		PasswordHash: "hash", Enabled: true, CreatedAt: stamp, UpdatedAt: stamp,
	}}
	users := app.NewUserService(repository, apiHasher{}, apiID{}, apiClock{})
	permissions := &apiPermissions{summaries: map[domain.ID][]app.UserPermissionSummary{
		userID: {{
			ShareID: "22222222-2222-4222-8222-222222222222", ShareName: "Archive",
			ShareSlug: "archive", ShareEnabled: false, Permission: domain.PermissionRead,
		}},
	}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil,
		WithUserService(users), WithPermissionService(permissions))
	if err != nil {
		t.Fatal(err)
	}
	response := apiRequest(t, server, http.MethodGet, "/api/v1/users", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"share_enabled":false`) {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
}

func TestShareAPIIncludesAuthorizedUserCount(t *testing.T) {
	stamp, _ := domain.NewTimestamp(apiClock{}.Now())
	shareID := domain.ID("22222222-2222-4222-8222-222222222222")
	shares := &apiShares{share: &domain.Share{
		ID: shareID, Name: "Team", Slug: "team", Path: "/srv/team", Enabled: true,
		CreatedAt: stamp, UpdatedAt: stamp,
	}}
	permissions := &apiPermissions{counts: map[domain.ID]int{shareID: 3}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithShareService(shares), WithPermissionService(permissions))
	if err != nil {
		t.Fatal(err)
	}
	response := apiRequest(t, server, http.MethodGet, "/api/v1/shares", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"authorized_user_count":3`) {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
}
