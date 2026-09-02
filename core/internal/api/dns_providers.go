package api

import (
	"context"
	"net/http"
	"strings"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/domain"
)

type dnsProviderService interface {
	List(context.Context) ([]app.DNSProviderCredentialView, error)
	Get(context.Context, domain.ID) (app.DNSProviderCredentialView, error)
	Save(context.Context, app.DNSProviderUpdate) (app.DNSProviderCredentialView, error)
	Delete(context.Context, domain.ID) error
}

func (s *Server) handleDNSProviders(writer http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		providers, err := s.dnsProviders.List(request.Context())
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, providers)
	case http.MethodPost:
		var input struct {
			Name         string                   `json:"name"`
			Provider     domain.DNSProviderType   `json:"provider"`
			AllowedZones []string                 `json:"allowed_zones"`
			Secret       domain.DNSProviderSecret `json:"secret"`
		}
		if requestError := decodeJSON(writer, request, &input); requestError != nil {
			writeError(writer, http.StatusBadRequest, requestError.Code, requestError.Message, requestError.Details)
			return
		}
		provider, err := s.dnsProviders.Save(request.Context(), app.DNSProviderUpdate{Name: input.Name, Provider: input.Provider, AllowedZones: input.AllowedZones, Secret: input.Secret})
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusCreated, provider)
	default:
		writer.Header().Set("Allow", http.MethodGet+", "+http.MethodPost)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
	}
}

func (s *Server) handleDNSProvider(writer http.ResponseWriter, request *http.Request) {
	remainder := strings.TrimPrefix(request.URL.Path, "/api/v1/dns/providers/")
	parts := strings.Split(remainder, "/")
	if len(parts) != 1 || parts[0] == "" {
		s.handleNotFound(writer, request)
		return
	}
	id, err := domain.ParseID(parts[0])
	if err != nil {
		writeError(writer, http.StatusBadRequest, ErrorInvalidRequest, "Invalid DNS provider ID", nil)
		return
	}
	switch request.Method {
	case http.MethodGet:
		provider, err := s.dnsProviders.Get(request.Context(), id)
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, provider)
	case http.MethodPut:
		var input struct {
			Name         string                   `json:"name"`
			Provider     domain.DNSProviderType   `json:"provider"`
			AllowedZones []string                 `json:"allowed_zones"`
			Secret       domain.DNSProviderSecret `json:"secret"`
		}
		if requestError := decodeJSON(writer, request, &input); requestError != nil {
			writeError(writer, http.StatusBadRequest, requestError.Code, requestError.Message, requestError.Details)
			return
		}
		provider, err := s.dnsProviders.Save(request.Context(), app.DNSProviderUpdate{ID: id, Name: input.Name, Provider: input.Provider, AllowedZones: input.AllowedZones, Secret: input.Secret})
		if err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, provider)
	case http.MethodDelete:
		if err := s.dnsProviders.Delete(request.Context(), id); err != nil {
			writeApplicationError(writer, err)
			return
		}
		writeSuccess(writer, http.StatusOK, map[string]any{"id": id, "deleted": true})
	default:
		writer.Header().Set("Allow", http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
		writeError(writer, http.StatusMethodNotAllowed, ErrorMethodNotAllowed, "Method not allowed", nil)
	}
}
