package api

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/diagnostics"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiDiagnosticClock struct{}

func (apiDiagnosticClock) Now() time.Time {
	return time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC)
}

type apiDiagnosticCheck struct{}

func (apiDiagnosticCheck) ID() string { return "daemon" }
func (apiDiagnosticCheck) Run(_ context.Context) diagnostics.Result {
	return diagnostics.Result{ID: "daemon", Title: "Daemon", Status: diagnostics.StatusPass, Message: "DavDeck daemon is running"}
}

func TestDiagnosticsAPIRequiresRunAndOnlyServesRedactedReports(t *testing.T) {
	service := diagnostics.NewService([]diagnostics.Check{apiDiagnosticCheck{}}, apiDiagnosticClock{}, "test")
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithDiagnosticsService(service))
	if err != nil {
		t.Fatal(err)
	}
	missing := apiRequest(t, server, http.MethodGet, "/api/v1/diagnostics/report?mode=redacted", "")
	if missing.Code != http.StatusNotFound || !strings.Contains(missing.Body.String(), "DIAGNOSTICS_NOT_RUN") {
		t.Fatalf("missing = %d: %s", missing.Code, missing.Body.String())
	}
	run := apiRequest(t, server, http.MethodPost, "/api/v1/diagnostics/run", "")
	if run.Code != http.StatusOK || !strings.Contains(run.Body.String(), `"sanitized":true`) || !strings.Contains(run.Body.String(), `"overall":"PASS"`) {
		t.Fatalf("run = %d: %s", run.Code, run.Body.String())
	}
	report := apiRequest(t, server, http.MethodGet, "/api/v1/diagnostics/report?mode=redacted", "")
	if report.Code != http.StatusOK || !strings.Contains(report.Body.String(), `"app_version":"test"`) {
		t.Fatalf("report = %d: %s", report.Code, report.Body.String())
	}
	full := apiRequest(t, server, http.MethodGet, "/api/v1/diagnostics/report?mode=full", "")
	if full.Code != http.StatusBadRequest {
		t.Fatalf("full report status = %d: %s", full.Code, full.Body.String())
	}
}
