package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/platform"
	"davdeck.dev/davdeck/core/internal/status"
)

func TestValidateLoopbackAddress(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		address string
		valid   bool
	}{
		{"127.0.0.1:0", true}, {"[::1]:8080", true},
		{"0.0.0.0:8080", false}, {"192.168.1.10:8080", false}, {"localhost:8080", false},
	} {
		err := ValidateLoopbackAddress(testCase.address)
		if (err == nil) != testCase.valid {
			t.Errorf("ValidateLoopbackAddress(%q) error = %v, valid = %v", testCase.address, err, testCase.valid)
		}
	}
}

func TestStatusRequiresValidToken(t *testing.T) {
	t.Parallel()
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{Name: "DavDeck", Daemon: status.DaemonRunning}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name       string
		token      string
		statusCode int
	}{
		{"missing", "", http.StatusUnauthorized},
		{"invalid", "wrong", http.StatusUnauthorized},
		{"valid", "secret", http.StatusOK},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, statusPath, nil)
			if testCase.token != "" {
				request.Header.Set("Authorization", "Bearer "+testCase.token)
			}
			response := httptest.NewRecorder()
			server.http.Handler.ServeHTTP(response, request)
			if response.Code != testCase.statusCode {
				t.Fatalf("status = %d, want %d", response.Code, testCase.statusCode)
			}
			var payload Envelope
			if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload.Success != (testCase.statusCode == http.StatusOK) {
				t.Fatalf("success = %v", payload.Success)
			}
		})
	}
}

func TestStatusAggregatesRuntimeServiceAndOwnershipState(t *testing.T) {
	runtime := &apiRuntime{state: caddyruntime.RuntimeFailed}
	service := &apiServiceManager{status: platform.ServiceStatus{Installed: true, State: platform.ServiceStateFailed, StartsAtBoot: true}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{
		Name:                "DavDeck",
		PortableDaemonOwned: true,
	}, nil, WithRuntimeService(runtime), WithServiceManager(service))
	if err != nil {
		t.Fatal(err)
	}
	response := apiRequest(t, server, http.MethodGet, statusPath, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	for _, fragment := range []string{
		`"daemon":"RUNNING"`,
		`"caddy":"FAILED"`,
		`"webdav":"FAILED"`,
		`"state":"FAILED"`,
		`"starts_at_boot":true`,
		`"portable_daemon_owned":true`,
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("status body %q does not contain %q", body, fragment)
		}
	}
}

func TestStatusDoesNotPromoteUnsupportedDesktopServiceToRuntimeError(t *testing.T) {
	runtime := &apiRuntime{state: caddyruntime.RuntimeRunning}
	service := &apiServiceManager{err: &platform.ServiceError{Code: platform.CodePlatformUnsupported, Message: "Linux only"}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithRuntimeService(runtime), WithServiceManager(service))
	if err != nil {
		t.Fatal(err)
	}

	response := apiRequest(t, server, http.MethodGet, statusPath, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			LastErrorCode string `json:"last_error_code"`
			Service       struct {
				LastErrorCode string `json:"last_error_code"`
			} `json:"service"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.LastErrorCode != "" {
		t.Fatalf("unsupported service leaked into runtime error: %q", payload.Data.LastErrorCode)
	}
	if payload.Data.Service.LastErrorCode != string(platform.CodePlatformUnsupported) {
		t.Fatalf("service error = %q", payload.Data.Service.LastErrorCode)
	}
}

func TestAPIUsesJSONEnvelopeForErrors(t *testing.T) {
	t.Parallel()
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{Name: "DavDeck"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name       string
		method     string
		path       string
		statusCode int
		code       ErrorCode
	}{
		{"unknown route", http.MethodGet, "/api/v1/missing", http.StatusNotFound, ErrorNotFound},
		{"wrong method", http.MethodPost, statusPath, http.StatusMethodNotAllowed, ErrorMethodNotAllowed},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			request.Header.Set("Authorization", "Bearer secret")
			response := httptest.NewRecorder()
			server.http.Handler.ServeHTTP(response, request)
			if response.Code != testCase.statusCode {
				t.Fatalf("status = %d, want %d", response.Code, testCase.statusCode)
			}
			if got := response.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
				t.Fatalf("content type = %q", got)
			}
			if response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unexpected permissive CORS header")
			}
			var payload Envelope
			if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload.Success || payload.Error == nil || payload.Error.Code != testCase.code {
				t.Fatalf("payload = %#v", payload)
			}
		})
	}
}

func TestDaemonShutdownRequiresPostAndSignals(t *testing.T) {
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	wrongMethod := httptest.NewRequest(http.MethodGet, daemonShutdownPath, nil)
	wrongMethod.Header.Set("Authorization", "Bearer secret")
	wrongMethodResponse := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(wrongMethodResponse, wrongMethod)
	if wrongMethodResponse.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status = %d, want %d", wrongMethodResponse.Code, http.StatusMethodNotAllowed)
	}

	unauthorized := httptest.NewRequest(http.MethodPost, daemonShutdownPath, nil)
	unauthorizedResponse := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(unauthorizedResponse, unauthorized)
	if unauthorizedResponse.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized POST status = %d, want %d", unauthorizedResponse.Code, http.StatusUnauthorized)
	}

	request := httptest.NewRequest(http.MethodPost, daemonShutdownPath, nil)
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST status = %d, want %d", response.Code, http.StatusOK)
	}
	select {
	case <-server.ShutdownRequested():
	default:
		t.Fatal("daemon shutdown request was not signaled")
	}
}
