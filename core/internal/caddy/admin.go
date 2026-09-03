package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Admin interface {
	Health(context.Context) error
	Reload(context.Context, []byte) error
	Stop(context.Context) error
}

type AdminClient struct {
	endpoint string
	http     *http.Client
}

// CertificateRenewalStatus is the sanitized state of a force-renewal
// operation running inside the managed Caddy process.
type CertificateRenewalStatus struct {
	State     string `json:"state"`
	Message   string `json:"message"`
	ErrorCode string `json:"error_code,omitempty"`
}

func NewAdminClient(endpoint string) (*AdminClient, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("Caddy Admin endpoint must be loopback HTTP")
	}
	ip := net.ParseIP(parsed.Hostname())
	if ip == nil || !ip.IsLoopback() || parsed.Port() == "" {
		return nil, fmt.Errorf("Caddy Admin endpoint must be loopback HTTP")
	}
	return &AdminClient{endpoint: strings.TrimRight(endpoint, "/"), http: &http.Client{Timeout: 5 * time.Second}}, nil
}

func (c *AdminClient) Health(ctx context.Context) error {
	return c.request(ctx, http.MethodGet, "/config/", nil)
}
func (c *AdminClient) Reload(ctx context.Context, configuration []byte) error {
	return c.request(ctx, http.MethodPost, "/load", configuration)
}
func (c *AdminClient) Stop(ctx context.Context) error {
	return c.request(ctx, http.MethodPost, "/stop", nil)
}

// StartCertificateRenewal asks the in-process DavDeck Caddy module to force
// renewal through the active ACME automation policy.
func (c *AdminClient) StartCertificateRenewal(ctx context.Context, hostname string) error {
	body, err := json.Marshal(struct {
		Hostname string `json:"hostname"`
	}{Hostname: hostname})
	if err != nil {
		return err
	}
	return c.request(ctx, http.MethodPost, "/davdeck/tls/renew", body)
}

// CertificateRenewalStatus reads the current state from the in-process
// DavDeck Caddy module. The endpoint is loopback-only through Caddy Admin.
func (c *AdminClient) CertificateRenewalStatus(ctx context.Context, hostname string) (CertificateRenewalStatus, error) {
	var status CertificateRenewalStatus
	path := "/davdeck/tls/renew?hostname=" + url.QueryEscape(hostname)
	if err := c.requestJSON(ctx, http.MethodGet, path, nil, &status); err != nil {
		return CertificateRenewalStatus{}, err
	}
	return status, nil
}

// CancelCertificateRenewal cancels the force-renewal operation for hostname.
func (c *AdminClient) CancelCertificateRenewal(ctx context.Context, hostname string) error {
	body, err := json.Marshal(struct {
		Hostname string `json:"hostname"`
	}{Hostname: hostname})
	if err != nil {
		return err
	}
	return c.request(ctx, http.MethodPost, "/davdeck/tls/renew/cancel", body)
}

func (c *AdminClient) request(ctx context.Context, method, path string, body []byte) error {
	_, err := c.doRequest(ctx, method, path, body)
	return err
}

func (c *AdminClient) requestJSON(ctx context.Context, method, path string, body []byte, value any) error {
	response, err := c.doRequest(ctx, method, path, body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(response, value); err != nil {
		return fmt.Errorf("decode Caddy Admin response: %w", err)
	}
	return nil
}

func (c *AdminClient) doRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	limited, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Caddy Admin returned HTTP %d: %s", response.StatusCode, safeCommandOutput(limited))
	}
	return limited, nil
}
