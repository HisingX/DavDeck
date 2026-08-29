package api

import (
	"context"
	"net/http"
	"strings"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type applyService interface {
	Apply(context.Context) (domain.ConfigRevision, error)
	Validate(context.Context) (app.ValidationResult, error)
	Restore(context.Context, domain.ID) (domain.ConfigRevision, error)
	State(context.Context) (app.RevisionState, error)
	List(context.Context) ([]domain.ConfigRevision, error)
	Get(context.Context, domain.ID) (domain.ConfigRevision, error)
	Delete(context.Context, domain.ID) error
}

type validationResponse struct {
	Valid      bool     `json:"valid"`
	ConfigHash string   `json:"config_hash"`
	Warnings   []string `json:"warnings,omitempty"`
}

type revisionResponse struct {
	ID                     domain.ID                       `json:"id"`
	Number                 uint64                          `json:"number"`
	CreatedAt              domain.Timestamp                `json:"created_at"`
	ConfigHash             string                          `json:"config_hash"`
	ValidationStatus       domain.RevisionValidationStatus `json:"validation_status"`
	ApplyStatus            domain.RevisionApplyStatus      `json:"apply_status"`
	StateSnapshotAvailable bool                            `json:"state_snapshot_available"`
	AppVersion             string                          `json:"app_version"`
	ErrorCode              string                          `json:"error_code,omitempty"`
	ErrorSummary           string                          `json:"error_summary,omitempty"`
}

func publicRevision(value domain.ConfigRevision) revisionResponse {
	return revisionResponse{ID: value.ID, Number: value.Number, CreatedAt: value.CreatedAt, ConfigHash: value.ConfigHash, ValidationStatus: value.ValidationStatus, ApplyStatus: value.ApplyStatus, StateSnapshotAvailable: len(value.StateSnapshotJSON) > 0, AppVersion: value.AppVersion, ErrorCode: value.ErrorCode, ErrorSummary: value.ErrorSummary}
}

func (s *Server) handleConfigApply(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	revision, err := s.apply.Apply(request.Context())
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, publicRevision(revision))
}

func (s *Server) handleConfigValidate(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	result, err := s.apply.Validate(request.Context())
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, validationResponse{Valid: result.Valid, ConfigHash: result.ConfigHash, Warnings: result.Warnings})
}

func (s *Server) handleConfigState(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	state, err := s.apply.State(request.Context())
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, state)
}

func (s *Server) handleRevisions(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	values, err := s.apply.List(request.Context())
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	result := make([]revisionResponse, 0, len(values))
	for _, value := range values {
		result = append(result, publicRevision(value))
	}
	writeSuccess(writer, http.StatusOK, result)
}

func (s *Server) handleRevision(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodPost && request.Method != http.MethodDelete {
		writer.Header().Set("Allow", http.MethodGet+", "+http.MethodPost+", "+http.MethodDelete)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	value := strings.TrimPrefix(request.URL.Path, "/api/v1/revisions/")
	if request.Method == http.MethodDelete {
		if value == "" || strings.Contains(value, "/") {
			s.handleNotFound(writer, request)
			return
		}
		id, err := domain.ParseID(value)
		if err != nil {
			writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Invalid revision ID", nil)
			return
		}
		if err := s.apply.Delete(request.Context(), id); err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, map[string]any{"id": id, "deleted": true})
		return
	}
	if request.Method == http.MethodPost {
		if !strings.HasSuffix(request.URL.Path, "/restore") {
			s.handleNotFound(writer, request)
			return
		}
		value = strings.TrimSuffix(value, "/restore")
		if value == "" || strings.Contains(value, "/") {
			s.handleNotFound(writer, request)
			return
		}
		id, err := domain.ParseID(value)
		if err != nil {
			writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Invalid revision ID", nil)
			return
		}
		revision, err := s.apply.Restore(request.Context(), id)
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, publicRevision(revision))
		return
	}
	if value == "" || strings.Contains(value, "/") {
		s.handleNotFound(writer, request)
		return
	}
	id, err := domain.ParseID(value)
	if err != nil {
		writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Invalid revision ID", nil)
		return
	}
	revision, err := s.apply.Get(request.Context(), id)
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, publicRevision(revision))
}
