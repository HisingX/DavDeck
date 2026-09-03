// Package api implements the authenticated, loopback-only Management API.
package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"davdeck.dev/davdeck/core/internal/logging"
	"davdeck.dev/davdeck/core/internal/platform"
	"davdeck.dev/davdeck/core/internal/status"
)

const statusPath = "/api/v1/status"
const daemonShutdownPath = "/api/v1/daemon/shutdown"

const maxRequestBodyBytes = 1 << 20

// Server is the local Management API server.
type Server struct {
	http          *http.Server
	token         string
	snapshot      status.Snapshot
	logger        *slog.Logger
	users         userService
	shares        shareService
	permissions   permissionService
	apply         applyService
	tls           tlsService
	dnsProviders  dnsProviderService
	endpoints     endpointService
	diagnostics   diagnosticsService
	configuration configFileService
	runtime       runtimeService
	settings      serverSettingsService
	service       platform.ServiceManager
	logs          *logging.Store
	shutdown      chan struct{}
}

// Option configures optional Management API capabilities.
type Option func(*Server)

// WithUserService enables the user-management endpoints.
func WithUserService(service userService) Option {
	return func(server *Server) { server.users = service }
}

// WithShareService enables the share-management endpoints.
func WithShareService(service shareService) Option {
	return func(server *Server) { server.shares = service }
}

// WithPermissionService enables per-share ACL endpoints.
func WithPermissionService(service permissionService) Option {
	return func(server *Server) { server.permissions = service }
}

// WithApplyService enables configuration apply, revisions, and automatic runtime
// application after ordinary user/share/ACL mutations.
func WithApplyService(service applyService) Option {
	return func(server *Server) { server.apply = service }
}

// applyAfterRuntimeMutation makes ordinary WebDAV access changes effective only
// after the daemon has validated and activated the generated Caddy config.
// TLS and YAML import intentionally use the explicit Apply endpoint instead.
func (s *Server) applyAfterRuntimeMutation(ctx context.Context) error {
	if s.apply == nil {
		return nil
	}
	_, err := s.apply.Apply(ctx)
	return err
}

// WithTLSService enables managed HTTPS configuration and preflight endpoints.
func WithTLSService(service tlsService) Option {
	return func(server *Server) { server.tls = service }
}

// WithDNSProviderService enables encrypted DNS credential management endpoints.
func WithDNSProviderService(service dnsProviderService) Option {
	return func(server *Server) { server.dnsProviders = service }
}

// WithEndpointService enables the user-facing endpoint summary.
func WithEndpointService(service endpointService) Option {
	return func(server *Server) { server.endpoints = service }
}

// WithDiagnosticsService enables sanitized diagnostic report endpoints.
func WithDiagnosticsService(service diagnosticsService) Option {
	return func(server *Server) { server.diagnostics = service }
}

// WithConfigService enables safe YAML configuration import and export.
func WithConfigService(service configFileService) Option {
	return func(server *Server) { server.configuration = service }
}

// WithRuntimeService enables managed Caddy runtime controls.
func WithRuntimeService(service runtimeService) Option {
	return func(server *Server) { server.runtime = service }
}

// WithServerSettingsService enables managed WebDAV listener settings.
func WithServerSettingsService(service serverSettingsService) Option {
	return func(server *Server) { server.settings = service }
}

// WithServiceManager enables system-service lifecycle endpoints.
func WithServiceManager(manager platform.ServiceManager) Option {
	return func(server *Server) { server.service = manager }
}

// WithLogStore enables the sanitized recent-log API.
func WithLogStore(store *logging.Store) Option {
	return func(server *Server) { server.logs = store }
}

