package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiDNSProviders struct {
	provider app.DNSProviderCredentialView
	secret   domain.DNSProviderSecret
}

func (s *apiDNSProviders) List(context.Context) ([]app.DNSProviderCredentialView, error) {
	if s.provider.ID == "" {
		return []app.DNSProviderCredentialView{}, nil
	}
	return []app.DNSProviderCredentialView{s.provider}, nil
}

func (s *apiDNSProviders) Get(context.Context, domain.ID) (app.DNSProviderCredentialView, error) {
	return s.provider, nil
}

func (s *apiDNSProviders) Save(_ context.Context, update app.DNSProviderUpdate) (app.DNSProviderCredentialView, error) {
	s.secret = update.Secret
	stamp := s.provider.CreatedAt
	if stamp.Time().IsZero() {
		stamp, _ = domain.NewTimestamp(fixedAPITime())
	}
	s.provider = app.DNSProviderCredentialView{ID: "11111111-1111-4111-8111-111111111111", Name: update.Name, Provider: update.Provider, AllowedZones: update.AllowedZones, SecretConfigured: len(update.Secret) > 0, CreatedAt: stamp, UpdatedAt: stamp}
	return s.provider, nil
}

func (*apiDNSProviders) Delete(context.Context, domain.ID) error { return nil }

func fixedAPITime() time.Time {
	return time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC)
}

func TestDNSProviderAPIManagesMetadataWithoutReturningSecrets(t *testing.T) {
	service := &apiDNSProviders{}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithDNSProviderService(service))
	if err != nil {
		t.Fatal(err)
	}
	create := apiRequest(t, server, http.MethodPost, "/api/v1/dns/providers", `{"name":"Cloudflare production","provider":"cloudflare","allowed_zones":["example.com"],"secret":{"api_token":"secret-token"}}`)
	if create.Code != http.StatusCreated || strings.Contains(create.Body.String(), "secret-token") || !strings.Contains(create.Body.String(), `"secret_configured":true`) {
		t.Fatalf("create = %d: %s", create.Code, create.Body.String())
	}
	if service.secret["api_token"] != "secret-token" {
		t.Fatalf("secret was not passed to service: %#v", service.secret)
	}

	list := apiRequest(t, server, http.MethodGet, "/api/v1/dns/providers", "")
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), "secret-token") {
		t.Fatalf("list = %d: %s", list.Code, list.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/dns/providers", bytes.NewBufferString(""))
	request.Header.Set("Authorization", "Bearer wrong")
	unauthorized := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(unauthorized, request)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized = %d", unauthorized.Code)
	}
}
