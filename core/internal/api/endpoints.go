package api

import (
	"context"
	"net/http"

	"davdeck.dev/davdeck/core/internal/app"
)

type endpointService interface {
	Endpoints(context.Context) (app.EndpointSnapshot, error)
}

func (s *Server) handleServerEndpoints(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
		return
	}
	endpoints, err := s.endpoints.Endpoints(request.Context())
	if err != nil {
		writeApplicationError(writer, err)
		return
	}
	writeSuccess(writer, http.StatusOK, endpoints)
}

var _ endpointService = (*app.EndpointService)(nil)
