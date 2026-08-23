//go:build darwin

package platform

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const (
	launchdLabel = "dev.davdeck.davd"
	launchdPath  = "/Library/LaunchDaemons/dev.davdeck.davd.plist"
)

type launchdServiceManager struct {
	config         ServiceConfig
	definitionPath string
	runner         serviceCommandRunner
	privileged     func() bool
}

func NewServiceManager(config ServiceConfig) (ServiceManager, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &launchdServiceManager{config: config, definitionPath: launchdPath, runner: execServiceCommandRunner{}, privileged: func() bool { return os.Geteuid() == 0 }}, nil
}

func (m *launchdServiceManager) Install(ctx context.Context) error {
	if err := m.requirePrivilege(); err != nil {
		return err
	}
	body, err := renderLaunchdDefinition(m.config)
	if err != nil {
		return &ServiceError{Code: CodeServiceInstallFailed, Message: "Launchd service definition is invalid", Cause: err}
	}
	if err := installServiceFile(m.definitionPath, body); err != nil {
		return &ServiceError{Code: CodeServiceInstallFailed, Message: "Launchd service definition could not be installed", Cause: err}
	}
	if _, err := m.runner.Run(ctx, "launchctl", "bootstrap", "system", m.definitionPath); err != nil {
		_ = os.Remove(m.definitionPath)
		return &ServiceError{Code: CodeServiceInstallFailed, Message: "Launchd could not bootstrap DavDeck", Cause: err}
	}
	return nil
}

func (m *launchdServiceManager) Uninstall(ctx context.Context) error {
	if err := m.requirePrivilege(); err != nil {
		return err
	}
	if _, err := os.Lstat(m.definitionPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return &ServiceError{Code: CodeServiceUninstallFailed, Message: "Launchd service definition could not be inspected", Cause: err}
	}
	if _, err := m.runner.Run(ctx, "launchctl", "bootout", "system/"+launchdLabel); err != nil {
		return &ServiceError{Code: CodeServiceUninstallFailed, Message: "Launchd could not stop DavDeck for uninstall", Cause: err}
	}
	if err := os.Remove(m.definitionPath); err != nil {
		return &ServiceError{Code: CodeServiceUninstallFailed, Message: "Launchd service definition could not be removed", Cause: err}
	}
	return nil
}

func (m *launchdServiceManager) Start(ctx context.Context) error {
	if err := m.requirePrivilege(); err != nil {
		return err
	}
	if _, err := m.runner.Run(ctx, "launchctl", "kickstart", "system/"+launchdLabel); err != nil {
		return &ServiceError{Code: CodeServiceStartFailed, Message: "Launchd could not start DavDeck", Cause: err}
	}
	return nil
}

func (m *launchdServiceManager) Stop(ctx context.Context) error {
	if err := m.requirePrivilege(); err != nil {
		return err
	}
	if _, err := m.runner.Run(ctx, "launchctl", "kill", "SIGTERM", "system/"+launchdLabel); err != nil {
		return &ServiceError{Code: CodeServiceStopFailed, Message: "Launchd could not stop DavDeck", Cause: err}
	}
	return nil
}

func (m *launchdServiceManager) Status(ctx context.Context) (ServiceStatus, error) {
	if _, err := os.Lstat(m.definitionPath); os.IsNotExist(err) {
		return ServiceStatus{State: ServiceStateNotInstalled}, nil
	} else if err != nil {
		return ServiceStatus{}, &ServiceError{Code: CodeServiceStatusFailed, Message: "Launchd service definition could not be inspected", Cause: err}
	}
	output, err := m.runner.Run(ctx, "launchctl", "print", "system/"+launchdLabel)
	if err != nil {
		if strings.Contains(strings.ToLower(string(output)), "could not find service") {
			return ServiceStatus{Installed: true, State: ServiceStateStopped}, nil
		}
		return ServiceStatus{}, &ServiceError{Code: CodeServiceStatusFailed, Message: "Launchd could not query DavDeck", Cause: err}
	}
	state := ServiceStateUnknown
	text := strings.ToLower(string(output))
	switch {
	case strings.Contains(text, "state = running"):
		state = ServiceStateRunning
	case strings.Contains(text, "state = waiting"), strings.Contains(text, "state = exited"):
		state = ServiceStateStopped
	}
	return ServiceStatus{Installed: true, State: state, StartsAtBoot: true}, nil
}

func (m *launchdServiceManager) requirePrivilege() error {
	if m.privileged != nil && !m.privileged() {
		return &ServiceError{Code: CodePrivilegeRequired, Message: "Administrator privileges are required to manage the launch daemon"}
	}
	return nil
}

func (m *launchdServiceManager) String() string { return fmt.Sprintf("launchd:%s", launchdLabel) }
