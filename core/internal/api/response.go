package api

import "encoding/json"

// ErrorCode is a stable machine-readable Management API error code.
type ErrorCode string

const (
	ErrorUnauthorized           ErrorCode = "UNAUTHORIZED"
	ErrorNotFound               ErrorCode = "NOT_FOUND"
	ErrorMethodNotAllowed       ErrorCode = "METHOD_NOT_ALLOWED"
	ErrorInvalidRequest         ErrorCode = "INVALID_REQUEST"
	ErrorInternal               ErrorCode = "INTERNAL_ERROR"
	ErrorPrivilegeRequired      ErrorCode = "PRIVILEGE_REQUIRED"
	ErrorPlatformUnsupported    ErrorCode = "PLATFORM_UNSUPPORTED"
	ErrorServiceInstallFailed   ErrorCode = "SERVICE_INSTALL_FAILED"
	ErrorServiceUninstallFailed ErrorCode = "SERVICE_UNINSTALL_FAILED"
	ErrorServiceStartFailed     ErrorCode = "SERVICE_START_FAILED"
	ErrorServiceStopFailed      ErrorCode = "SERVICE_STOP_FAILED"
	ErrorServiceStatusFailed    ErrorCode = "SERVICE_STATUS_FAILED"
	ErrorLogsUnavailable        ErrorCode = "LOGS_UNAVAILABLE"
	ErrorInvalidLogQuery        ErrorCode = "INVALID_LOG_QUERY"
)

// APIError is safe to return to local management clients.
type APIError struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Envelope is the common response shape for every Management API endpoint.
type Envelope struct {
	Success bool      `json:"success"`
	Data    any       `json:"data"`
	Error   *APIError `json:"error,omitempty"`
}

func encodeEnvelope(value Envelope) ([]byte, error) {
	return json.Marshal(value)
}
