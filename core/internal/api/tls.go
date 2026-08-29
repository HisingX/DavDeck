package api

import (
	"context"
	"net/http"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type tlsService interface {
	Get(context.Context) (domain.TLSProfile, bool, error)
	Update(context.Context, app.TLSUpdate) (domain.TLSProfile, error)
	Disable(context.Context) error
	Check(context.Context) (app.TLSCheckResult, error)
}

func (s *Server) handleTLS(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		profile, found, err := s.tls.Get(request.Context())
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		if !found {
			writeSuccess(writer, http.StatusOK, nil)
			return
		}
		writeSuccess(writer, http.StatusOK, profile)
	case http.MethodPut:
		var input struct {
			Mode            domain.TLSMode `json:"mode"`
			Hostname        string         `json:"hostname"`
			CertificatePath string         `json:"certificate_path"`
			PrivateKeyPath  string         `json:"private_key_path"`
		}
		if requestError := decodeJSON(writer, request, &input); requestError != nil {
			writeError(writer, http.StatusBadRequest, requestError.Code, requestError.Message, requestError.Details)
			return
		}
		profile, err := s.tls.Update(request.Context(), app.TLSUpdate{Mode: input.Mode, Hostname: input.Hostname, CertificatePath: input.CertificatePath, PrivateKeyPath: input.PrivateKeyPath})
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, profile)
	case http.MethodDelete:
		if err := s.tls.Disable(request.Context()); err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, nil)
	default:
		writer.Header().Set("Allow", http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
	}
}

func (s *Server) handleTLSCheck(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	result, err := s.tls.Check(request.Context())
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, result)
}
