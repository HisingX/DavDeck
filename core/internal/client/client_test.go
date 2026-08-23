package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

func TestNewAcceptsOnlyLoopbackHTTP(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		endpoint string
		valid    bool
	}{
		{"http://127.0.0.1:8080", true},
		{"http://[::1]:8080", true},
		{"http://127.0.0.1:8080/", false},
		{"https://127.0.0.1:8080", false},
		{"http://localhost:8080", false},
		{"http://127.0.0.1:8080@evil.example", false},
		{"http://192.168.1.10:8080", false},
		{"http://127.0.0.1", false},
	} {
		_, err := New(testCase.endpoint, "token")
		if (err == nil) != testCase.valid {
			t.Errorf("New(%q) error = %v, valid = %v", testCase.endpoint, err, testCase.valid)
		}
	}
}

func TestConfigValidationRestoreAndLogsUseManagementAPI(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/config/validate":
			if request.Method != http.MethodPost {
				t.Errorf("validate method = %s", request.Method)
			}
			_, _ = writer.Write([]byte(`{"success":true,"data":{"valid":true,"config_hash":"abc","warnings":["warning"]}}`))
		case "/api/v1/revisions/11111111-1111-4111-8111-111111111111/restore":
			if request.Method != http.MethodPost {
				t.Errorf("restore method = %s", request.Method)
			}
			_, _ = writer.Write([]byte(`{"success":true,"data":{"id":"11111111-1111-4111-8111-111111111111","number":4,"config_hash":"abc","validation_status":"VALID","apply_status":"APPLIED","app_version":"test"}}`))
		case "/api/v1/logs":
			if request.Method != http.MethodGet || request.URL.Query().Get("limit") != "2" || request.URL.Query().Get("cursor") != "8" || request.URL.Query().Get("level") != "ERROR" || request.URL.Query().Get("component") != "caddy" {
				t.Errorf("logs request = %s %s", request.Method, request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"success":true,"data":{"records":[{"id":9,"timestamp":"2026-08-20T01:02:03Z","level":"ERROR","component":"caddy","message":"reload failed"}],"next_cursor":9,"has_more":true}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	apiClient, err := New(server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	validation, err := apiClient.ValidateConfig(context.Background())
	if err != nil || !validation.Valid || validation.ConfigHash != "abc" || len(validation.Warnings) != 1 {
		t.Fatalf("validation = %#v, error = %v", validation, err)
	}
	revision, err := apiClient.RestoreRevision(context.Background(), domain.ID("11111111-1111-4111-8111-111111111111"))
	if err != nil || revision.Number != 4 {
		t.Fatalf("revision = %#v, error = %v", revision, err)
	}
	since := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	page, err := apiClient.Logs(context.Background(), LogQuery{Limit: 2, Cursor: 8, Since: &since, Level: "ERROR", Component: "caddy"})
	if err != nil || len(page.Records) != 1 || !page.HasMore || page.Records[0].Message != "reload failed" {
		t.Fatalf("logs = %#v, error = %v", page, err)
	}
}

func TestConfigExportAndImportPreserveYAMLBody(t *testing.T) {
	t.Parallel()
	yaml := "version: 1\nusers: []\n"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/v1/config/export":
			if request.Method != http.MethodGet {
				t.Errorf("export method = %s", request.Method)
			}
			_, _ = writer.Write([]byte(`{"success":true,"data":{"format":"yaml","content":"version: 1\nusers: []\n","contains_secrets":false}}`))
		case "/api/v1/config/import":
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Error(err)
			}
			if request.Method != http.MethodPost || request.Header.Get("Content-Type") != "application/yaml" || string(body) != yaml {
				t.Errorf("import request = %s %q %q", request.Method, request.Header.Get("Content-Type"), body)
			}
			_, _ = writer.Write([]byte(`{"success":true,"data":{"users_created":1,"password_reset_required":["Alice"],"pending_apply":true}}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	apiClient, err := New(server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	exported, err := apiClient.ExportConfig(context.Background())
	if err != nil || exported != yaml {
		t.Fatalf("export = %q, error = %v", exported, err)
	}
	result, err := apiClient.ImportConfig(context.Background(), []byte(yaml))
	if err != nil || result.UsersCreated != 1 || !result.PendingApply || len(result.PasswordResetRequired) != 1 {
		t.Fatalf("import = %#v, error = %v", result, err)
	}
}

func TestConfigExportRejectsSecretBearingEnvelope(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"success":true,"data":{"format":"yaml","content":"password: leaked","contains_secrets":true}}`))
	}))
	defer server.Close()
	apiClient, err := New(server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := apiClient.ExportConfig(context.Background()); err == nil {
		t.Fatal("secret-bearing export was accepted")
	}
}

func TestStatusReturnsTypedAPIError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"success":false,"error":{"code":"UNAUTHORIZED","message":"Authentication required"}}`))
	}))
	defer server.Close()
	apiClient, err := New(server.URL, "token")
	if err != nil {
		t.Fatal(err)
	}
	_, err = apiClient.Status(context.Background())
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.Code != "UNAUTHORIZED" || apiError.StatusCode != http.StatusUnauthorized {
		t.Fatalf("error = %#v", err)
	}
}
