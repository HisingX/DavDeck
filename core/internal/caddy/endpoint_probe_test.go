package caddy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestLocalEndpointProbeChecksHTTPHostAndPath(t *testing.T) {
	var receivedHost, receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedHost = request.Host
		receivedPath = request.URL.Path
		writer.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	probeServer(t, server, "http", "dav.local", "/dav/")
	if receivedHost != "dav.local" || receivedPath != "/dav/" {
		t.Fatalf("request host/path = %q %q", receivedHost, receivedPath)
	}
}

func TestLocalEndpointProbePerformsTLSHandshakeWithSNI(t *testing.T) {
	var receivedServerName string
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedServerName = request.TLS.ServerName
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	probeServer(t, server, "https", "dav.local", "/dav/")
	if receivedServerName != "dav.local" {
		t.Fatalf("TLS server name = %q, want dav.local", receivedServerName)
	}
}

func probeServer(t *testing.T, server *httptest.Server, scheme, hostname, path string) {
	t.Helper()
	parsed, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}
	if err := (LocalEndpointProbe{}).Probe(context.Background(), scheme, port, hostname, path); err != nil {
		t.Fatalf("probe failed: %v", err)
	}
}
