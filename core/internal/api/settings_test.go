package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiSettings struct{ value domain.ServerSettings }

func (s *apiSettings) Get(context.Context) (domain.ServerSettings, error) { return s.value, nil }
func (s *apiSettings) UpdatePorts(_ context.Context, httpPort, httpsPort int) (domain.ServerSettings, error) {
	s.value.HTTPPort, s.value.HTTPSPort = httpPort, httpsPort
	return s.value, nil
}

func TestServerSettingsAPIGetAndUpdate(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC))
	service := &apiSettings{value: domain.ServerSettings{ID: "11111111-1111-4111-8111-111111111111", PublicBasePath: "/dav", HTTPPort: 8080, HTTPSPort: 8443, RuntimeMode: domain.RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithServerSettingsService(service))
	if err != nil {
		t.Fatal(err)
	}
	get := apiRequest(t, server, "GET", "/api/v1/server/settings", "")
	if get.Code != 200 || !strings.Contains(get.Body.String(), `"http_port":8080`) {
		t.Fatalf("get = %d: %s", get.Code, get.Body.String())
	}
	update := apiRequest(t, server, "PUT", "/api/v1/server/settings", `{"http_port":9080,"https_port":9443}`)
	if update.Code != 200 || service.value.HTTPPort != 9080 || service.value.HTTPSPort != 9443 {
		t.Fatalf("update = %d: %s", update.Code, update.Body.String())
	}
}
