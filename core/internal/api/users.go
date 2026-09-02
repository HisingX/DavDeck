package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type userService interface {
	List(context.Context) ([]domain.User, error)
	Get(context.Context, domain.ID) (domain.User, error)
	Create(context.Context, string, string) (domain.User, error)
	Delete(context.Context, domain.ID) error
	SetEnabled(context.Context, domain.ID, bool) error
	ChangePassword(context.Context, domain.ID, string) error
}

type userResponse struct {
	ID        domain.ID        `json:"id"`
	Username  string           `json:"username"`
	Enabled   bool             `json:"enabled"`
	CreatedAt domain.Timestamp `json:"created_at"`
	UpdatedAt domain.Timestamp `json:"updated_at"`
}

func publicUser(user domain.User) userResponse {
	return userResponse{ID: user.ID, Username: user.Username, Enabled: user.Enabled, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}

func (s *Server) handleUsers(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		users, err := s.users.List(request.Context())
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		result := make([]userResponse, 0, len(users))
		for _, user := range users {
			result = append(result, publicUser(user))
		}
		writeSuccess(writer, http.StatusOK, result)
	case http.MethodPost:
		var input struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if requestError := decodeJSON(writer, request, &input); requestError != nil {
			writeError(writer, http.StatusBadRequest, requestError.Code, requestError.Message, requestError.Details)
			return
		}
		user, err := s.users.Create(request.Context(), input.Username, input.Password)
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		if err := s.applyAfterRuntimeMutation(request.Context()); err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusCreated, publicUser(user))
	default:
		writer.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
	}
}

func (s *Server) handleUser(writer http.ResponseWriter, request *http.Request) {
	remainder := strings.TrimPrefix(request.URL.Path, "/api/v1/users/")
	parts := strings.Split(remainder, "/")
	if len(parts) == 0 || parts[0] == "" || len(parts) > 2 || (len(parts) == 2 && parts[1] != "password") {
		s.handleNotFound(writer, request)
		return
	}
	id, err := domain.ParseID(parts[0])
	if err != nil {
		writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Invalid user ID", nil)
		return
	}
	if len(parts) == 2 {
		s.handleUserPassword(writer, request, id)
		return
	}
	switch request.Method {
	case http.MethodGet:
		user, err := s.users.Get(request.Context(), id)
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, publicUser(user))
	case http.MethodPatch:
		var input struct {
			Enabled *bool `json:"enabled"`
		}
		if requestError := decodeJSON(writer, request, &input); requestError != nil {
			writeError(writer, http.StatusBadRequest, requestError.Code, requestError.Message, requestError.Details)
			return
		}
		if input.Enabled == nil {
			writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Invalid request body", nil)
			return
		}
		if err := s.users.SetEnabled(request.Context(), id, *input.Enabled); err != nil {
			writeApplicationError(writer, err)
			return
		}
		if err := s.applyAfterRuntimeMutation(request.Context()); err != nil {
			writeApplicationError(writer, err)
			return
		}
		user, err := s.users.Get(request.Context(), id)
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, publicUser(user))
	case http.MethodDelete:
		if err := s.users.Delete(request.Context(), id); err != nil {
			writeApplicationError(writer, err)
			return
		}
		if err := s.applyAfterRuntimeMutation(request.Context()); err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, map[string]any{"id": id, "deleted": true})
	default:
		writer.Header().Set("Allow", http.MethodGet+", "+http.MethodPatch+", "+http.MethodDelete)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
	}
}

func (s *Server) handleUserPassword(writer http.ResponseWriter, request *http.Request, id domain.ID) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	if requestError := decodeJSON(writer, request, &input); requestError != nil {
		writeError(writer, http.StatusBadRequest, requestError.Code, requestError.Message, requestError.Details)
		return
	}
	if err := s.users.ChangePassword(request.Context(), id, input.Password); err != nil {
		writeApplicationError(writer, err)
		return
	}
	if err := s.applyAfterRuntimeMutation(request.Context()); err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, map[string]any{"id": id, "password_changed": true})
}

func writeApplicationError(writer http.ResponseWriter, err error) {
	var applicationError *app.Error
	if !errors.As(err, &applicationError) {
		writeError(writer, http.StatusInternalServerError, ErrorInternal, "Internal server error", nil)
		return
	}
	statusCode := http.StatusBadRequest
	switch applicationError.Code {
	case app.CodeUserNotFound:
		statusCode = http.StatusNotFound
	case app.CodeUserAlreadyExists:
		statusCode = http.StatusConflict
	case app.CodeShareNotFound:
		statusCode = http.StatusNotFound
	case app.CodeShareAlreadyExists:
		statusCode = http.StatusConflict
	case app.CodeDNSProviderNotFound:
		statusCode = http.StatusNotFound
	case app.CodeDNSProviderAlreadyExists, app.CodeDNSProviderInUse:
		statusCode = http.StatusConflict
	case app.CodeDatabase:
		statusCode = http.StatusInternalServerError
	case app.CodeRevisionNotFound:
		statusCode = http.StatusNotFound
	case app.CodeRevisionStateUnavailable:
		statusCode = http.StatusConflict
	case app.CodeRevisionActive, app.CodeRevisionDesired:
		statusCode = http.StatusConflict
	case app.CodeApplyInProgress:
		statusCode = http.StatusConflict
	case app.CodeCaddyValidateFailed:
		statusCode = http.StatusUnprocessableEntity
	case app.CodeCaddyApplyFailed, app.CodeCaddyStartFailed, app.CodeCaddyStopFailed, app.CodeCaddyNotFound, app.CodeCaddyModuleMissing, app.CodeRuntimeUnhealthy:
		statusCode = http.StatusBadGateway
	case app.CodeDNSCheckFailed:
		statusCode = http.StatusUnprocessableEntity
	case app.CodeDNSProviderSecretMissing, app.CodeDNSProviderZoneNotAllowed:
		statusCode = http.StatusUnprocessableEntity
	case app.CodeConfigVersionUnsupported:
		statusCode = http.StatusUnprocessableEntity
	}
	writeError(writer, statusCode, ErrorCode(applicationError.Code), applicationError.Message, nil)
}
