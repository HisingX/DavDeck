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

type apiPermissions struct{ permission domain.Permission }

func (p *apiPermissions) List(_ context.Context, shareID domain.ID) ([]app.PermissionEntry, error) {
	permission := p.permission
	if permission == "" {
		permission = domain.PermissionNone
	}
	return []app.PermissionEntry{{ShareID: shareID, UserID: "11111111-1111-4111-8111-111111111111", Username: "Alice", Permission: permission}}, nil
}
func (p *apiPermissions) Set(_ context.Context, shareID, userID domain.ID, permission domain.Permission) (app.PermissionEntry, error) {
	if !permission.Valid() {
		return app.PermissionEntry{}, &app.Error{Code: app.CodeInvalidPermission, Message: "Permission must be NONE, READ, or READ_WRITE"}
	}
	p.permission = permission
	return app.PermissionEntry{ShareID: shareID, UserID: userID, Username: "Alice", Permission: permission}, nil
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
	invalid := apiRequest(t, server, http.MethodPut, path+"/11111111-1111-4111-8111-111111111111", `{"permission":"OWNER"}`)
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "INVALID_PERMISSION") {
		t.Fatalf("invalid = %d: %s", invalid.Code, invalid.Body.String())
	}
	if runtime.calls != 1 {
		t.Fatalf("automatic apply calls = %d, want 1", runtime.calls)
	}
}