// NewServer creates a hardened HTTP server without starting a listener.
func NewServer(address, token string, snapshot status.Snapshot, logger *slog.Logger, options ...Option) (*Server, error) {
	if token == "" {
		return nil, errors.New("management token is required")
	}
	if err := ValidateLoopbackAddress(address); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{token: token, snapshot: snapshot, logger: logger, shutdown: make(chan struct{}, 1)}
	for _, option := range options {
		option(server)
	}
	mux := http.NewServeMux()
	mux.HandleFunc(statusPath, server.handleStatus)
	mux.HandleFunc("/api/v1/logs", server.handleLogs)
	mux.HandleFunc(daemonShutdownPath, server.handleDaemonShutdown)
	if server.users != nil {
		mux.HandleFunc("/api/v1/users", server.handleUsers)
		mux.HandleFunc("/api/v1/users/", server.handleUser)
	}
	if server.shares != nil {
		mux.HandleFunc("/api/v1/shares", server.handleShares)
		mux.HandleFunc("/api/v1/shares/", server.handleShare)
	}
	if server.apply != nil {
		mux.HandleFunc("/api/v1/config/validate", server.handleConfigValidate)
		mux.HandleFunc("/api/v1/config/apply", server.handleConfigApply)
		mux.HandleFunc("/api/v1/config/state", server.handleConfigState)
		mux.HandleFunc("/api/v1/revisions", server.handleRevisions)
		mux.HandleFunc("/api/v1/revisions/", server.handleRevision)
	}
	if server.runtime != nil {
		mux.HandleFunc("/api/v1/server/status", server.handleServerStatus)
		mux.HandleFunc("/api/v1/server/start", server.handleServerStart)
		mux.HandleFunc("/api/v1/server/stop", server.handleServerStop)
		mux.HandleFunc("/api/v1/server/restart", server.handleServerRestart)
	}
	if server.endpoints != nil {
		mux.HandleFunc("/api/v1/server/endpoints", server.handleServerEndpoints)
	}
	if server.settings != nil {
		mux.HandleFunc("/api/v1/server/settings", server.handleServerSettings)
	}
	if server.service != nil {
		mux.HandleFunc("/api/v1/service/install", server.handleServiceInstall)
		mux.HandleFunc("/api/v1/service/uninstall", server.handleServiceUninstall)
		mux.HandleFunc("/api/v1/service/start", server.handleServiceStart)
		mux.HandleFunc("/api/v1/service/stop", server.handleServiceStop)
		mux.HandleFunc("/api/v1/service/status", server.handleServiceStatus)
	}
	if server.tls != nil {
		mux.HandleFunc("/api/v1/tls", server.handleTLS)
		mux.HandleFunc("/api/v1/tls/check", server.handleTLSCheck)
		mux.HandleFunc("/api/v1/tls/renew", server.handleTLSRenew)
		mux.HandleFunc("/api/v1/tls/renew/cancel", server.handleTLSRenewCancel)
	}
	if server.dnsProviders != nil {
		mux.HandleFunc("/api/v1/dns/providers", server.handleDNSProviders)
		mux.HandleFunc("/api/v1/dns/providers/", server.handleDNSProvider)
	}
	if server.diagnostics != nil {
		mux.HandleFunc("/api/v1/diagnostics", server.handleDiagnostics)
		mux.HandleFunc("/api/v1/diagnostics/run", server.handleDiagnosticsRun)
		mux.HandleFunc("/api/v1/diagnostics/report", server.handleDiagnosticsReport)
	}
	if server.configuration != nil {
		mux.HandleFunc("/api/v1/config/export", server.handleConfigExport)
		mux.HandleFunc("/api/v1/config/import", server.handleConfigImport)
	}
	mux.HandleFunc("/", server.handleNotFound)
	server.http = &http.Server{
		Addr:              address,
		Handler:           server.recoverPanic(server.authenticate(http.MaxBytesHandler(mux, maxRequestBodyBytes))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	return server, nil
}

// ValidateLoopbackAddress rejects wildcard and non-loopback listeners.
func ValidateLoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid management address: %w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("management address must use a loopback IP")
	}
	return nil
}

// Serve runs the API on an already-bound loopback listener.
func (s *Server) Serve(listener net.Listener) error { return s.http.Serve(listener) }

// Shutdown gracefully stops the API.
func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

