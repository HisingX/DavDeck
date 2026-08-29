package caddy

import (
	"context"
	cryptotls "crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// LocalEndpointProbe checks the managed listener through loopback. Internal
// certificates are intentionally not verified here because trust is a
// per-client concern; this probe only establishes that the TLS listener and
// configured SNI are usable.
type LocalEndpointProbe struct{}

func (LocalEndpointProbe) Probe(ctx context.Context, scheme string, port int, hostname, path string) error {
	target := url.URL{Scheme: scheme, Host: "127.0.0.1:" + strconv.Itoa(port), Path: path}
	transport := &http.Transport{Proxy: nil}
	if scheme == "https" {
		transport.TLSClientConfig = &cryptotls.Config{ServerName: hostname, InsecureSkipVerify: true} //nolint:gosec // local reachability probe; client trust is checked separately
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   3 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	if hostname != "" {
		request.Host = hostname
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("endpoint returned HTTP status %d", response.StatusCode)
	}
	return nil
}
