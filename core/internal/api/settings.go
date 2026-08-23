package api

import (
	"context"
	"net/http"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type serverSettingsService interface {
	Get(context.Context) (domain.ServerSettings, error)
	UpdatePorts(context.Context, int, int) (domain.ServerSettings, error)
}

type serverSettingsResponse struct {
	HTTPPort  int `json:"http_port"`
	HTTPSPort int `json:"https_port"`
}

func newServerSettingsResponse(settings domain.ServerSettings) serverSettingsResponse {
	return serverSettingsResponse{HTTPPort: settings.HTTPPort, HTTPSPort: settings.HTTPSPort}
}

func (s *Server) handleServerSettings(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		settings, err := s.settings.Get(request.Context())
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, newServerSettingsResponse(settings))
	case http.MethodPut:
		var input struct {
			HTTPPort  int `json:"http_port"`
			HTTPSPort int `json:"https_port"`
		}
		if requestError := decodeJSON(writer, request, &input); requestError != nil {
			writeError(writer, http.StatusBadRequest, requestError.Code, requestError.Message, requestError.Details)
			return
		}
		settings, err := s.settings.UpdatePorts(request.Context(), input.HTTPPort, input.HTTPSPort)
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		if err := s.applyAfterRuntimeMutation(request.Context()); err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, newServerSettingsResponse(settings))
	default:
		writer.Header().Set("Allow", http.MethodGet+", "+http.MethodPut)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
	}
}

var _ serverSettingsService = (*app.ServerSettingsService)(nil)
