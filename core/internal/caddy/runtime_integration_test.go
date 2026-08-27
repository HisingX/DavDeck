package caddy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	cryptotls "crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"davdeck.dev/davdeck/core/internal/domain"
)

func TestPinnedCaddyRuntimeLifecycle(t *testing.T) {
	binary := os.Getenv("DAVDECK_CADDY_BINARY")
	if binary == "" {
		t.Skip("set DAVDECK_CADDY_BINARY to run pinned Caddy integration tests")
	}
	adminPort := freeTCPPort(t)
	httpPort := freeTCPPort(t)
	admin, err := NewAdminClient(fmt.Sprintf("http://127.0.0.1:%d", adminPort))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	validator := BinaryValidator{BinaryPath: binary, TempDirectory: directory}
	manager := NewRuntimeManager(binary, filepath.Join(directory, "caddy.json"), validator, admin, io.Discard, io.Discard)
	configuration := runtimeTestConfig(adminPort, httpPort)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	info, err := (ModuleInspector{BinaryPath: binary}).Inspect(ctx)
	if err != nil || !info.WebDAVModule || !info.DiscoveryModule {
		t.Fatalf("info = %#v, err = %v", info, err)
	}
	if err := validator.Validate(ctx, configuration); err != nil {
		t.Fatal(err)
	}
	if err := validator.Validate(ctx, []byte(`{"apps":`)); err == nil {
		t.Fatal("invalid configuration passed validation")
	} else {
		var runtimeError *RuntimeError
		if !errors.As(err, &runtimeError) || runtimeError.Code != CodeCaddyValidateFailed {
			t.Fatalf("validation error = %v", err)
		}
	}
	if err := manager.Start(ctx, configuration); err != nil {
		t.Fatal(err)
	}
	if state := manager.Status(ctx); state != RuntimeRunning {
		t.Fatalf("state = %s", state)
	}
	if err := manager.Reload(ctx, configuration); err != nil {
		t.Fatal(err)
	}
	if err := manager.Restart(ctx, configuration); err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	if state := manager.Status(ctx); state != RuntimeStopped {
		t.Fatalf("state = %s", state)
	}
}

func TestPinnedCaddyStartsInternalTLSEndpoint(t *testing.T) {
	binary := os.Getenv("DAVDECK_CADDY_BINARY")
	if binary == "" {
		t.Skip("set DAVDECK_CADDY_BINARY to run pinned Caddy integration tests")
	}
	adminPort, httpPort, httpsPort := freeTCPPort(t), freeTCPPort(t), freeTCPPort(t)
	input := compilerFixture(t)
	input.ServerSettings.HTTPPort, input.ServerSettings.HTTPSPort = httpPort, httpsPort
	shareDirectory := filepath.Join(t.TempDir(), "documents")
	if err := os.MkdirAll(shareDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	input.Shares[0].Share.Path = shareDirectory
	stamp := input.ServerSettings.CreatedAt
	input.TLSProfile = &domain.TLSProfile{ID: "66666666-6666-4666-8666-666666666666", Mode: domain.TLSModeInternal, Hostname: "dav.local", CreatedAt: stamp, UpdatedAt: stamp}
	compiled, err := (Compiler{}).Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	configuration := []byte(strings.Replace(string(compiled.JSON), "127.0.0.1:2019", fmt.Sprintf("127.0.0.1:%d", adminPort), 1))
	admin, err := NewAdminClient(fmt.Sprintf("http://127.0.0.1:%d", adminPort))
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	validator := BinaryValidator{BinaryPath: binary, TempDirectory: directory}
	manager := NewRuntimeManager(binary, filepath.Join(directory, "caddy-tls.json"), validator, admin, io.Discard, io.Discard)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := manager.Start(ctx, configuration); err != nil {
		t.Fatal(err)
	}
	defer manager.Stop(context.Background())
	redirectClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Timeout:       5 * time.Second,
	}
	redirectRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/not-found", httpPort), nil)
	if err != nil {
		t.Fatal(err)
	}
	redirectRequest.Host = fmt.Sprintf("dav.local:%d", httpPort)
	redirectResponse, err := redirectClient.Do(redirectRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer redirectResponse.Body.Close()
	if redirectResponse.StatusCode != http.StatusMovedPermanently && redirectResponse.StatusCode != http.StatusPermanentRedirect {
		t.Fatalf("HTTP redirect status = %d, want 301 or 308", redirectResponse.StatusCode)
	}
	wantLocation := fmt.Sprintf("https://dav.local:%d/not-found", httpsPort)
	if location := redirectResponse.Header.Get("Location"); location != wantLocation {
		t.Fatalf("HTTP redirect location = %q, want %q", location, wantLocation)
	}
	transport := &http.Transport{TLSClientConfig: &cryptotls.Config{InsecureSkipVerify: true, ServerName: "dav.local"}} // Test-only internal CA client.
	client := &http.Client{Transport: transport, Timeout: time.Second}
	tlsContext, tlsCancel := context.WithTimeout(ctx, 10*time.Second)
	defer tlsCancel()
	response, err := waitForInternalTLSResponse(tlsContext, client, httpsPort, "dav.local")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.TLS == nil || len(response.TLS.PeerCertificates) == 0 || response.StatusCode != http.StatusNotFound {
		t.Fatalf("TLS response status = %d, state = %#v", response.StatusCode, response.TLS)
	}
}

func waitForInternalTLSResponse(ctx context.Context, client *http.Client, port int, hostname string) (*http.Response, error) {
	var lastErr error
	for {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://127.0.0.1:%d/not-found", port), nil)
		if err != nil {
			return nil, err
		}
		request.Host = hostname
		response, err := client.Do(request)
		if err == nil {
			return response, nil
		}
		if response != nil {
			_ = response.Body.Close()
		}
		lastErr = err
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("internal TLS endpoint did not become ready: %w", lastErr)
		case <-timer.C:
		}
	}
}

