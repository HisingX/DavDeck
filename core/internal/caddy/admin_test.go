package caddy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminClientUsesExpectedEndpoints(t *testing.T) {
	requests := make([]string, 0, 6)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests = append(requests, request.Method+" "+request.URL.Path)
		if request.URL.Path == "/load" && request.Header.Get("Content-Type") != "application/json" {
			t.Error("reload content type missing")
		}
		if request.URL.Path == "/davdeck/tls/renew" && request.Method == http.MethodGet {
			_, _ = writer.Write([]byte(`{"state":"ISSUING","message":"issuing"}`))
			return
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	client, err := NewAdminClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := client.Reload(context.Background(), []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := client.StartCertificateRenewal(context.Background(), "dav.example.com"); err != nil {
		t.Fatal(err)
	}
	if status, err := client.CertificateRenewalStatus(context.Background(), "dav.example.com"); err != nil || status.State != "ISSUING" {
		t.Fatalf("renewal status = %#v, %v", status, err)
	}
	if err := client.CancelCertificateRenewal(context.Background(), "dav.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := client.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET /config/", "POST /load", "POST /davdeck/tls/renew", "GET /davdeck/tls/renew", "POST /davdeck/tls/renew/cancel", "POST /stop"}
	if len(requests) != len(want) {
		t.Fatalf("requests = %#v", requests)
	}
	for index := range want {
		if requests[index] != want[index] {
			t.Fatalf("requests = %#v", requests)
		}
	}
}

func TestAdminClientRejectsNonLoopbackAndErrors(t *testing.T) {
	for _, endpoint := range []string{"http://0.0.0.0:2019", "http://example.com:2019", "https://127.0.0.1:2019", "http://localhost:2019"} {
		if _, err := NewAdminClient(endpoint); err == nil {
			t.Errorf("accepted %q", endpoint)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "rejected", http.StatusBadRequest)
	}))
	defer server.Close()
	client, _ := NewAdminClient(server.URL)
	if err := client.Reload(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("admin error was ignored")
	}
}
