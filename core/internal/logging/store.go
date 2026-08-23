package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	DefaultCapacity = 2000
	DefaultPageSize = 100
	MaximumPageSize = 200
	RedactedValue   = "[REDACTED]"
)

// Record is the sanitized representation exposed by the Logs API.
type Record struct {
	ID        uint64         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Level     string         `json:"level"`
	Component string         `json:"component"`
	Message   string         `json:"message"`
	Fields    map[string]any `json:"fields,omitempty"`
}

// Query selects a bounded page from newest to oldest. Cursor is the exclusive
// upper bound for record IDs; a zero cursor starts at the newest record.
type Query struct {
	Limit     int
	Cursor    uint64
	Since     *time.Time
	Level     string
	Component string
}

type Page struct {
	Records    []Record `json:"records"`
	NextCursor uint64   `json:"next_cursor,omitempty"`
	HasMore    bool     `json:"has_more"`
}

// Store is a bounded in-memory log boundary. It deliberately has no file
// access; rotation or unavailable external log files cannot expose arbitrary
// user-selected content through the Management API.
type Store struct {
	mu       sync.RWMutex
	capacity int
	nextID   uint64
	records  []Record
}

func NewStore(capacity int) *Store {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	return &Store{capacity: capacity, records: make([]Record, 0, capacity)}
}

func (s *Store) Add(record Record) {
	if s == nil {
		return
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now().UTC()
	} else {
		record.Timestamp = record.Timestamp.UTC()
	}
	record.Level = normalizeLevel(record.Level)
	if record.Component == "" {
		record.Component = "daemon"
	}
	record.Component = sanitizeMessage(record.Component)
	record.Message = sanitizeMessage(record.Message)
	record.Fields = sanitizeFields(record.Fields)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	record.ID = s.nextID
	if len(s.records) == s.capacity {
		copy(s.records, s.records[1:])
		s.records[len(s.records)-1] = record
		return
	}
	s.records = append(s.records, record)
}

func (s *Store) Query(query Query) Page {
	if s == nil {
		return Page{Records: []Record{}}
	}
	limit := query.Limit
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaximumPageSize {
		limit = MaximumPageSize
	}
	level := strings.ToUpper(strings.TrimSpace(query.Level))
	component := strings.ToLower(strings.TrimSpace(query.Component))

	s.mu.RLock()
	defer s.mu.RUnlock()
	result := Page{Records: make([]Record, 0, limit)}
	for index := len(s.records) - 1; index >= 0; index-- {
		record := s.records[index]
		if query.Cursor > 0 && record.ID >= query.Cursor {
			continue
		}
		if query.Since != nil && record.Timestamp.Before(query.Since.UTC()) {
			continue
		}
		if level != "" && record.Level != level {
			continue
		}
		if component != "" && strings.ToLower(record.Component) != component {
			continue
		}
		if len(result.Records) == limit {
			result.HasMore = true
			result.NextCursor = result.Records[len(result.Records)-1].ID
			break
		}
		result.Records = append(result.Records, cloneRecord(sanitizeRecord(record)))
	}
	return result
}

func sanitizeRecord(record Record) Record {
	record.Level = normalizeLevel(record.Level)
	record.Component = sanitizeMessage(record.Component)
	record.Message = sanitizeMessage(record.Message)
	record.Fields = sanitizeFields(record.Fields)
	return record
}

func cloneRecord(record Record) Record {
	if record.Fields == nil {
		return record
	}
	fields := make(map[string]any, len(record.Fields))
	for key, value := range record.Fields {
		fields[key] = value
	}
	record.Fields = fields
	return record
}

func normalizeLevel(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "DEBUG":
		return "DEBUG"
	case "WARN", "WARNING":
		return "WARN"
	case "ERROR":
		return "ERROR"
	case "INFO":
		return "INFO"
	default:
		return "INFO"
	}
}