func TestPinnedCaddyValidatesCustomTLSConfig(t *testing.T) {
	binary := os.Getenv("DAVDECK_CADDY_BINARY")
	if binary == "" {
		t.Skip("set DAVDECK_CADDY_BINARY to run pinned Caddy integration tests")
	}
	directory := t.TempDir()
	certificatePath, keyPath := writeTestCertificatePair(t, directory, "dav.example.com")
	input := compilerFixture(t)
	shareDirectory := filepath.Join(directory, "documents")
	if err := os.MkdirAll(shareDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	input.Shares[0].Share.Path = shareDirectory
	stamp := input.ServerSettings.CreatedAt
	input.TLSProfile = &domain.TLSProfile{ID: "66666666-6666-4666-8666-666666666666", Mode: domain.TLSModeCustom, Hostname: "dav.example.com", CertificatePath: certificatePath, PrivateKeyPath: keyPath, CreatedAt: stamp, UpdatedAt: stamp}
	compiled, err := (Compiler{}).Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	validator := BinaryValidator{BinaryPath: binary, TempDirectory: directory}
	if err := validator.Validate(context.Background(), compiled.JSON); err != nil {
		t.Fatal(err)
	}
}

func TestPinnedCaddyValidatesAutomaticTLSConfig(t *testing.T) {
	binary := os.Getenv("DAVDECK_CADDY_BINARY")
	if binary == "" {
		t.Skip("set DAVDECK_CADDY_BINARY to run pinned Caddy integration tests")
	}
	input := compilerFixture(t)
	shareDirectory := filepath.Join(t.TempDir(), "documents")
	if err := os.MkdirAll(shareDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	input.Shares[0].Share.Path = shareDirectory
	stamp := input.ServerSettings.CreatedAt
	input.TLSProfile = &domain.TLSProfile{ID: "66666666-6666-4666-8666-666666666666", Mode: domain.TLSModeAutomatic, Hostname: "dav.example.com", CreatedAt: stamp, UpdatedAt: stamp}
	compiled, err := (Compiler{}).Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := (BinaryValidator{BinaryPath: binary, TempDirectory: t.TempDir()}).Validate(context.Background(), compiled.JSON); err != nil {
		t.Fatal(err)
	}
}

func writeTestCertificatePair(t *testing.T, directory, hostname string) (string, string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: hostname}, DNSNames: []string{hostname}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	certificateDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificatePath, keyPath := filepath.Join(directory, "certificate.pem"), filepath.Join(directory, "private-key.pem")
	if err := os.WriteFile(certificatePath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certificatePath, keyPath
}

func runtimeTestConfig(adminPort, httpPort int) []byte {
	return []byte(fmt.Sprintf(`{"admin":{"listen":"127.0.0.1:%d"},"apps":{"http":{"servers":{"test":{"listen":["127.0.0.1:%d"],"routes":[]}}}}}`, adminPort, httpPort))
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}
