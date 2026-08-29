package caddy

import (
	"context"
	cryptotls "crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// LocalEndpointProbe checks the managed listener locally. Named hosts use
// loopback with an explicit Host/SNI, while IP hosts use the configured IP so
// the probe follows the same network path that the dashboard advertises.
// Internal certificates are intentionally not verified here because trust is
// a per-client concern.
type LocalEndpointProbe struct{}

func (LocalEndpointProbe) Probe(ctx context.Context, scheme string, port int, hostname, path string) error {
	probeContext, cancel := context.WithTimeout(ctx, endpointProbeTimeout)
	defer cancel()

	target := url.URL{Scheme: scheme, Host: net.JoinHostPort(probeHost(hostname), strconv.Itoa(port)), Path: path}
	var lastErr error
	for {
		if err := probeOnce(probeContext, target, scheme, hostname); err == nil {
			return nil
		} else {
			lastErr = err
		}

		timer := time.NewTimer(endpointProbeRetryDelay)
		select {
		case <-probeContext.Done():
			timer.Stop()
			return fmt.Errorf("endpoint probe did not succeed: %w", lastErr)
		case <-timer.C:
		}
	}
}

const (
	endpointProbeTimeout     = 5 * time.Second
	endpointProbeAttemptTime = 750 * time.Millisecond
	endpointProbeRetryDelay  = 150 * time.Millisecond
)

func probeHost(hostname string) string {
	if net.ParseIP(hostname) != nil {
		return hostname
	}
	return "127.0.0.1"
}

func probeOnce(ctx context.Context, target url.URL, scheme, hostname string) error {
	transport := &http.Transport{Proxy: nil}
	if scheme == "https" {
		transport.TLSClientConfig = &cryptotls.Config{ServerName: hostname, InsecureSkipVerify: true} //nolint:gosec // local reachability probe; client trust is checked separately
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   endpointProbeAttemptTime,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer transport.CloseIdleConnections()
	attemptContext, cancel := context.WithTimeout(ctx, endpointProbeAttemptTime)
	defer cancel()
	request, err := http.NewRequestWithContext(attemptContext, http.MethodGet, target.String(), nil)
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
