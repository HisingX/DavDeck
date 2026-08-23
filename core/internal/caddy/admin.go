package caddy

import (
	"bytes"
	"context"
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

func (c *AdminClient) request(ctx context.Context, method, path string, body []byte) error {
	request, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	limited, _ := io.ReadAll(io.LimitReader(response.Body, 2048))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Caddy Admin returned HTTP %d: %s", response.StatusCode, safeCommandOutput(limited))
	}
	return nil
}
