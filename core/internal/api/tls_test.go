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

type apiTLSService struct{ profile *domain.TLSProfile }

func (s *apiTLSService) Get(context.Context) (domain.TLSProfile, bool, error) {
	if s.profile == nil {
		return domain.TLSProfile{}, false, nil
	}
	return *s.profile, true, nil
}
func (s *apiTLSService) Update(_ context.Context, update app.TLSUpdate) (domain.TLSProfile, error) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	profile := domain.TLSProfile{ID: "11111111-1111-4111-8111-111111111111", Mode: update.Mode, Hostname: update.Hostname, CertificatePath: update.CertificatePath, PrivateKeyPath: update.PrivateKeyPath, CreatedAt: stamp, UpdatedAt: stamp}
	s.profile = &profile
	return profile, nil
}
func (s *apiTLSService) Check(context.Context) (app.TLSCheckResult, error) {
	return app.TLSCheckResult{Ready: true, Checks: []app.TLSCheck{{Name: "configuration", OK: true, Message: "valid"}}}, nil
}

func TestTLSAPIGetPutAndCheck(t *testing.T) {
	service := &apiTLSService{}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithTLSService(service))
	if err != nil {
		t.Fatal(err)
	}
	empty := apiRequest(t, server, http.MethodGet, "/api/v1/tls", "")
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"data":null`) {
		t.Fatalf("empty = %d: %s", empty.Code, empty.Body.String())
	}
	update := apiRequest(t, server, http.MethodPut, "/api/v1/tls", `{"mode":"custom","hostname":"dav.local","certificate_path":"/cert.pem","private_key_path":"/key.pem"}`)
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), `"private_key_path":"/key.pem"`) {
		t.Fatalf("update = %d: %s", update.Code, update.Body.String())
	}
	check := apiRequest(t, server, http.MethodPost, "/api/v1/tls/check", "")
	if check.Code != http.StatusOK || !strings.Contains(check.Body.String(), `"ready":true`) {
		t.Fatalf("check = %d: %s", check.Code, check.Body.String())
	}
	invalid := apiRequest(t, server, http.MethodPut, "/api/v1/tls", `{"mode":"internal","hostname":"dav.local","private_key":"secret material"}`)
	if invalid.Code != http.StatusBadRequest || strings.Contains(invalid.Body.String(), "secret material") {
		t.Fatalf("invalid = %d: %s", invalid.Code, invalid.Body.String())
	}
}
