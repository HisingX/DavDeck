package api

import (
	"context"
	"net/http"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type permissionService interface {
	List(context.Context, domain.ID) ([]app.PermissionEntry, error)
	Set(context.Context, domain.ID, domain.ID, domain.Permission) (app.PermissionEntry, error)
}

func (s *Server) handlePermissions(writer http.ResponseWriter, request *http.Request, parts []string) {
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" {
		s.handleNotFound(writer, request)
		return
	}
	shareID, err := domain.ParseID(parts[0])
	if err != nil {
		writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Invalid share ID", nil)
		return
	}
	if len(parts) == 2 {
		if request.Method != http.MethodGet {
			writer.Header().Set("Allow", http.MethodGet)
			writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
			return
		}
		entries, err := s.permissions.List(request.Context(), shareID)
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, entries)
		return
	}
	if request.Method != http.MethodPut {
		writer.Header().Set("Allow", http.MethodPut)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	userID, err := domain.ParseID(parts[2])
	if err != nil {
		writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Invalid user ID", nil)
		return
	}
	var input struct {
		Permission domain.Permission `json:"permission"`
	}
	if requestError := decodeJSON(writer, request, &input); requestError != nil {
		writeError(writer, http.StatusBadRequest, requestError.Code, requestError.Message, requestError.Details)
		return
	}
	entry, err := s.permissions.Set(request.Context(), shareID, userID, input.Permission)
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	if err := s.applyAfterRuntimeMutation(request.Context()); err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, entry)
}
