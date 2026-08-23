package api

import (
	"context"
	"errors"
	"net/http"

	"davdeck.dev/davdeck/core/internal/platform"
)

func (s *Server) handleServiceInstall(writer http.ResponseWriter, request *http.Request) {
	s.handleServiceMutation(writer, request, func(ctx context.Context) error {
		return s.service.Install(ctx)
	}, "installed")
}

func (s *Server) handleServiceUninstall(writer http.ResponseWriter, request *http.Request) {
	s.handleServiceMutation(writer, request, func(ctx context.Context) error {
		return s.service.Uninstall(ctx)
	}, "uninstalled")
}

func (s *Server) handleServiceStart(writer http.ResponseWriter, request *http.Request) {
	s.handleServiceMutation(writer, request, func(ctx context.Context) error {
		return s.service.Start(ctx)
	}, "started")
}

func (s *Server) handleServiceStop(writer http.ResponseWriter, request *http.Request) {
	s.handleServiceMutation(writer, request, func(ctx context.Context) error {
		return s.service.Stop(ctx)
	}, "stopped")
}

func (s *Server) handleServiceMutation(writer http.ResponseWriter, request *http.Request, operation func(context.Context) error, result string) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	if err := operation(request.Context()); err != nil {
		s.writeServiceError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, map[string]string{"result": result})
}

func (s *Server) handleServiceStatus(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	serviceStatus, err := s.service.Status(request.Context())
	if err != nil {
		s.writeServiceError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, serviceStatus)
}

func (s *Server) writeServiceError(writer http.ResponseWriter, err error) {
	var serviceError *platform.ServiceError
	if !errors.As(err, &serviceError) {
		s.logger.With("component", "platform").Error("native service operation failed", "error_code", ErrorInternal, "error", "Service operation failed")
		writeError(writer, http.StatusInternalServerError, ErrorInternal, "Internal server error", nil)
		return
	}
	s.logger.With("component", "platform").Error("native service operation failed", "error_code", string(serviceError.Code), "error", serviceError.Message)

	statusCode := http.StatusInternalServerError
	errorCode := ErrorInternal
	switch serviceError.Code {
	case platform.CodePrivilegeRequired:
		statusCode = http.StatusForbidden
		errorCode = ErrorPrivilegeRequired
	case platform.CodePlatformUnsupported:
		statusCode = http.StatusNotImplemented
		errorCode = ErrorPlatformUnsupported
	case platform.CodeServiceInstallFailed:
		errorCode = ErrorServiceInstallFailed
	case platform.CodeServiceUninstallFailed:
		errorCode = ErrorServiceUninstallFailed
	case platform.CodeServiceStartFailed:
		errorCode = ErrorServiceStartFailed
	case platform.CodeServiceStopFailed:
		errorCode = ErrorServiceStopFailed
	case platform.CodeServiceStatusFailed:
		errorCode = ErrorServiceStatusFailed
	}
	writeError(writer, statusCode, errorCode, serviceError.Message, nil)
}
