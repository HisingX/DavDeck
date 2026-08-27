package platform

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"davdeck.dev/davdeck/core/internal/status"
)

const (
	serviceName        = "davdeck"
	serviceDisplayName = "DavDeck WebDAV Server Manager"
)

func ServiceName() string { return serviceName }

type ServiceState = status.State

const (
	ServiceStateNotInstalled = status.StateNotInstalled
	ServiceStateStopped      = status.StateStopped
	ServiceStateStarting     = status.StateStarting
	ServiceStateRunning      = status.StateRunning
	ServiceStateStopping     = status.StateStopping
	ServiceStateFailed       = status.StateFailed
	ServiceStateUnknown      = status.StateUnknown
)

type ServiceStatus struct {
	Installed     bool         `json:"installed"`
	State         ServiceState `json:"state"`
	StartsAtBoot  bool         `json:"starts_at_boot"`
	LastErrorCode string       `json:"last_error_code,omitempty"`
}

type ServiceConfig struct {
	Executable  string
	Arguments   []string
	Description string
	User        string
}

func (c ServiceConfig) Validate() error {
	if !filepath.IsAbs(c.Executable) {
		return errors.New("service executable must be an absolute path")
	}
	for _, value := range append([]string{c.Executable, c.Description, c.User}, c.Arguments...) {
		if strings.IndexFunc(value, func(character rune) bool { return character < 0x20 || character == 0x7f }) >= 0 {
			return errors.New("service configuration contains control characters")
		}
	}
	return nil
}

type ServiceManager interface {
	Install(context.Context) error
	Uninstall(context.Context) error
	Start(context.Context) error
	Stop(context.Context) error
	Status(context.Context) (ServiceStatus, error)
}

func newUnsupportedServiceManager(config ServiceConfig) (ServiceManager, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return unsupportedServiceManager{}, nil
}

type unsupportedServiceManager struct{}

func (unsupportedServiceManager) Install(context.Context) error {
	return unsupportedServiceError()
}

func (unsupportedServiceManager) Uninstall(context.Context) error {
	return unsupportedServiceError()
}

func (unsupportedServiceManager) Start(context.Context) error {
	return unsupportedServiceError()
}

func (unsupportedServiceManager) Stop(context.Context) error {
	return unsupportedServiceError()
}

func (unsupportedServiceManager) Status(context.Context) (ServiceStatus, error) {
	return ServiceStatus{State: ServiceStateUnknown}, unsupportedServiceError()
}

func unsupportedServiceError() error {
	return &ServiceError{
		Code:    CodePlatformUnsupported,
		Message: "Native system service management is currently supported only on Linux",
	}
}

type ServiceErrorCode string

const (
	CodeServiceInstallFailed   ServiceErrorCode = "SERVICE_INSTALL_FAILED"
	CodeServiceUninstallFailed ServiceErrorCode = "SERVICE_UNINSTALL_FAILED"
	CodeServiceStartFailed     ServiceErrorCode = "SERVICE_START_FAILED"
	CodeServiceStopFailed      ServiceErrorCode = "SERVICE_STOP_FAILED"
	CodeServiceStatusFailed    ServiceErrorCode = "SERVICE_STATUS_FAILED"
	CodePrivilegeRequired      ServiceErrorCode = "PRIVILEGE_REQUIRED"
	CodePlatformUnsupported    ServiceErrorCode = "PLATFORM_UNSUPPORTED"
)

type ServiceError struct {
	Code    ServiceErrorCode
	Message string
	Cause   error
}

func (e *ServiceError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
}

func (e *ServiceError) Unwrap() error { return e.Cause }