var (
	jsonSecretValuePattern = regexp.MustCompile(`(?i)("(?:password(?:_hash)?|management[_-]?token|authorization|bearer|private[_-]?key(?:_path)?|dns[_-]?(?:api[_-]?)?token|api[_-]?key|client[_-]?secret|secret|credential)s?"\s*:\s*)("[^"]*"|'[^']*'|[^,\s}]+)`)
	secretValuePattern     = regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+|\b(?:password(?:_hash)?|management[_-]?token|token|private[_-]?key|dns[_-]?(?:api[_-]?)?token|api[_-]?key|secret)\b\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`)
	pemPrivateKeyPattern   = regexp.MustCompile(`(?s)-----BEGIN [^-]*PRIVATE KEY-----.*?-----END [^-]*PRIVATE KEY-----`)
)

func isSecretField(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(key))
	switch normalized {
	case "password", "passwordhash", "managementtoken", "authorization", "bearer", "privatekey", "privatekeypath", "dnstoken", "dnsapitoken", "apikey", "clientsecret", "secret", "credential", "credentials", "token":
		return true
	default:
		return strings.HasSuffix(normalized, "password") || strings.HasSuffix(normalized, "passwordhash") || strings.HasSuffix(normalized, "token") || strings.HasSuffix(normalized, "secret") || strings.HasSuffix(normalized, "privatekey") || strings.HasSuffix(normalized, "apikey")
	}
}

func sanitizeFields(fields map[string]any) map[string]any {
	if len(fields) == 0 {
		return nil
	}
	result := make(map[string]any, len(fields))
	for key, value := range fields {
		if isSecretField(key) {
			result[key] = RedactedValue
			continue
		}
		result[key] = sanitizeValue(value)
	}
	return result
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case string:
		return sanitizeMessage(typed)
	case []byte:
		return sanitizeMessage(string(typed))
	case error:
		return sanitizeMessage(typed.Error())
	case map[string]any:
		return sanitizeFields(typed)
	case map[string]string:
		result := make(map[string]string, len(typed))
		for key, item := range typed {
			if isSecretField(key) {
				result[key] = RedactedValue
				continue
			}
			result[key] = sanitizeMessage(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = sanitizeValue(item)
		}
		return result
	default:
		return value
	}
}

func sanitizeMessage(message string) string {
	message = pemPrivateKeyPattern.ReplaceAllString(message, RedactedValue)
	message = jsonSecretValuePattern.ReplaceAllString(message, `${1}`+`"`+RedactedValue+`"`)
	return secretValuePattern.ReplaceAllString(message, `${1}`+RedactedValue)
}

// LineWriter converts newline-delimited daemon/Caddy output into structured,
// sanitized records without allowing an unbounded partial line to accumulate.
type LineWriter struct {
	mu        sync.Mutex
	store     *Store
	component string
	level     string
	output    io.Writer
	buffer    bytes.Buffer
}

func NewLineWriter(store *Store, component, level string, outputs ...io.Writer) *LineWriter {
	writer := &LineWriter{store: store, component: component, level: normalizeLevel(level)}
	if len(outputs) > 0 {
		writer.output = outputs[0]
	}
	return writer
}

func (w *LineWriter) Write(value []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	written := len(value)
	for len(value) > 0 {
		lineEnd := bytes.IndexByte(value, '\n')
		if lineEnd < 0 {
			_, _ = w.buffer.Write(value)
			if w.buffer.Len() > 64*1024 {
				w.emit(w.buffer.String()[:64*1024])
				w.buffer.Reset()
			}
			break
		}
		_, _ = w.buffer.Write(value[:lineEnd])
		w.emit(w.buffer.String())
		w.buffer.Reset()
		value = value[lineEnd+1:]
	}
	return written, nil
}

func (w *LineWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buffer.Len() > 0 {
		w.emit(w.buffer.String())
		w.buffer.Reset()
	}
}

func (w *LineWriter) emit(line string) {
	line = strings.TrimSuffix(line, "\r")
	if line == "" || w.store == nil {
		return
	}
	record := Record{Timestamp: time.Now().UTC(), Level: w.level, Component: w.component, Message: line}
	var payload map[string]any
	parsedJSON := json.Unmarshal([]byte(line), &payload) == nil && payload != nil
	if parsedJSON {
		if message, ok := payload["msg"].(string); ok {
			record.Message = message
		} else if message, ok := payload["message"].(string); ok {
			record.Message = message
		}
		if level, ok := payload["level"].(string); ok {
			record.Level = normalizeLevel(level)
		}
		if timestamp, ok := payload["ts"].(float64); ok && timestamp > 0 {
			record.Timestamp = time.Unix(0, int64(timestamp*float64(time.Second))).UTC()
		}
		delete(payload, "msg")
		delete(payload, "message")
		delete(payload, "level")
		delete(payload, "ts")
		record.Fields = payload
	}
	w.store.Add(record)
	if w.output == nil {
		return
	}
	if !parsedJSON {
		_, _ = io.WriteString(w.output, sanitizeMessage(line)+"\n")
		return
	}
	safePayload := make(map[string]any, len(payload)+3)
	for key, value := range payload {
		if isSecretField(key) {
			safePayload[key] = RedactedValue
			continue
		}
		safePayload[key] = sanitizeValue(value)
	}
	safePayload["level"] = normalizeLevel(record.Level)
	safePayload["component"] = sanitizeMessage(record.Component)
	safePayload["msg"] = sanitizeMessage(record.Message)
	encoded, err := json.Marshal(safePayload)
	if err != nil {
		_, _ = io.WriteString(w.output, sanitizeMessage(line)+"\n")
		return
	}
	_, _ = io.WriteString(w.output, string(encoded)+"\n")
}
