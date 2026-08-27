package api

import (
	"context"
	"net/http"

	"davdeck.dev/davdeck/core/internal/app"
	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
)

type runtimeService interface {
	Start(context.Context) error
	Stop(context.Context) error
	Restart(context.Context) error
	RuntimeStatus(context.Context) caddyruntime.RuntimeState
	RuntimeStatusSnapshot(context.Context) caddyruntime.RuntimeSnapshot
}

type serverStatusResponse struct {
	Caddy           string  `json:"caddy"`
	WebDAV          string  `json:"webdav"`
	LastErrorCode   string  `json:"last_error_code,omitempty"`
	DesiredRevision *uint64 `json:"desired_revision"`
	ActiveRevision  *uint64 `json:"active_revision"`
	PendingChanges  bool    `json:"pending_changes"`
}

func (s *Server) currentServerStatus(ctx context.Context) serverStatusResponse {
	runtimeStatus := s.runtime.RuntimeStatusSnapshot(ctx)
	result := serverStatusResponse{
		Caddy:         string(runtimeStatus.Caddy),
		WebDAV:        string(runtimeStatus.WebDAV),
		LastErrorCode: string(runtimeStatus.LastErrorCode),
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

func (s *Server) handleServerStatus(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	writeSuccess(writer, http.StatusOK, s.currentServerStatus(request.Context()))
}

func (s *Server) handleServerStart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	err := s.runtime.Start(request.Context())
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, s.currentServerStatus(request.Context()))
}

func (s *Server) handleServerStop(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	if err := s.runtime.Stop(request.Context()); err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, s.currentServerStatus(request.Context()))
}

func (s *Server) handleServerRestart(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	err := s.runtime.Restart(request.Context())
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, s.currentServerStatus(request.Context()))
}

var _ runtimeService = (*app.ApplyService)(nil)
