package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiConfigFiles struct{ imported []byte }

func (s *apiConfigFiles) Export(context.Context) ([]byte, error) {
	return []byte("version: 1\nusers: []\n"), nil
}
func (s *apiConfigFiles) Import(_ context.Context, body []byte) (app.ConfigImportResult, error) {
	s.imported = append([]byte(nil), body...)
	return app.ConfigImportResult{UsersCreated: 1, PasswordResetRequired: []string{"Alice"}, PendingApply: true}, nil
}

func TestConfigFileAPIExportsEnvelopeAndAcceptsOnlyYAML(t *testing.T) {
	service := &apiConfigFiles{}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithConfigService(service))
	if err != nil {
		t.Fatal(err)
	}
	exported := apiRequest(t, server, http.MethodGet, "/api/v1/config/export", "")
	if exported.Code != http.StatusOK || !strings.Contains(exported.Body.String(), `"format":"yaml"`) || !strings.Contains(exported.Body.String(), `"contains_secrets":false`) {
		t.Fatalf("export = %d: %s", exported.Code, exported.Body.String())
	}
	body := "version: 1\nusers: []\n"
	request := httptest.NewRequest(http.MethodPost, "/api/v1/config/import", bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Content-Type", "application/yaml")
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || string(service.imported) != body || !strings.Contains(response.Body.String(), `"pending_apply":true`) {
		t.Fatalf("import = %d: %s, body = %q", response.Code, response.Body.String(), service.imported)
	}
	wrongType := apiRequest(t, server, http.MethodPost, "/api/v1/config/import", `{"version":1}`)
	if wrongType.Code != http.StatusBadRequest || len(service.imported) != len(body) {
		t.Fatalf("wrong type = %d: %s", wrongType.Code, wrongType.Body.String())
	}
}
