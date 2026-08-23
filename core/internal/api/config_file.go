package api

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"

	"davdeck.dev/davdeck/core/internal/app"
)

type configFileService interface {
	Export(context.Context) ([]byte, error)
	Import(context.Context, []byte) (app.ConfigImportResult, error)
}

func (s *Server) handleConfigExport(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	body, err := s.configuration.Export(request.Context())
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, map[string]any{"format": "yaml", "content": string(body), "contains_secrets": false})
}

func (s *Server) handleConfigImport(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		writer.Header().Set("Allow", http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || (mediaType != "application/yaml" && mediaType != "text/yaml" && mediaType != "application/x-yaml") {
		writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Content-Type must be application/yaml", nil)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(writer, http.StatusRequestEntityTooLarge, ErrorInvalidRequest, "Configuration is too large", nil)
			return
		}
		writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Configuration could not be read", nil)
		return
	}
	result, err := s.configuration.Import(request.Context(), body)
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, result)
}
