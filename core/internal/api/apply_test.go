package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiApply struct {
	revision  domain.ConfigRevision
	state     app.RevisionState
	err       error
	deleteErr error
	calls     int
}

func (a *apiApply) Apply(context.Context) (domain.ConfigRevision, error) {
	a.calls++
	return a.revision, a.err
}
func (a *apiApply) Validate(context.Context) (app.ValidationResult, error) {
	if a.err != nil {
		return app.ValidationResult{}, a.err
	}
	return app.ValidationResult{Valid: true, ConfigHash: a.revision.ConfigHash, Warnings: []string{"test warning"}}, nil
}
func (a *apiApply) Restore(_ context.Context, id domain.ID) (domain.ConfigRevision, error) {
	if id != a.revision.ID {
		return domain.ConfigRevision{}, &app.Error{Code: app.CodeRevisionNotFound, Message: "Revision was not found", Cause: app.ErrRevisionNotFound}
	}
	return a.revision, a.err
}
func (a *apiApply) State(context.Context) (app.RevisionState, error) { return a.state, a.err }
func (a *apiApply) List(context.Context) ([]domain.ConfigRevision, error) {
	return []domain.ConfigRevision{a.revision}, a.err
}
func (a *apiApply) Get(_ context.Context, id domain.ID) (domain.ConfigRevision, error) {
	if id != a.revision.ID {
		return domain.ConfigRevision{}, &app.Error{Code: app.CodeRevisionNotFound, Message: "Revision was not found", Cause: app.ErrRevisionNotFound}
	}
	return a.revision, a.err
}

func (a *apiApply) Delete(context.Context, domain.ID) error { return a.deleteErr }

func TestApplyAndRevisionAPIHidesRawConfiguration(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	body := []byte(`{"secret_hash":"must-not-leak"}`)
	revision := domain.ConfigRevision{ID: "11111111-1111-4111-8111-111111111111", Number: 1, CreatedAt: stamp, ConfigJSON: body, ConfigHash: domain.HashConfigJSON(body), ValidationStatus: domain.RevisionValidationValid, ApplyStatus: domain.RevisionApplyApplied, AppVersion: "test"}
	number := uint64(1)
	service := &apiApply{revision: revision, state: app.RevisionState{DesiredRevision: &number, ActiveRevision: &number}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithApplyService(service))
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct{ method, path string }{{http.MethodPost, "/api/v1/config/validate"}, {http.MethodPost, "/api/v1/config/apply"}, {http.MethodGet, "/api/v1/revisions"}, {http.MethodGet, "/api/v1/revisions/11111111-1111-4111-8111-111111111111"}, {http.MethodPost, "/api/v1/revisions/11111111-1111-4111-8111-111111111111/restore"}, {http.MethodGet, "/api/v1/config/state"}} {
		response := apiRequest(t, server, request.method, request.path, "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s %s = %d: %s", request.method, request.path, response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "must-not-leak") || strings.Contains(response.Body.String(), "config_json") {
			t.Fatalf("raw configuration leaked: %s", response.Body.String())
		}
	}
}

func TestConfigValidateAndRevisionRestoreRejectWrongMethods(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	body := []byte(`{}`)
	service := &apiApply{revision: domain.ConfigRevision{ID: "11111111-1111-4111-8111-111111111111", Number: 1, CreatedAt: stamp, ConfigJSON: body, ConfigHash: domain.HashConfigJSON(body), ValidationStatus: domain.RevisionValidationValid, ApplyStatus: domain.RevisionApplyApplied, AppVersion: "test"}}
	server, _ := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithApplyService(service))
	response := apiRequest(t, server, http.MethodGet, "/api/v1/config/validate", "")
	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("validate wrong method = %d", response.Code)
	}
	response = apiRequest(t, server, http.MethodPost, "/api/v1/revisions/11111111-1111-4111-8111-111111111111/other", "")
	if response.Code != http.StatusNotFound {
		t.Fatalf("restore wrong suffix = %d", response.Code)
	}
}

func TestRevisionDeleteAPIUsesDeleteMethodAndMapsProtectedRevision(t *testing.T) {
	service := &apiApply{deleteErr: &app.Error{Code: app.CodeRevisionActive, Message: "active revision"}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithApplyService(service))
	if err != nil {
		t.Fatal(err)
	}
	response := apiRequest(t, server, http.MethodDelete, "/api/v1/revisions/11111111-1111-4111-8111-111111111111", "")
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "REVISION_ACTIVE") {
		t.Fatalf("delete protected = %d: %s", response.Code, response.Body.String())
	}
	service.deleteErr = nil
	response = apiRequest(t, server, http.MethodDelete, "/api/v1/revisions/11111111-1111-4111-8111-111111111111", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"deleted":true`) {
		t.Fatalf("delete = %d: %s", response.Code, response.Body.String())
	}
}

func TestApplyAPIMapsConflictAndValidationErrors(t *testing.T) {
	service := &apiApply{err: &app.Error{Code: app.CodeApplyInProgress, Message: "Another configuration apply is in progress"}}
	server, _ := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithApplyService(service))
	response := apiRequest(t, server, http.MethodPost, "/api/v1/config/apply", "")
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "CONFIG_APPLY_IN_PROGRESS") {
		t.Fatalf("conflict = %d: %s", response.Code, response.Body.String())
	}
	service.err = &app.Error{Code: app.CodeCaddyValidateFailed, Message: "invalid"}
	response = apiRequest(t, server, http.MethodPost, "/api/v1/config/apply", "")
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("validation = %d: %s", response.Code, response.Body.String())
	}
}
