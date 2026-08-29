package caddy

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/logging"
)

func TestNewRuntimeManagerAllowsColdCaddyStart(t *testing.T) {
	manager := NewRuntimeManager("", "", nil, nil, nil, nil)
	if manager.startTimeout != 15*time.Second {
		t.Fatalf("start timeout = %s, want 15s", manager.startTimeout)
	}
}

func TestRuntimeFailureIsLoggedWithoutRawCause(t *testing.T) {
	store := logging.NewStore(10)
	manager := &RuntimeManager{
		logger: logging.NewWithStore(io.Discard, slog.LevelDebug, "runtime", store),
	}
	failure := &RuntimeError{
		Code:    CodeCaddyStartFailed,
		Message: "Unable to start Caddy",
		Cause:   errors.New("management_token=do-not-log"),
	}
	if err := manager.recordFailure(failure); err != failure {
		t.Fatalf("recordFailure returned %v, want original error", err)
	}
	page := store.Query(logging.Query{Limit: 10})
	if len(page.Records) != 1 {
		t.Fatalf("records = %#v", page.Records)
	}
	record := page.Records[0]
	if record.Component != "runtime" || record.Level != "ERROR" || record.Message != "managed Caddy operation failed" {
		t.Fatalf("record = %#v", record)
	}
	if record.Fields["error_code"] != string(CodeCaddyStartFailed) || record.Fields["error"] != "Unable to start Caddy" {
		t.Fatalf("fields = %#v", record.Fields)
	}
	if _, ok := record.Fields["cause"]; ok {
		t.Fatalf("raw cause was logged: %#v", record.Fields)
	}
}
