package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/app"
	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/domain"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiTLSService struct {
	profile     *domain.TLSProfile
	renewCalls  int
	cancelCalls int
}

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
func (s *apiTLSService) Disable(context.Context) error {
	s.profile = nil
	return nil
}
func (s *apiTLSService) Check(context.Context) (app.TLSCheckResult, error) {
	return app.TLSCheckResult{Ready: true, Checks: []app.TLSCheck{{Name: "configuration", OK: true, Message: "valid"}}}, nil
}
func (s *apiTLSService) Renew(context.Context) error {
	s.renewCalls++
	return nil
}
func (s *apiTLSService) CancelRenew(context.Context) error {
	s.cancelCalls++
	return nil
}

type apiTLSServiceWithCertificateStatus struct {
	apiTLSService
	status caddyruntime.CertificateStatus
}

func (s *apiTLSServiceWithCertificateStatus) CertificateStatus(context.Context, domain.TLSProfile) caddyruntime.CertificateStatus {
	return s.status
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
	disable := apiRequest(t, server, http.MethodDelete, "/api/v1/tls", "")
	if disable.Code != http.StatusOK || !strings.Contains(disable.Body.String(), `"data":null`) {
		t.Fatalf("disable = %d: %s", disable.Code, disable.Body.String())
	}
	empty = apiRequest(t, server, http.MethodGet, "/api/v1/tls", "")
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"data":null`) {
		t.Fatalf("empty after disable = %d: %s", empty.Code, empty.Body.String())
	}
	invalid := apiRequest(t, server, http.MethodPut, "/api/v1/tls", `{"mode":"internal","hostname":"dav.local","private_key":"secret material"}`)
	if invalid.Code != http.StatusBadRequest || strings.Contains(invalid.Body.String(), "secret material") {
		t.Fatalf("invalid = %d: %s", invalid.Code, invalid.Body.String())
	}
}

func TestTLSAPIGetIncludesAutomaticCertificateStatus(t *testing.T) {
	service := &apiTLSServiceWithCertificateStatus{
		status: caddyruntime.CertificateStatus{
			State:           caddyruntime.CertificateStatusReady,
			StoragePath:     "/var/lib/caddy",
			CertificatePath: "/var/lib/caddy/certificates/example.crt",
			Message:         "ready",
		},
	}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithTLSService(service))
	if err != nil {
		t.Fatal(err)
	}
	update := apiRequest(t, server, http.MethodPut, "/api/v1/tls", `{"mode":"automatic","hostname":"dav.example.com"}`)
	if update.Code != http.StatusOK {
		t.Fatalf("update = %d: %s", update.Code, update.Body.String())
	}
	get := apiRequest(t, server, http.MethodGet, "/api/v1/tls", "")
	body := get.Body.String()
	if get.Code != http.StatusOK || !strings.Contains(body, `"certificate_status"`) || !strings.Contains(body, `"state":"READY"`) {
		t.Fatalf("get = %d: %s", get.Code, body)
	}
}

func TestTLSAPIDoesNotAddCertificateStatusForCustomTLS(t *testing.T) {
	service := &apiTLSServiceWithCertificateStatus{
		status: caddyruntime.CertificateStatus{State: caddyruntime.CertificateStatusReady},
	}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithTLSService(service))
	if err != nil {
		t.Fatal(err)
	}
	update := apiRequest(t, server, http.MethodPut, "/api/v1/tls", `{"mode":"custom","hostname":"dav.local"}`)
	if update.Code != http.StatusOK {
		t.Fatalf("update = %d: %s", update.Code, update.Body.String())
	}
	get := apiRequest(t, server, http.MethodGet, "/api/v1/tls", "")
	if strings.Contains(get.Body.String(), `"certificate_status"`) {
		t.Fatalf("custom TLS response unexpectedly included certificate status: %s", get.Body.String())
	}
}

func TestTLSAPIRenewReturnsAcceptedProfile(t *testing.T) {
	service := &apiTLSServiceWithCertificateStatus{
		status: caddyruntime.CertificateStatus{State: caddyruntime.CertificateStatusReady},
	}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithTLSService(service))
	if err != nil {
		t.Fatal(err)
	}
	update := apiRequest(t, server, http.MethodPut, "/api/v1/tls", `{"mode":"automatic","hostname":"dav.example.com"}`)
	if update.Code != http.StatusOK {
		t.Fatalf("update = %d: %s", update.Code, update.Body.String())
	}
	renew := apiRequest(t, server, http.MethodPost, "/api/v1/tls/renew", "")
	if renew.Code != http.StatusAccepted || !strings.Contains(renew.Body.String(), `"certificate_status"`) || service.renewCalls != 1 {
		t.Fatalf("renew = %d: %s, calls = %d", renew.Code, renew.Body.String(), service.renewCalls)
	}
}

func TestTLSAPICancelRenewReturnsProfile(t *testing.T) {
	service := &apiTLSServiceWithCertificateStatus{
		status: caddyruntime.CertificateStatus{
			State:   caddyruntime.CertificateStatusIssuing,
			Renewal: true,
		},
	}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithTLSService(service))
	if err != nil {
		t.Fatal(err)
	}
	update := apiRequest(t, server, http.MethodPut, "/api/v1/tls", `{"mode":"automatic","hostname":"dav.example.com"}`)
	if update.Code != http.StatusOK {
		t.Fatalf("update = %d: %s", update.Code, update.Body.String())
	}
	cancel := apiRequest(t, server, http.MethodPost, "/api/v1/tls/renew/cancel", "")
	if cancel.Code != http.StatusOK || !strings.Contains(cancel.Body.String(), `"certificate_status"`) || service.cancelCalls != 1 {
		t.Fatalf("cancel = %d: %s, calls = %d", cancel.Code, cancel.Body.String(), service.cancelCalls)
	}
}
