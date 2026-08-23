//go:build windows

package platform

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

type windowsServiceManager struct{ config ServiceConfig }

func NewServiceManager(config ServiceConfig) (ServiceManager, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	return &windowsServiceManager{config: config}, nil
}

func (m *windowsServiceManager) Install(context.Context) error {
	manager, err := mgr.Connect()
	if err != nil {
		return windowsServiceError(CodeServiceInstallFailed, "Windows Service Control Manager could not be opened", err)
	}
	defer manager.Disconnect()
	service, err := manager.CreateService(serviceName, m.config.Executable, mgr.Config{DisplayName: serviceDisplayName, Description: m.config.Description, StartType: mgr.StartAutomatic}, m.config.Arguments...)
	if err != nil {
		return windowsServiceError(CodeServiceInstallFailed, "Windows could not install the DavDeck service", err)
	}
	return service.Close()
}

func (m *windowsServiceManager) Uninstall(ctx context.Context) error {
	manager, err := mgr.Connect()
	if err != nil {
		return windowsServiceError(CodeServiceUninstallFailed, "Windows Service Control Manager could not be opened", err)
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(serviceName)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return nil
	}
	if err != nil {
		return windowsServiceError(CodeServiceUninstallFailed, "Windows could not open the DavDeck service", err)
	}
	defer service.Close()
	status, queryErr := service.Query()
	if queryErr != nil {
		return windowsServiceError(CodeServiceUninstallFailed, "Windows could not query the DavDeck service", queryErr)
	}
	if status.State != svc.Stopped {
		if _, err := service.Control(svc.Stop); err != nil {
			return windowsServiceError(CodeServiceUninstallFailed, "Windows could not stop the DavDeck service for uninstall", err)
		}
		if err := waitWindowsService(ctx, service, svc.Stopped); err != nil {
			return windowsServiceError(CodeServiceUninstallFailed, "Windows did not stop the DavDeck service for uninstall", err)
		}
	}
	if err := service.Delete(); err != nil {
		return windowsServiceError(CodeServiceUninstallFailed, "Windows could not uninstall the DavDeck service", err)
	}
	return nil
}

func (m *windowsServiceManager) Start(context.Context) error {
	return m.withService(CodeServiceStartFailed, "Windows could not start the DavDeck service", func(service *mgr.Service) error { return service.Start() })
}

func (m *windowsServiceManager) Stop(context.Context) error {
	return m.withService(CodeServiceStopFailed, "Windows could not stop the DavDeck service", func(service *mgr.Service) error {
		_, err := service.Control(svc.Stop)
		return err
	})
}

func (m *windowsServiceManager) Status(context.Context) (ServiceStatus, error) {
	manager, err := mgr.Connect()
	if err != nil {
		return ServiceStatus{}, windowsServiceError(CodeServiceStatusFailed, "Windows Service Control Manager could not be opened", err)
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(serviceName)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		return ServiceStatus{State: ServiceStateNotInstalled}, nil
	}
	if err != nil {
		return ServiceStatus{}, windowsServiceError(CodeServiceStatusFailed, "Windows could not open the DavDeck service", err)
	}
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return ServiceStatus{}, windowsServiceError(CodeServiceStatusFailed, "Windows could not query the DavDeck service", err)
	}
	return ServiceStatus{Installed: true, State: windowsServiceState(status.State), StartsAtBoot: true}, nil
}

func (m *windowsServiceManager) withService(code ServiceErrorCode, message string, operation func(*mgr.Service) error) error {
	manager, err := mgr.Connect()
	if err != nil {
		return windowsServiceError(code, message, err)
	}
	defer manager.Disconnect()
	service, err := manager.OpenService(serviceName)
	if err != nil {
		return windowsServiceError(code, message, err)
	}
	defer service.Close()
	if err := operation(service); err != nil {
		return windowsServiceError(code, message, err)
	}
	return nil
}

func waitWindowsService(ctx context.Context, service *mgr.Service, target svc.State) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		status, err := service.Query()
		if err != nil {
			return err
		}
		if status.State == target {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func windowsServiceState(state svc.State) ServiceState {
	switch state {
	case svc.Stopped, svc.Paused:
		return ServiceStateStopped
	case svc.StartPending, svc.ContinuePending:
		return ServiceStateStarting
	case svc.Running:
		return ServiceStateRunning
	case svc.StopPending, svc.PausePending:
		return ServiceStateStopping
	default:
		return ServiceStateUnknown
	}
}

func windowsServiceError(code ServiceErrorCode, message string, err error) error {
	if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		return &ServiceError{Code: CodePrivilegeRequired, Message: "Administrator privileges are required to manage the Windows service", Cause: err}
	}
	return &ServiceError{Code: code, Message: message, Cause: err}
}
