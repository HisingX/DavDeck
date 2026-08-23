package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/logging"
	"davdeck.dev/davdeck/core/internal/status"
)

func TestLogsAPIProvidesSanitizedPagedFilteredRecords(t *testing.T) {
	store := logging.NewStore(10)
	base := time.Date(2026, 8, 23, 1, 0, 0, 0, time.UTC)
	store.Add(logging.Record{Timestamp: base, Level: "INFO", Component: "daemon", Message: "started"})
	store.Add(logging.Record{Timestamp: base.Add(time.Minute), Level: "ERROR", Component: "caddy", Message: "Authorization: Bearer hidden"})
	store.Add(logging.Record{Timestamp: base.Add(2 * time.Minute), Level: "WARN", Component: "platform", Message: "service warning"})
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithLogStore(store))
	if err != nil {
		t.Fatal(err)
	}

	first := apiRequest(t, server, http.MethodGet, "/api/v1/logs?limit=2", "")
	if first.Code != http.StatusOK || !strings.Contains(first.Body.String(), `"has_more":true`) || !strings.Contains(first.Body.String(), `"component":"platform"`) {
		t.Fatalf("first page = %d: %s", first.Code, first.Body.String())
	}
	if strings.Contains(first.Body.String(), "hidden") {
		t.Fatal("log secret leaked through API")
	}

	second := apiRequest(t, server, http.MethodGet, "/api/v1/logs?limit=2&cursor=2", "")
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"component":"daemon"`) || strings.Contains(second.Body.String(), `"component":"platform"`) {
		t.Fatalf("second page = %d: %s", second.Code, second.Body.String())
	}
	filtered := apiRequest(t, server, http.MethodGet, "/api/v1/logs?level=error&component=CADDY", "")
	if filtered.Code != http.StatusOK || !strings.Contains(filtered.Body.String(), `"component":"caddy"`) || strings.Contains(filtered.Body.String(), `"component":"daemon"`) {
		t.Fatalf("filtered page = %d: %s", filtered.Code, filtered.Body.String())
	}
}

func TestLogsAPIRejectsInvalidQueriesAndReportsUnavailableStore(t *testing.T) {
	store := logging.NewStore(10)
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithLogStore(store))
	if err != nil {
		t.Fatal(err)
	}
	empty := apiRequest(t, server, http.MethodGet, "/api/v1/logs", "")
	if empty.Code != http.StatusOK || !strings.Contains(empty.Body.String(), `"records":[]`) {
		t.Fatalf("empty logs = %d: %s", empty.Code, empty.Body.String())
	}
	invalid := apiRequest(t, server, http.MethodGet, "/api/v1/logs?limit=201", "")
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), `"code":"INVALID_LOG_QUERY"`) {
		t.Fatalf("invalid query = %d: %s", invalid.Code, invalid.Body.String())
	}

	withoutStore, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	unavailable := apiRequest(t, withoutStore, http.MethodGet, "/api/v1/logs", "")
	if unavailable.Code != http.StatusServiceUnavailable || !strings.Contains(unavailable.Body.String(), `"code":"LOGS_UNAVAILABLE"`) {
		t.Fatalf("unavailable logs = %d: %s", unavailable.Code, unavailable.Body.String())
	}
}
