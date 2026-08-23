// Package domain contains DavDeck's pure business entities, value objects, and
// validation rules. It has no persistence, HTTP, operating-system, or Caddy
// dependencies.
package domain

import "fmt"

// ErrorCode is a stable machine-readable domain validation code.
type ErrorCode string

const (
	CodeInvalidID             ErrorCode = "INVALID_ID"
	CodeInvalidTimestamp      ErrorCode = "INVALID_TIMESTAMP"
	CodeInvalidUsername       ErrorCode = "INVALID_USERNAME"
	CodeInvalidPasswordHash   ErrorCode = "INVALID_PASSWORD_HASH"
	CodeInvalidShareName      ErrorCode = "INVALID_SHARE_NAME"
	CodeInvalidShareSlug      ErrorCode = "INVALID_SHARE_SLUG"
	CodeInvalidSharePath      ErrorCode = "INVALID_SHARE_PATH"
	CodeInvalidPermission     ErrorCode = "INVALID_PERMISSION"
	CodeInvalidTLSMode        ErrorCode = "INVALID_TLS_MODE"
	CodeInvalidHostname       ErrorCode = "INVALID_HOSTNAME"
	CodeInvalidCertificate    ErrorCode = "INVALID_CERTIFICATE_PATH"
	CodeInvalidPrivateKey     ErrorCode = "INVALID_PRIVATE_KEY_PATH"
	CodeInvalidRuntimeMode    ErrorCode = "INVALID_RUNTIME_MODE"
	CodeInvalidBasePath       ErrorCode = "INVALID_PUBLIC_BASE_PATH"
	CodeInvalidPort           ErrorCode = "INVALID_PORT"
	CodeInvalidRevision       ErrorCode = "INVALID_REVISION"
	CodeInvalidConfig         ErrorCode = "INVALID_CONFIG_JSON"
	CodeInvalidConfigHash     ErrorCode = "INVALID_CONFIG_HASH"
	CodeInvalidRevisionStatus ErrorCode = "INVALID_REVISION_STATUS"
)

// ValidationError identifies one invalid field without exposing secrets.
type ValidationError struct {
	Code    ErrorCode
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validator is implemented by every independently validatable domain entity.
type Validator interface {
	Validate() error
}

func invalid(code ErrorCode, field, message string) error {
	return &ValidationError{Code: code, Field: field, Message: message}
}
