package api

import (
	"context"
	"net/http"
	"strings"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type shareService interface {
	List(context.Context) ([]domain.Share, error)
	Get(context.Context, domain.ID) (domain.Share, error)
	Create(context.Context, string, string, string) (domain.Share, error)
	Update(context.Context, domain.ID, app.ShareUpdate) (domain.Share, error)
	Delete(context.Context, domain.ID) error
}

type shareResponse struct {
	ID        domain.ID        `json:"id"`
	Name      string           `json:"name"`
	Slug      string           `json:"slug"`
	Path      string           `json:"path"`
	Enabled   bool             `json:"enabled"`
	CreatedAt domain.Timestamp `json:"created_at"`
	UpdatedAt domain.Timestamp `json:"updated_at"`
}

func publicShare(share domain.Share) shareResponse {
	return shareResponse{ID: share.ID, Name: share.Name, Slug: share.Slug, Path: share.Path, Enabled: share.Enabled, CreatedAt: share.CreatedAt, UpdatedAt: share.UpdatedAt}
}

func (s *Server) handleShares(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		shares, err := s.shares.List(request.Context())
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		result := make([]shareResponse, 0, len(shares))
		for _, share := range shares {
			result = append(result, publicShare(share))
		}
		writeSuccess(writer, http.StatusOK, result)
	case http.MethodPost:
		var input struct {
			Name string `json:"name"`
			Slug string `json:"slug"`
			Path string `json:"path"`
		}
		if requestError := decodeJSON(writer, request, &input); requestError != nil {
			writeError(writer, http.StatusBadRequest, requestError.Code, requestError.Message, requestError.Details)
			return
		}
		share, err := s.shares.Create(request.Context(), input.Name, input.Slug, input.Path)
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		if err := s.applyAfterRuntimeMutation(request.Context()); err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusCreated, publicShare(share))
	default:
		writer.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
	}
}

func (s *Server) handleShare(writer http.ResponseWriter, request *http.Request) {
	remainder := strings.TrimPrefix(request.URL.Path, "/api/v1/shares/")
	parts := strings.Split(remainder, "/")
	if len(parts) >= 2 && parts[1] == "permissions" && s.permissions != nil {
		s.handlePermissions(writer, request, parts)
		return
	}
	if remainder == "" || len(parts) != 1 {
		s.handleNotFound(writer, request)
		return
	}
	id, err := domain.ParseID(remainder)
	if err != nil {
		writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Invalid share ID", nil)
		return
	}
	switch request.Method {
	case http.MethodGet:
		share, err := s.shares.Get(request.Context(), id)
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, publicShare(share))
	case http.MethodPatch:
		var input struct {
			Name    *string `json:"name"`
			Slug    *string `json:"slug"`
			Path    *string `json:"path"`
			Enabled *bool   `json:"enabled"`
		}
		if requestError := decodeJSON(writer, request, &input); requestError != nil {
			writeError(writer, http.StatusBadRequest, requestError.Code, requestError.Message, requestError.Details)
			return
		}
		if input.Name == nil && input.Slug == nil && input.Path == nil && input.Enabled == nil {
			writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "At least one share field is required", nil)
			return
		}
		share, err := s.shares.Update(request.Context(), id, app.ShareUpdate{Name: input.Name, Slug: input.Slug, Path: input.Path, Enabled: input.Enabled})
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		if err := s.applyAfterRuntimeMutation(request.Context()); err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, publicShare(share))
	case http.MethodDelete:
		if err := s.shares.Delete(request.Context(), id); err != nil {
			writeApplicationError(writer, err)
			return
		}
		if err := s.applyAfterRuntimeMutation(request.Context()); err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, map[string]any{"id": id, "deleted": true, "files_preserved": true})
	default:
		writer.Header().Set("Allow", http.MethodGet+", "+http.MethodPatch+", "+http.MethodDelete)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
	}
}
