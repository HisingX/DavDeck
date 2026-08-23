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

type apiShares struct{ share *domain.Share }

func (s *apiShares) List(context.Context) ([]domain.Share, error) {
	if s.share == nil {
		return nil, nil
	}
	return []domain.Share{*s.share}, nil
}
func (s *apiShares) Get(_ context.Context, id domain.ID) (domain.Share, error) {
	if s.share == nil || s.share.ID != id {
		return domain.Share{}, &app.Error{Code: app.CodeShareNotFound, Message: "Share was not found", Cause: app.ErrShareNotFound}
	}
	return *s.share, nil
}
func (s *apiShares) Create(_ context.Context, name, slug, path string) (domain.Share, error) {
	if s.share != nil {
		return domain.Share{}, &app.Error{Code: app.CodeShareAlreadyExists, Message: "Share slug already exists", Cause: app.ErrShareAlreadyExists}
	}
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	share := domain.Share{ID: "11111111-1111-4111-8111-111111111111", Name: name, Slug: slug, Path: path, Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	s.share = &share
	return share, nil
}
func (s *apiShares) Update(_ context.Context, id domain.ID, update app.ShareUpdate) (domain.Share, error) {
	share, err := s.Get(context.Background(), id)
	if err != nil {
		return domain.Share{}, err
	}
	if update.Name != nil {
		share.Name = *update.Name
	}
	if update.Slug != nil {
		share.Slug = *update.Slug
	}
	if update.Path != nil {
		share.Path = *update.Path
	}
	if update.Enabled != nil {
		share.Enabled = *update.Enabled
	}
	s.share = &share
	return share, nil
}
func (s *apiShares) Delete(_ context.Context, id domain.ID) error {
	if _, err := s.Get(context.Background(), id); err != nil {
		return err
	}
	s.share = nil
	return nil
}

func TestShareAPILifecycleAndPreservationSignal(t *testing.T) {
	service := &apiShares{}
	runtime := &apiApply{}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithShareService(service), WithApplyService(runtime))
	if err != nil {
		t.Fatal(err)
	}
	created := apiRequest(t, server, http.MethodPost, "/api/v1/shares", `{"name":"Documents","slug":"documents","path":"/srv/documents"}`)
	if created.Code != http.StatusCreated || !strings.Contains(created.Body.String(), `"slug":"documents"`) {
		t.Fatalf("create = %d: %s", created.Code, created.Body.String())
	}
	patched := apiRequest(t, server, http.MethodPatch, "/api/v1/shares/11111111-1111-4111-8111-111111111111", `{"enabled":false}`)
	if patched.Code != http.StatusOK || !strings.Contains(patched.Body.String(), `"enabled":false`) {
		t.Fatalf("patch = %d: %s", patched.Code, patched.Body.String())
	}
	deleted := apiRequest(t, server, http.MethodDelete, "/api/v1/shares/11111111-1111-4111-8111-111111111111", "")
	if deleted.Code != http.StatusOK || !strings.Contains(deleted.Body.String(), `"files_preserved":true`) {
		t.Fatalf("delete = %d: %s", deleted.Code, deleted.Body.String())
	}
	missing := apiRequest(t, server, http.MethodGet, "/api/v1/shares/11111111-1111-4111-8111-111111111111", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing = %d: %s", missing.Code, missing.Body.String())
	}
	if runtime.calls != 3 {
		t.Fatalf("automatic apply calls = %d, want 3", runtime.calls)
	}
}

func TestShareAPIRejectsInvalidShapesAndDuplicateSlug(t *testing.T) {
	service := &apiShares{}
	runtime := &apiApply{}
	server, _ := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithShareService(service), WithApplyService(runtime))
	invalid := apiRequest(t, server, http.MethodPost, "/api/v1/shares", `{"name":"Docs","slug":"docs","path":"/srv/docs","unknown":true}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid = %d: %s", invalid.Code, invalid.Body.String())
	}
	first := apiRequest(t, server, http.MethodPost, "/api/v1/shares", `{"name":"Docs","slug":"docs","path":"/srv/docs"}`)
	if first.Code != http.StatusCreated {
		t.Fatal(first.Body.String())
	}
	duplicate := apiRequest(t, server, http.MethodPost, "/api/v1/shares", `{"name":"Other","slug":"docs","path":"/srv/other"}`)
	if duplicate.Code != http.StatusConflict || !strings.Contains(duplicate.Body.String(), "SHARE_ALREADY_EXISTS") {
		t.Fatalf("duplicate = %d: %s", duplicate.Code, duplicate.Body.String())
	}
	if runtime.calls != 1 {
		t.Fatalf("automatic apply calls = %d, want 1", runtime.calls)
	}
}
