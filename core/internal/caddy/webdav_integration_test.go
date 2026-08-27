package caddy

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

func TestPinnedWebDAVAuthenticationAndACLMatrix(t *testing.T) {
	binary := os.Getenv("DAVDECK_CADDY_BINARY")
	if binary == "" {
		t.Skip("set DAVDECK_CADDY_BINARY to run pinned WebDAV integration tests")
	}
	root := t.TempDir()
	runtimeDirectory := t.TempDir()
	httpPort := freeTCPPort(t)
	adminPort := freeTCPPort(t)
	input := webDAVFixture(t, root, httpPort)
	compiled, err := (Compiler{}).Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	configuration := []byte(strings.Replace(string(compiled.JSON), "127.0.0.1:2019", fmt.Sprintf("127.0.0.1:%d", adminPort), 1))
	admin, err := NewAdminClient(fmt.Sprintf("http://127.0.0.1:%d", adminPort))
	if err != nil {
		t.Fatal(err)
	}
	manager := NewRuntimeManager(binary, filepath.Join(runtimeDirectory, "caddy.json"), BinaryValidator{BinaryPath: binary, TempDirectory: runtimeDirectory}, admin, io.Discard, io.Discard)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := manager.Start(ctx, configuration); err != nil {
		t.Fatal(err)
	}
	defer func() {
		stopContext, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		if err := manager.Stop(stopContext); err != nil {
			t.Errorf("stop Caddy: %v", err)
		}
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d/dav/documents", httpPort)
	rootURL := fmt.Sprintf("http://127.0.0.1:%d/dav", httpPort)
	assertDiscoveryEntries(t, rootURL, "alice", "alice password", "1", []string{"/dav/", "/dav/documents/", "/dav/photos/"})
	assertDiscoveryEntries(t, rootURL, "bob", "bob password", "1", []string{"/dav/", "/dav/documents/"})
	assertDiscoveryEntries(t, rootURL, "alice", "alice password", "0", []string{"/dav/"})
	assertDAVStatus(t, "PROPFIND", rootURL, "charlie", "charlie password", nil, map[string]string{"Depth": "1"}, http.StatusUnauthorized)
	assertDAVStatus(t, http.MethodPut, rootURL+"/", "alice", "alice password", []byte("blocked"), nil, http.StatusMethodNotAllowed)
	assertDAVStatus(t, http.MethodPut, baseURL+"/hello.txt", "alice", "alice password", []byte("hello davdeck"), nil, http.StatusCreated)
	assertDAVStatus(t, http.MethodGet, baseURL+"/hello.txt", "alice", "alice password", nil, nil, http.StatusOK)
	unicodeName := "space 文件.txt"
	unicodeURL := baseURL + "/" + url.PathEscape(unicodeName)
	assertDAVStatus(t, http.MethodPut, unicodeURL, "alice", "alice password", []byte("unicode and spaces"), nil, http.StatusCreated)
	unicodeResponse := davRequest(t, http.MethodGet, unicodeURL, "alice", "alice password", nil, nil)
	unicodeBody, _ := io.ReadAll(unicodeResponse.Body)
	unicodeResponse.Body.Close()
	if unicodeResponse.StatusCode != http.StatusOK || string(unicodeBody) != "unicode and spaces" {
		t.Fatalf("unicode GET status=%d body=%q", unicodeResponse.StatusCode, unicodeBody)
	}
	assertDAVStatus(t, http.MethodHead, baseURL+"/hello.txt", "bob", "bob password", nil, nil, http.StatusOK)
	assertDAVStatus(t, http.MethodOptions, baseURL+"/", "bob", "bob password", nil, nil, http.StatusOK)
	assertDAVStatus(t, "PROPFIND", baseURL+"/", "bob", "bob password", nil, map[string]string{"Depth": "1"}, http.StatusMultiStatus)
	response := davRequest(t, http.MethodGet, baseURL+"/hello.txt", "bob", "bob password", nil, nil)
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(body) != "hello davdeck" {
		t.Fatalf("GET body = %q", body)
	}

	for _, mutation := range []struct {
		method, path string
		headers      map[string]string
	}{
		{http.MethodPut, "/blocked.txt", nil}, {"MKCOL", "/blocked", nil}, {http.MethodDelete, "/hello.txt", nil},
		{"COPY", "/hello.txt", map[string]string{"Destination": baseURL + "/blocked-copy.txt"}},
		{"MOVE", "/hello.txt", map[string]string{"Destination": baseURL + "/blocked-move.txt"}},
	} {
		assertDAVStatus(t, mutation.method, baseURL+mutation.path, "bob", "bob password", []byte("blocked"), mutation.headers, http.StatusUnauthorized)
	}

	assertDAVStatus(t, http.MethodGet, baseURL+"/hello.txt", "charlie", "charlie password", nil, nil, http.StatusUnauthorized)
	assertDAVStatus(t, http.MethodGet, baseURL+"/hello.txt", "dave", "dave password", nil, nil, http.StatusUnauthorized)
	assertDAVStatus(t, http.MethodGet, baseURL+"/hello.txt", "", "", nil, nil, http.StatusUnauthorized)

	outsideSecret := []byte("DAVDECK_OUTSIDE_SHARE_SECRET")
	parentOutside, err := os.CreateTemp(filepath.Dir(root), "davdeck-outside-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	parentOutsidePath := parentOutside.Name()
	defer os.Remove(parentOutsidePath)
	if _, err := parentOutside.Write(outsideSecret); err != nil {
		parentOutside.Close()
		t.Fatal(err)
	}
	if err := parentOutside.Close(); err != nil {
		t.Fatal(err)
	}
	parentOutsideName := filepath.Base(parentOutsidePath)
	outsideDirectory := t.TempDir()
	outsideFile := filepath.Join(outsideDirectory, "outside-secret.txt")
	if err := os.WriteFile(outsideFile, outsideSecret, 0o600); err != nil {
		t.Fatal(err)
	}
	for name, requestURL := range map[string]string{
		"raw parent":            baseURL + "/../" + parentOutsideName,
		"encoded parent":        baseURL + "/%2e%2e/" + parentOutsideName,
		"double encoded parent": baseURL + "/%252e%252e/" + parentOutsideName,
		"mixed separator":       baseURL + "/%5c..%5c" + parentOutsideName,
		"encoded absolute":      baseURL + "/%2Fetc%2Fpasswd",
	} {
		t.Run("traversal_"+name, func(t *testing.T) {
			assertDAVDoesNotExpose(t, requestURL, outsideSecret)
		})
	}

	symlinkPath := filepath.Join(root, "documents", "escape-link")
	if err := os.Symlink(outsideDirectory, symlinkPath); err != nil {
		t.Logf("symlink probe unavailable: %v", err)
	} else {
		response := davRequest(t, http.MethodGet, baseURL+"/escape-link/outside-secret.txt", "alice", "alice password", nil, nil)
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		exposed := response.StatusCode == http.StatusOK && bytes.Equal(body, outsideSecret)
		if exposed {
			t.Fatal("WebDAV runtime followed a symlink outside the configured share")
		}
	}

	assertDAVStatus(t, "MKCOL", baseURL+"/folder", "alice", "alice password", nil, nil, http.StatusCreated)
	assertDAVStatus(t, "COPY", baseURL+"/hello.txt", "alice", "alice password", nil, map[string]string{"Destination": baseURL + "/copy.txt"}, http.StatusCreated)
	assertDAVStatus(t, "MOVE", baseURL+"/copy.txt", "alice", "alice password", nil, map[string]string{"Destination": baseURL + "/moved.txt"}, http.StatusCreated)
	assertDAVStatus(t, http.MethodDelete, baseURL+"/moved.txt", "alice", "alice password", nil, nil, http.StatusNoContent)
	if _, err := os.Stat(filepath.Join(root, "documents", "hello.txt")); err != nil {
		t.Fatalf("source file was unexpectedly removed: %v", err)
	}
}

func assertDAVDoesNotExpose(t *testing.T, requestURL string, forbidden []byte) {
	t.Helper()
	response := davRequest(t, http.MethodGet, requestURL, "alice", "alice password", nil, nil)
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode >= 500 || bytes.Contains(body, forbidden) {
		t.Fatalf("traversal request status=%d exposed forbidden content=%t: %q", response.StatusCode, bytes.Contains(body, forbidden), body)
	}
}

func webDAVFixture(t *testing.T, root string, httpPort int) RuntimeConfigInput {
	t.Helper()
	documentsRoot := filepath.Join(root, "documents")
	photosRoot := filepath.Join(root, "photos")
	if err := os.MkdirAll(documentsRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(photosRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	newUser := func(id, name, password string, enabled bool) domain.User {
		hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			t.Fatal(err)
		}
		hash := string(hashBytes)
		return domain.User{ID: domain.ID(id), Username: name, UsernameNormalized: domain.NormalizeUsername(name), PasswordHash: hash, Enabled: enabled, CreatedAt: stamp, UpdatedAt: stamp}
	}
	alice := newUser("11111111-1111-4111-8111-111111111111", "alice", "alice password", true)
	bob := newUser("22222222-2222-4222-8222-222222222222", "bob", "bob password", true)
	charlie := newUser("33333333-3333-4333-8333-333333333333", "charlie", "charlie password", true)
	dave := newUser("44444444-4444-4444-8444-444444444444", "dave", "dave password", false)
	documents := domain.Share{ID: "55555555-5555-4555-8555-555555555555", Name: "Documents", Slug: "documents", Path: documentsRoot, Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	photos := domain.Share{ID: "66666666-6666-4666-8666-666666666666", Name: "Photos", Slug: "photos", Path: photosRoot, Enabled: true, CreatedAt: stamp, UpdatedAt: stamp}
	permission := func(share domain.Share, user domain.User, value domain.Permission) domain.SharePermission {
		return domain.SharePermission{ShareID: share.ID, UserID: user.ID, Permission: value, CreatedAt: stamp, UpdatedAt: stamp}
	}
	settings := domain.ServerSettings{ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", PublicBasePath: "/dav", HTTPPort: httpPort, HTTPSPort: freeTCPPort(t), RuntimeMode: domain.RuntimeModePortable, CreatedAt: stamp, UpdatedAt: stamp}
	return RuntimeConfigInput{ServerSettings: settings, Users: []domain.User{alice, bob, charlie, dave}, Shares: []ShareWithPermissions{
		{Share: documents, Permissions: []domain.SharePermission{permission(documents, alice, domain.PermissionReadWrite), permission(documents, bob, domain.PermissionRead), permission(documents, charlie, domain.PermissionNone), permission(documents, dave, domain.PermissionReadWrite)}},
		{Share: photos, Permissions: []domain.SharePermission{permission(photos, alice, domain.PermissionRead), permission(photos, bob, domain.PermissionNone)}},
	}}
}

func assertDiscoveryEntries(t *testing.T, requestURL, username, password, depth string, expected []string) {
	t.Helper()
	response := davRequest(t, "PROPFIND", requestURL, username, password, nil, map[string]string{"Depth": depth})
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusMultiStatus {
		t.Fatalf("PROPFIND %s as %q status = %d, want %d: %s", requestURL, username, response.StatusCode, http.StatusMultiStatus, body)
	}
	var document struct {
		Responses []struct {
			Href string `xml:"href"`
		} `xml:"response"`
	}
	if err := xml.Unmarshal(body, &document); err != nil {
		t.Fatalf("invalid discovery response: %v\n%s", err, body)
	}
	if len(document.Responses) != len(expected) {
		t.Fatalf("discovery response count = %d, want %d: %s", len(document.Responses), len(expected), body)
	}
	for index, want := range expected {
		if document.Responses[index].Href != want {
			t.Fatalf("discovery response[%d] = %q, want %q: %s", index, document.Responses[index].Href, want, body)
		}
	}
}

func assertDAVStatus(t *testing.T, method, url, username, password string, body []byte, headers map[string]string, expected int) {
	t.Helper()
	response := davRequest(t, method, url, username, password, body, headers)
	defer response.Body.Close()
	if response.StatusCode != expected {
		responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
		t.Fatalf("%s %s as %q status = %d, want %d: %s", method, url, username, response.StatusCode, expected, responseBody)
	}
}

func davRequest(t *testing.T, method, url, username, password string, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if username != "" {
		request.SetBasicAuth(username, password)
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response, err := (&http.Client{Timeout: 5 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
