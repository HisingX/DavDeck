package caddy

import (
	"context"
	"net"
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

func TestLocalEndpointProbeRetriesTransientFailure(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts++
		if attempts == 1 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	probeServer(t, server, "http", "dav.local", "/dav/")
	if attempts < 2 {
		t.Fatalf("probe attempts = %d, want retry", attempts)
	}
}

func TestProbeHostUsesConfiguredIPAndLoopbackForNames(t *testing.T) {
	if got := probeHost("192.168.201.108"); got != "192.168.201.108" {
		t.Fatalf("probe host for IPv4 = %q", got)
	}
	if got := probeHost("dav.local"); got != "127.0.0.1" {
		t.Fatalf("probe host for hostname = %q", got)
	}
	if got := probeHost("::1"); got != "::1" {
		t.Fatalf("probe host for IPv6 = %q", got)
	}
	if got := net.JoinHostPort(probeHost("192.168.201.108"), "18443"); got != "192.168.201.108:18443" {
		t.Fatalf("probe address = %q", got)
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
