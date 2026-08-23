package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"davdeck.dev/davdeck/core/internal/logging"
	"davdeck.dev/davdeck/core/internal/platform"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiServiceManager struct {
	status platform.ServiceStatus
	calls  []string
	err    error
}

func (s *apiServiceManager) Install(context.Context) error {
	s.calls = append(s.calls, "install")
	return s.err
}

func (s *apiServiceManager) Uninstall(context.Context) error {
	s.calls = append(s.calls, "uninstall")
	return s.err
}

func (s *apiServiceManager) Start(context.Context) error {
	s.calls = append(s.calls, "start")
	return s.err
}

func (s *apiServiceManager) Stop(context.Context) error {
	s.calls = append(s.calls, "stop")
	return s.err
}

func (s *apiServiceManager) Status(context.Context) (platform.ServiceStatus, error) {
	s.calls = append(s.calls, "status")
	return s.status, s.err
}

func TestServiceAPIRequiresAuthenticationAndControlsManager(t *testing.T) {
	manager := &apiServiceManager{status: platform.ServiceStatus{Installed: true, State: platform.ServiceStateRunning, StartsAtBoot: true}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithServiceManager(manager))
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/service/status", nil)
	unauthenticated := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(unauthenticated, request)
	if unauthenticated.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d: %s", unauthenticated.Code, unauthenticated.Body.String())
	}

	statusResponse := apiRequest(t, server, http.MethodGet, "/api/v1/service/status", "")
	if statusResponse.Code != http.StatusOK || !strings.Contains(statusResponse.Body.String(), `"state":"RUNNING"`) || !strings.Contains(statusResponse.Body.String(), `"starts_at_boot":true`) {
		t.Fatalf("status = %d: %s", statusResponse.Code, statusResponse.Body.String())
	}
	for _, operation := range []string{"install", "uninstall", "start", "stop"} {
		response := apiRequest(t, server, http.MethodPost, "/api/v1/service/"+operation, "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s = %d: %s", operation, response.Code, response.Body.String())
		}
	}
	if strings.Join(manager.calls, ",") != "status,install,uninstall,start,stop" {
		t.Fatalf("calls = %v", manager.calls)
	}
}

func TestServiceAPIMapsPrivilegeAndFailureErrorsWithoutLeakingCause(t *testing.T) {
	store := logging.NewStore(10)
	manager := &apiServiceManager{err: &platform.ServiceError{
		Code:    platform.CodePrivilegeRequired,
		Message: "Administrator privileges are required to manage the system service",
		Cause:   errors.New("private command output must not be returned"),
	}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, logging.NewWithStore(io.Discard, slog.LevelDebug, "daemon", store), WithServiceManager(manager), WithLogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	response := apiRequest(t, server, http.MethodPost, "/api/v1/service/install", "")
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"code":"PRIVILEGE_REQUIRED"`) {
		t.Fatalf("privilege error = %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "private command output") {
		t.Fatal("service error cause leaked to API response")
	}
	page := store.Query(logging.Query{Limit: 10})
	if len(page.Records) != 1 || page.Records[0].Component != "platform" || page.Records[0].Fields["error_code"] != string(platform.CodePrivilegeRequired) {
		t.Fatalf("platform log = %#v", page.Records)
	}
	if strings.Contains(page.Records[0].Message, "private command output") {
		t.Fatal("service error cause leaked to logs")
	}

	manager.err = &platform.ServiceError{Code: platform.CodeServiceStatusFailed, Message: "status unavailable"}
	response = apiRequest(t, server, http.MethodGet, "/api/v1/service/status", "")
	if response.Code != http.StatusInternalServerError || !strings.Contains(response.Body.String(), `"code":"SERVICE_STATUS_FAILED"`) {
		t.Fatalf("status error = %d: %s", response.Code, response.Body.String())
	}
}
