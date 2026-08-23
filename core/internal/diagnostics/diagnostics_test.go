package diagnostics

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/app"
	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/domain"
)

type fixedDiagnosticClock struct{}

func (fixedDiagnosticClock) Now() time.Time {
	return time.Date(2026, 8, 20, 1, 2, 3, 0, time.UTC)
}

type staticCheck struct{ result Result }

func (c staticCheck) ID() string                 { return c.result.ID }
func (c staticCheck) Run(context.Context) Result { return c.result }

type panicCheck struct{}

func (panicCheck) ID() string { return "panic" }
func (panicCheck) Run(context.Context) Result {
	panic("management-token-secret")
}

func TestServiceOrdersChecksComputesOverallAndRecoversSafely(t *testing.T) {
	service := NewService([]Check{
		staticCheck{Result{ID: "warning", Title: "Warning", Status: StatusWarn, Message: "Action recommended"}},
		panicCheck{},
		staticCheck{Result{ID: "healthy", Title: "Healthy", Status: StatusPass, Message: "Ready"}},
	}, fixedDiagnosticClock{}, "test")
	report := service.Run(context.Background())
	if report.Overall != StatusFail || !report.Sanitized || report.GeneratedAt != "2026-08-20T01:02:03Z" {
		t.Fatalf("report = %#v", report)
	}
	if report.Build.Product != "DavDeck" || report.Build.Version != "test" || report.Build.GoVersion == "" || report.Build.TargetOS == "" {
		t.Fatalf("build metadata = %#v", report.Build)
	}
	if got := []string{report.Results[0].ID, report.Results[1].ID, report.Results[2].ID}; strings.Join(got, ",") != "healthy,panic,warning" {
		t.Fatalf("order = %#v", got)
	}
	body, _ := json.Marshal(report)
	if strings.Contains(string(body), "management-token-secret") {
		t.Fatalf("panic secret leaked: %s", body)
	}
	latest, ok := service.Latest()
	if !ok || latest.Summary() != "DavDeck diagnostics: FAIL (3 checks)" {
		t.Fatalf("latest = %#v, ok = %t", latest, ok)
	}
}

type failingInspector struct{ err error }

func (i failingInspector) Inspect(context.Context) (caddyruntime.BinaryInfo, error) {
	return caddyruntime.BinaryInfo{}, i.err
}

type failingSnapshots struct{ err error }

func (s failingSnapshots) Snapshot(context.Context) (domain.RuntimeConfigInput, error) {
	return domain.RuntimeConfigInput{}, s.err
}

type unusedCompiler struct{}

func (unusedCompiler) Compile(domain.RuntimeConfigInput) (caddyruntime.CompiledConfig, error) {
	return caddyruntime.CompiledConfig{}, nil
}

type unusedValidator struct{}

func (unusedValidator) Validate(context.Context, []byte) error { return nil }

type failingShares struct{ err error }

func (s failingShares) List(context.Context) ([]domain.Share, error) { return nil, s.err }

type validPaths struct{}

func (validPaths) ValidateSharePath(string) error { return nil }

type failingTLS struct{ err error }

func (t failingTLS) Check(context.Context) (app.TLSCheckResult, error) {
	return app.TLSCheckResult{}, t.err
}

func TestChecksNeverExposeUnderlyingErrorsOrPaths(t *testing.T) {
	secret := "password=hidden management-token-secret /Users/private/key.pem"
	checks := []Check{
		CaddyBinaryCheck{Inspector: failingInspector{errors.New(secret)}},
		ConfigCheck{Snapshots: failingSnapshots{errors.New(secret)}, Compiler: unusedCompiler{}, Validator: unusedValidator{}},
		SharePathsCheck{Shares: failingShares{errors.New(secret)}, Paths: validPaths{}},
		TLSCheck{TLS: failingTLS{&app.Error{Code: app.CodeTLSPrivateKey, Message: "safe", Cause: errors.New(secret)}}},
	}
	body, err := json.Marshal(NewService(checks, fixedDiagnosticClock{}, "test").Run(context.Background()))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"password=hidden", "management-token-secret", "/Users/private/key.pem"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("%q leaked: %s", forbidden, body)
		}
	}
}

func TestDirectoryCheckUsesSafeTemporaryProbeAndRedactsPath(t *testing.T) {
	directory := t.TempDir()
	result := (DirectoryCheck{Name: "data", Path: directory}).Run(context.Background())
	if result.Status != StatusPass {
		t.Fatalf("result = %#v", result)
	}
	entries, err := filepath.Glob(filepath.Join(directory, ".davdeck-diagnostic-*"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("probe files remain: %#v, err = %v", entries, err)
	}
	missing := (DirectoryCheck{Name: "config", Path: filepath.Join(directory, "private", "missing"), Required: true}).Run(context.Background())
	if missing.Status != StatusFail || strings.Contains(missing.Message, directory) {
		t.Fatalf("missing result = %#v", missing)
	}
}
