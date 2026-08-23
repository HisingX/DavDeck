package caddy

import "fmt"

type RuntimeErrorCode string

const (
	CodeCaddyNotFound       RuntimeErrorCode = "CADDY_NOT_FOUND"
	CodeCaddyModuleMissing  RuntimeErrorCode = "CADDY_MODULE_MISSING"
	CodeCaddyValidateFailed RuntimeErrorCode = "CADDY_VALIDATE_FAILED"
	CodeCaddyStartFailed    RuntimeErrorCode = "CADDY_START_FAILED"
	CodeCaddyStopFailed     RuntimeErrorCode = "CADDY_STOP_FAILED"
	CodeCaddyReloadFailed   RuntimeErrorCode = "CADDY_RELOAD_FAILED"
	CodeRuntimeUnhealthy    RuntimeErrorCode = "RUNTIME_UNHEALTHY"
)

type RuntimeError struct {
	Code    RuntimeErrorCode
	Message string
	Cause   error
}

func (e *RuntimeError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}
func (e *RuntimeError) Unwrap() error { return e.Cause }