// ShutdownRequested returns a signal emitted by the authenticated daemon
// shutdown endpoint. The daemon process owns deciding how to finish shutdown.
func (s *Server) ShutdownRequested() <-chan struct{} { return s.shutdown }

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		header := request.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) || len(header) == len(prefix) {
			writeError(writer, http.StatusUnauthorized, ErrorUnauthorized, "Authentication required", nil)
			return
		}
		provided := strings.TrimPrefix(header, prefix)
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.token)) != 1 {
			writeError(writer, http.StatusUnauthorized, ErrorUnauthorized, "Authentication required", nil)
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.logger.Error("management API panic", "method", request.Method, "path", request.URL.Path)
				writeError(writer, http.StatusInternalServerError, ErrorInternal, "Internal server error", nil)
			}
		}()
		next.ServeHTTP(writer, request)
	})
}

func (s *Server) handleNotFound(writer http.ResponseWriter, request *http.Request) {
	writeError(writer, http.StatusNotFound, ErrorNotFound, "Endpoint not found", nil)
}

func (s *Server) handleStatus(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	writeSuccess(writer, http.StatusOK, s.currentStatus(request.Context()))
}

func (s *Server) currentStatus(ctx context.Context) status.Snapshot {
	result := s.snapshot
	if result.Daemon == "" {
		// A request reaching this handler proves that the daemon and API are up.
		result.Daemon = status.DaemonRunning
	}
	if result.Caddy == "" {
		result.Caddy = string(status.StateUnknown)
	}
	if result.WebDAV == "" {
		result.WebDAV = string(status.StateUnknown)
	}
	if result.Service.State == "" {
		result.Service.State = string(status.StateNotInstalled)
	}

	if s.runtime != nil {
		runtimeStatus := s.runtime.RuntimeStatusSnapshot(ctx)
		result.Caddy = string(runtimeStatus.Caddy)
		result.WebDAV = string(runtimeStatus.WebDAV)
		result.LastErrorCode = string(runtimeStatus.LastErrorCode)
	}
	if s.service != nil {
		serviceStatus, err := s.service.Status(ctx)
		if err != nil {
			serviceErrorCode := serviceStatusErrorCode(err)
			result.Service = status.ServiceStatus{Installed: false, State: string(status.StateUnknown), LastErrorCode: serviceErrorCode}
			if result.LastErrorCode == "" && serviceErrorCode != string(platform.CodePlatformUnsupported) {
				result.LastErrorCode = result.Service.LastErrorCode
			}
		} else {
			result.Service = status.ServiceStatus{
				Installed: serviceStatus.Installed, State: string(serviceStatus.State),
				StartsAtBoot: serviceStatus.StartsAtBoot, LastErrorCode: serviceStatus.LastErrorCode,
			}
		}
	}
	if s.apply != nil {
		if state, err := s.apply.State(ctx); err == nil {
			result.DesiredRevision = state.DesiredRevision
			result.ActiveRevision = state.ActiveRevision
			result.PendingChanges = state.Pending
		} else if result.LastErrorCode == "" {
			result.LastErrorCode = "DATABASE_ERROR"
		}
	}
	return result
}

func serviceStatusErrorCode(err error) string {
	var serviceError *platform.ServiceError
	if errors.As(err, &serviceError) && serviceError.Code != "" {
		return string(serviceError.Code)
	}
	return string(platform.CodeServiceStatusFailed)
}

func writeSuccess(writer http.ResponseWriter, code int, data any) {
	writeJSON(writer, code, Envelope{Success: true, Data: data})
}

func writeError(writer http.ResponseWriter, code int, errorCode ErrorCode, message string, details map[string]any) {
	writeJSON(writer, code, Envelope{Success: false, Error: &APIError{Code: errorCode, Message: message, Details: details}})
}

func writeJSON(writer http.ResponseWriter, code int, value Envelope) {
	body, err := encodeEnvelope(value)
	if err != nil {
		code = http.StatusInternalServerError
		body = []byte(`{"success":false,"error":{"code":"INTERNAL_ERROR","message":"Internal server error"}}`)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(code)
	_, _ = writer.Write(append(body, '\n'))
}
