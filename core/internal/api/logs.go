package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"davdeck.dev/davdeck/core/internal/logging"
)

const (
	defaultLogPageSize = logging.DefaultPageSize
	maximumLogPageSize = logging.MaximumPageSize
)

func (s *Server) handleLogs(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	if s.logs == nil {
		writeError(writer, http.StatusServiceUnavailable, ErrorLogsUnavailable, "Recent logs are unavailable", nil)
		return
	}
	query, err := parseLogQuery(request)
	if err != nil {
		writeError(writer, http.StatusBadRequest, ErrorInvalidLogQuery, err.Error(), nil)
		return
	}
	writeSuccess(writer, http.StatusOK, s.logs.Query(query))
}

func parseLogQuery(request *http.Request) (logging.Query, error) {
	values := request.URL.Query()
	query := logging.Query{Limit: defaultLogPageSize}
	if value := strings.TrimSpace(values.Get("limit")); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > maximumLogPageSize {
			return logging.Query{}, errors.New("limit must be between 1 and 200")
		}
		query.Limit = limit
	}
	if value := strings.TrimSpace(values.Get("cursor")); value != "" {
		cursor, err := strconv.ParseUint(value, 10, 64)
		if err != nil || cursor == 0 {
			return logging.Query{}, errors.New("cursor must be a positive integer")
		}
		query.Cursor = cursor
	}
	if value := strings.TrimSpace(values.Get("since")); value != "" {
		since, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return logging.Query{}, errors.New("since must be an RFC3339 timestamp")
		}
		query.Since = &since
	}
	if value := strings.TrimSpace(values.Get("level")); value != "" {
		query.Level = strings.ToUpper(value)
		switch query.Level {
		case "DEBUG", "INFO", "WARN", "ERROR":
		default:
			return logging.Query{}, errors.New("level must be DEBUG, INFO, WARN, or ERROR")
		}
	}
	if value := strings.TrimSpace(values.Get("component")); value != "" {
		if len(value) > 64 || strings.IndexFunc(value, func(character rune) bool { return character < 0x20 || character == 0x7f }) >= 0 {
			return logging.Query{}, errors.New("component is invalid")
		}
		query.Component = value
	}
	return query, nil
}
