package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"davdeck.dev/davdeck/core/internal/app"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiEndpointService struct {
	value app.EndpointSnapshot
}

func (s apiEndpointService) Endpoints(context.Context) (app.EndpointSnapshot, error) {
	return s.value, nil
}

func TestServerEndpointsAPI(t *testing.T) {
	service := apiEndpointService{value: app.EndpointSnapshot{
		HTTP:  app.Endpoint{Protocol: "HTTP", URL: "http://localhost:8080/dav/", Port: 8080, State: app.EndpointStateRunning, Configured: true, Active: true, Copyable: true},
		HTTPS: app.Endpoint{Protocol: "HTTPS", Port: 8443, State: app.EndpointStateNotConfigured, Configured: false},
	}}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithEndpointService(service))
	if err != nil {
		t.Fatal(err)
	}
	response := apiRequest(t, server, http.MethodGet, "/api/v1/server/endpoints", "")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"url":"http://localhost:8080/dav/"`) || !strings.Contains(response.Body.String(), `"state":"NOT_CONFIGURED"`) {
		t.Fatalf("response = %d: %s", response.Code, response.Body.String())
	}
}
