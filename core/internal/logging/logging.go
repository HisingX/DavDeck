// Package logging configures DavDeck's structured application logging.
package logging

import (
	"context"
	"io"
	"log/slog"
)

// New returns a JSON logger with stable component metadata.
func New(output io.Writer, level slog.Leveler, component string) *slog.Logger {
	handler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level, ReplaceAttr: redactAttr})
	return slog.New(handler).With("component", component)
}

// NewWithStore returns a sanitized JSON logger that also retains recent
// records in the daemon-owned bounded store.
func NewWithStore(output io.Writer, level slog.Leveler, component string, store *Store) *slog.Logger {
	outputHandler := slog.NewJSONHandler(output, &slog.HandlerOptions{Level: level, ReplaceAttr: redactAttr})
	handler := &storeHandler{output: outputHandler, store: store, component: component}
	return slog.New(handler)
}

type storeHandler struct {
	output    slog.Handler
	store     *Store
	attrs     []slog.Attr
	component string
}

func (h *storeHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.output.Enabled(ctx, level)
}

func (h *storeHandler) Handle(ctx context.Context, record slog.Record) error {
	fields := make(map[string]any)
	component := h.component
	if component == "" {
		component = "daemon"
	}
	for _, attr := range h.attrs {
		appendAttr(fields, "", attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		appendAttr(fields, "", attr)
		return true
	})
	if value, ok := fields["component"].(string); ok && value != "" {
		component = value
		delete(fields, "component")
	}
	if h.store != nil {
		h.store.Add(Record{Timestamp: record.Time, Level: record.Level.String(), Component: component, Message: record.Message, Fields: fields})
	}
	record.Message = sanitizeMessage(record.Message)
	return h.output.Handle(ctx, record)
}

func (h *storeHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	component := h.component
	remaining := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		if attr.Key == "component" && attr.Value.Kind() == slog.KindString {
			component = attr.Value.String()
			continue
		}
		remaining = append(remaining, attr)
	}
	cloned := &storeHandler{output: h.output.WithAttrs(remaining), store: h.store, attrs: append(append([]slog.Attr(nil), h.attrs...), remaining...), component: component}
	return cloned
}

func (h *storeHandler) WithGroup(name string) slog.Handler {
	return &storeHandler{output: h.output.WithGroup(name), store: h.store, attrs: h.attrs, component: h.component}
}

func appendAttr(fields map[string]any, prefix string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	key := attr.Key
	if prefix != "" {
		key = prefix + "." + key
	}
	if attr.Value.Kind() == slog.KindGroup {
		for _, child := range attr.Value.Group() {
			appendAttr(fields, key, child)
		}
		return
	}
	fields[key] = sanitizeValue(attr.Value.Any())
}

func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	if isSecretField(attr.Key) {
		return slog.String(attr.Key, RedactedValue)
	}
	if attr.Value.Kind() == slog.KindAny {
		if err, ok := attr.Value.Any().(error); ok {
			return slog.String(attr.Key, sanitizeMessage(err.Error()))
		}
	}
	if attr.Value.Kind() == slog.KindString {
		return slog.String(attr.Key, sanitizeMessage(attr.Value.String()))
	}
	return attr
}

var _ slog.Handler = (*storeHandler)(nil)
