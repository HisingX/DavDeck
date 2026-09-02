// Package app implements DavDeck application use cases over small interfaces.
package app

import (
	"errors"
	"fmt"
)

// ErrorCode is a stable application error code mapped by API and CLI clients.
type ErrorCode string

const (
	CodeUserNotFound              ErrorCode = "USER_NOT_FOUND"
	CodeUserAlreadyExists         ErrorCode = "USER_ALREADY_EXISTS"
	CodeInvalidUsername           ErrorCode = "INVALID_USERNAME"
	CodeInvalidPassword           ErrorCode = "INVALID_PASSWORD"
	CodeDatabase                  ErrorCode = "DATABASE_ERROR"
	CodeShareNotFound             ErrorCode = "SHARE_NOT_FOUND"
	CodeShareAlreadyExists        ErrorCode = "SHARE_ALREADY_EXISTS"
	CodeInvalidShareName          ErrorCode = "INVALID_SHARE_NAME"
	CodeInvalidShareSlug          ErrorCode = "INVALID_SHARE_SLUG"
	CodeInvalidSharePath          ErrorCode = "INVALID_SHARE_PATH"
	CodeSharePathNotFound         ErrorCode = "SHARE_PATH_NOT_FOUND"
	CodeSharePathUnreadable       ErrorCode = "SHARE_PATH_NOT_READABLE"
	CodeSharePathUnwritable       ErrorCode = "SHARE_PATH_NOT_WRITABLE"
	CodeInvalidPermission         ErrorCode = "INVALID_PERMISSION"
	CodePermissionNotFound        ErrorCode = "PERMISSION_NOT_FOUND"
	CodeRevisionNotFound          ErrorCode = "REVISION_NOT_FOUND"
	CodeRevisionStateUnavailable  ErrorCode = "REVISION_STATE_UNAVAILABLE"
	CodeRevisionActive            ErrorCode = "REVISION_ACTIVE"
	CodeRevisionDesired           ErrorCode = "REVISION_DESIRED"
	CodeApplyInProgress           ErrorCode = "CONFIG_APPLY_IN_PROGRESS"
	CodeCaddyValidateFailed       ErrorCode = "CADDY_VALIDATE_FAILED"
	CodeCaddyApplyFailed          ErrorCode = "CADDY_RELOAD_FAILED"
	CodeCaddyStartFailed          ErrorCode = "CADDY_START_FAILED"
	CodeCaddyStopFailed           ErrorCode = "CADDY_STOP_FAILED"
	CodeCaddyNotFound             ErrorCode = "CADDY_NOT_FOUND"
	CodeCaddyModuleMissing        ErrorCode = "CADDY_MODULE_MISSING"
	CodeRuntimeUnhealthy          ErrorCode = "RUNTIME_UNHEALTHY"
	CodeTLSConfiguration          ErrorCode = "TLS_CONFIGURATION_ERROR"
	CodeTLSCertificate            ErrorCode = "TLS_CERTIFICATE_NOT_FOUND"
	CodeTLSPrivateKey             ErrorCode = "TLS_PRIVATE_KEY_NOT_FOUND"
	CodeDNSCheckFailed            ErrorCode = "DNS_CHECK_FAILED"
	CodeDNSProviderNotFound       ErrorCode = "DNS_PROVIDER_NOT_FOUND"
	CodeDNSProviderAlreadyExists  ErrorCode = "DNS_PROVIDER_ALREADY_EXISTS"
	CodeDNSProviderInUse          ErrorCode = "DNS_PROVIDER_IN_USE"
	CodeInvalidDNSProvider        ErrorCode = "INVALID_DNS_PROVIDER"
	CodeInvalidDNSProviderSecret  ErrorCode = "INVALID_DNS_PROVIDER_SECRET"
	CodeDNSProviderSecretMissing  ErrorCode = "DNS_PROVIDER_SECRET_MISSING"
	CodeDNSProviderZoneNotAllowed ErrorCode = "DNS_PROVIDER_ZONE_NOT_ALLOWED"
	CodeConfigImportInvalid       ErrorCode = "CONFIG_IMPORT_INVALID"
	CodeConfigVersionUnsupported  ErrorCode = "CONFIG_VERSION_UNSUPPORTED"
	CodeConfigExportFailed        ErrorCode = "CONFIG_EXPORT_FAILED"
	CodeInvalidServerPorts        ErrorCode = "INVALID_SERVER_PORTS"
	CodeServerPortUnavailable     ErrorCode = "SERVER_PORT_UNAVAILABLE"
	CodeServerSettingsNotFound    ErrorCode = "SERVER_SETTINGS_NOT_FOUND"
)

var (
	ErrRevisionNotFound = errors.New("revision not found")
	ErrRevisionActive   = errors.New("revision is active")
	ErrRevisionDesired  = errors.New("revision is desired")
)

// Error contains a client-safe message and an internal cause.
type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}

func (e *Error) Unwrap() error { return e.Cause }
