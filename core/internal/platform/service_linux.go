//go:build linux

package platform

import (
	"context"
	"os"
	"strings"
)

const systemdPath = "/etc/systemd/system/davdeck.service"

type systemdServiceManager struct {
	config         ServiceConfig
	definitionPath string
	runner         serviceCommandRunner
	privileged     func() bool
}

func NewServiceManager(config ServiceConfig) (ServiceManager, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &systemdServiceManager{config: config, definitionPath: systemdPath, runner: execServiceCommandRunner{}, privileged: func() bool { return os.Geteuid() == 0 }}, nil
}

func (m *systemdServiceManager) Install(ctx context.Context) error {
	if err := m.requirePrivilege(); err != nil {
		return err
	}
	body, err := renderSystemdDefinition(m.config)
	if err != nil {
		return &ServiceError{Code: CodeServiceInstallFailed, Message: "Systemd service definition is invalid", Cause: err}
	}
	if err := installServiceFile(m.definitionPath, body); err != nil {
		return &ServiceError{Code: CodeServiceInstallFailed, Message: "Systemd service definition could not be installed", Cause: err}
	}
	if _, err := m.runner.Run(ctx, "systemctl", "daemon-reload"); err != nil {
		_ = os.Remove(m.definitionPath)
		return &ServiceError{Code: CodeServiceInstallFailed, Message: "Systemd could not reload service definitions", Cause: err}
	}
	if _, err := m.runner.Run(ctx, "systemctl", "enable", serviceName+".service"); err != nil {
		_ = os.Remove(m.definitionPath)
		_, _ = m.runner.Run(ctx, "systemctl", "daemon-reload")
		return &ServiceError{Code: CodeServiceInstallFailed, Message: "Systemd could not enable DavDeck", Cause: err}
	}
	return nil
}

func (m *systemdServiceManager) Uninstall(ctx context.Context) error {
	if err := m.requirePrivilege(); err != nil {
		return err
	}
	if _, err := os.Lstat(m.definitionPath); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return &ServiceError{Code: CodeServiceUninstallFailed, Message: "Systemd service definition could not be inspected", Cause: err}
	}
	if _, err := m.runner.Run(ctx, "systemctl", "disable", "--now", serviceName+".service"); err != nil {
		return &ServiceError{Code: CodeServiceUninstallFailed, Message: "Systemd could not disable DavDeck", Cause: err}
	}
	if err := os.Remove(m.definitionPath); err != nil {
		return &ServiceError{Code: CodeServiceUninstallFailed, Message: "Systemd service definition could not be removed", Cause: err}
	}
	if _, err := m.runner.Run(ctx, "systemctl", "daemon-reload"); err != nil {
		return &ServiceError{Code: CodeServiceUninstallFailed, Message: "Systemd could not reload service definitions", Cause: err}
	}
	return nil
}

func (m *systemdServiceManager) Start(ctx context.Context) error {
	return m.runPrivileged(ctx, CodeServiceStartFailed, "Systemd could not start DavDeck", "start", serviceName+".service")
}

func (m *systemdServiceManager) Stop(ctx context.Context) error {
	return m.runPrivileged(ctx, CodeServiceStopFailed, "Systemd could not stop DavDeck", "stop", serviceName+".service")
}

func (m *systemdServiceManager) Status(ctx context.Context) (ServiceStatus, error) {
	if _, err := os.Lstat(m.definitionPath); os.IsNotExist(err) {
		return ServiceStatus{State: ServiceStateNotInstalled}, nil
	} else if err != nil {
		return ServiceStatus{}, &ServiceError{Code: CodeServiceStatusFailed, Message: "Systemd service definition could not be inspected", Cause: err}
	}
	output, err := m.runner.Run(ctx, "systemctl", "show", serviceName+".service", "--property=LoadState", "--property=ActiveState", "--property=UnitFileState", "--value")
	if err != nil {
		return ServiceStatus{}, &ServiceError{Code: CodeServiceStatusFailed, Message: "Systemd could not query DavDeck", Cause: err}
	}
	fields := strings.Fields(strings.ToLower(string(output)))
	if len(fields) < 2 || fields[0] == "not-found" {
		return ServiceStatus{State: ServiceStateNotInstalled}, nil
	}
	state := ServiceStateUnknown
	switch fields[1] {
	case "active":
		state = ServiceStateRunning
	case "activating", "reloading":
		state = ServiceStateStarting
	case "deactivating":
		state = ServiceStateStopping
	case "inactive":
		state = ServiceStateStopped
	case "failed":
		state = ServiceStateFailed
	}
	startsAtBoot := len(fields) >= 3 && isEnabledUnitFileState(fields[2])
	return ServiceStatus{Installed: true, State: state, StartsAtBoot: startsAtBoot}, nil
}

func isEnabledUnitFileState(value string) bool {
	switch value {
	case "enabled", "enabled-runtime", "static", "indirect", "generated", "alias":
		return true
	default:
		return false
	}
}

func (m *systemdServiceManager) runPrivileged(ctx context.Context, code ServiceErrorCode, message string, arguments ...string) error {
	if err := m.requirePrivilege(); err != nil {
		return err
	}
	if _, err := m.runner.Run(ctx, "systemctl", arguments...); err != nil {
		return &ServiceError{Code: code, Message: message, Cause: err}
	}
	return nil
}

func (m *systemdServiceManager) requirePrivilege() error {
	if m.privileged != nil && !m.privileged() {
		return &ServiceError{Code: CodePrivilegeRequired, Message: "Administrator privileges are required to manage the system service"}
	}
	return nil
}
