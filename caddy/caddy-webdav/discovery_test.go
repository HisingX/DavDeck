package webdav

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func TestDiscoveryPropfindReturnsOnlyAuthenticatedUserEntries(t *testing.T) {
	discovery := Discovery{
		BasePath: "/dav",
		Entries: map[string][]DiscoveryEntry{
			"alice": {
				{Slug: "documents", Name: "Documents"},
				{Slug: "photos", Name: "R&D <家庭照片>"},
			},
			"bob": {{Slug: "documents", Name: "Documents"}},
		},
	}

	request := httptest.NewRequest("PROPFIND", "http://example.test/dav/", nil)
	request.Header.Set("Depth", "1")
	request = withDiscoveryUser(request, "alice")
	recorder := httptest.NewRecorder()
	if err := discovery.ServeHTTP(recorder, request, nil); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMultiStatus)
	}

	var response struct {
		Responses []struct {
			Href string `xml:"href"`
		} `xml:"response"`
	}
	if err := xml.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid PROPFIND response: %v\n%s", err, recorder.Body.String())
	}
	if len(response.Responses) != 3 {
		t.Fatalf("response count = %d, want 3", len(response.Responses))
	}
	body := recorder.Body.String()
	for _, value := range []string{"/dav/", "/dav/documents/", "/dav/photos/", "R&amp;D &lt;家庭照片&gt;"} {
		if !strings.Contains(body, value) {
			t.Errorf("response does not contain %q: %s", value, body)
		}
	}
	if strings.Contains(body, "/srv/") {
		t.Fatalf("response leaked a physical path: %s", body)
	}
}

func TestDiscoverySupportsReadOnlyMethodsAndRejectsMutations(t *testing.T) {
	discovery := Discovery{BasePath: "/dav", Entries: map[string][]DiscoveryEntry{
		"alice": {{Slug: "documents", Name: "Documents"}},
	}}
	for _, testCase := range []struct {
		method string
		status int
	}{
		{http.MethodOptions, http.StatusOK},
		{http.MethodGet, http.StatusOK},
		{http.MethodHead, http.StatusOK},
		{"PUT", http.StatusMethodNotAllowed},
		{"MKCOL", http.StatusMethodNotAllowed},
		{http.MethodDelete, http.StatusMethodNotAllowed},
	} {
		t.Run(testCase.method, func(t *testing.T) {
			request := withDiscoveryUser(httptest.NewRequest(testCase.method, "http://example.test/dav/", nil), "alice")
			recorder := httptest.NewRecorder()
			err := discovery.ServeHTTP(recorder, request, nil)
			if testCase.status == http.StatusMethodNotAllowed {
				var handlerError caddyhttp.HandlerError
				if !errors.As(err, &handlerError) || handlerError.StatusCode != testCase.status {
					t.Fatalf("error = %v, want HTTP %d", err, testCase.status)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if recorder.Code != testCase.status {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.status)
			}
		})
	}
}

func TestDiscoveryRejectsUnsupportedDepth(t *testing.T) {
	discovery := Discovery{BasePath: "/dav", Entries: map[string][]DiscoveryEntry{
		"alice": {{Slug: "documents", Name: "Documents"}},
	}}
	request := httptest.NewRequest("PROPFIND", "http://example.test/dav/", nil)
	request.Header.Set("Depth", "infinity")
	request = withDiscoveryUser(request, "alice")
	recorder := httptest.NewRecorder()
	err := discovery.ServeHTTP(recorder, request, nil)
	if err == nil {
		t.Fatal("unsupported depth was accepted")
	}
	var handlerError caddyhttp.HandlerError
	if !errors.As(err, &handlerError) || handlerError.StatusCode != http.StatusForbidden {
		t.Fatalf("error = %v, want HTTP %d", err, http.StatusForbidden)
	}
}

func TestDiscoveryRejectsMissingAuthenticationContext(t *testing.T) {
	discovery := Discovery{BasePath: "/dav", Entries: map[string][]DiscoveryEntry{
		"alice": {{Slug: "documents", Name: "Documents"}},
	}}
	recorder := httptest.NewRecorder()
	err := discovery.ServeHTTP(recorder, httptest.NewRequest("PROPFIND", "http://example.test/dav/", nil), nil)
	if err == nil {
		t.Fatal("missing authentication context was accepted")
	}
	var handlerError caddyhttp.HandlerError
	if !errors.As(err, &handlerError) || handlerError.StatusCode != http.StatusForbidden {
		t.Fatalf("error = %v, want HTTP %d", err, http.StatusForbidden)
	}
}

func withDiscoveryUser(request *http.Request, username string) *http.Request {
	replacer := caddy.NewReplacer()
	replacer.Set("http.auth.user.id", username)
	return request.WithContext(context.WithValue(request.Context(), caddy.ReplacerCtxKey, replacer))
}
