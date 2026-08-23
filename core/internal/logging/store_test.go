package logging

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestStoreRedactsKnownSecretsBeforePersistence(t *testing.T) {
	store := NewStore(10)
	store.Add(Record{
		Timestamp: time.Date(2026, 8, 23, 1, 2, 3, 0, time.UTC),
		Level:     "error",
		Component: "runtime",
		Message:   `Authorization: Bearer abc password=secret {"password":"json-secret"}`,
		Fields: map[string]any{
			"password":         "secret",
			"management_token": "token",
			"safe":             "value",
		},
	})
	page := store.Query(Query{Limit: 1})
	if len(page.Records) != 1 {
		t.Fatalf("records = %#v", page.Records)
	}
	record := page.Records[0]
	if strings.Contains(record.Message, "abc") || strings.Contains(record.Message, "secret") || strings.Contains(record.Message, "json-secret") {
		t.Fatalf("message leaked secret: %q", record.Message)
	}
	if record.Fields["password"] != RedactedValue || record.Fields["management_token"] != RedactedValue || record.Fields["safe"] != "value" {
		t.Fatalf("fields were not redacted: %#v", record.Fields)
	}
}

func TestStorePaginationAndFiltersAreDeterministic(t *testing.T) {
	store := NewStore(10)
	base := time.Date(2026, 8, 23, 1, 0, 0, 0, time.UTC)
	for index, value := range []struct {
		level, component string
	}{
		{"INFO", "daemon"},
		{"ERROR", "caddy"},
		{"WARN", "platform"},
	} {
		stamp := base.Add(time.Duration(index) * time.Minute)
		store.Add(Record{Timestamp: stamp, Level: value.level, Component: value.component, Message: value.component})
	}
	first := store.Query(Query{Limit: 2})
	if len(first.Records) != 2 || first.Records[0].Component != "platform" || first.Records[1].Component != "caddy" || !first.HasMore || first.NextCursor != first.Records[1].ID {
		t.Fatalf("first page = %#v", first)
	}
	second := store.Query(Query{Limit: 2, Cursor: first.NextCursor})
	if len(second.Records) != 1 || second.Records[0].Component != "daemon" || second.HasMore {
		t.Fatalf("second page = %#v", second)
	}
	filtered := store.Query(Query{Limit: 10, Level: "error", Component: "CADDY"})
	if len(filtered.Records) != 1 || filtered.Records[0].Message != "caddy" {
		t.Fatalf("filtered page = %#v", filtered)
	}
	since := base.Add(2 * time.Minute)
	filtered = store.Query(Query{Limit: 10, Since: &since})
	if len(filtered.Records) != 1 || filtered.Records[0].Component != "platform" {
		t.Fatalf("since page = %#v", filtered)
	}
}

func TestStructuredLoggerAndLineWriterUseTheSameSanitizedBoundary(t *testing.T) {
	store := NewStore(10)
	var output bytes.Buffer
	logger := NewWithStore(&output, slog.LevelDebug, "daemon", store)
	logger.With("component", "runtime").Error("password=secret", "password", "secret")
	writer := NewLineWriter(store, "caddy", "ERROR", &output)
	_, _ = writer.Write([]byte(`{"level":"error","ts":1787446923.5,"msg":"Authorization: Bearer caddy-token","logger":"admin.api"}` + "\n"))
	writer.Flush()
	page := store.Query(Query{Limit: 10})
	if len(page.Records) != 2 || page.Records[0].Component != "caddy" || page.Records[1].Component != "runtime" {
		t.Fatalf("records = %#v", page.Records)
	}
	if strings.Contains(output.String(), "secret") || strings.Contains(output.String(), "caddy-token") {
		t.Fatalf("logger output leaked a secret: %s", output.String())
	}
	logger.Error("error boundary", "error", errors.New("management_token=error-secret"))
	if strings.Contains(output.String(), "error-secret") {
		t.Fatalf("error value leaked a secret: %s", output.String())
	}
	if err := logger.Handler().Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "safe", 0)); err != nil {
		t.Fatal(err)
	}
}
