package app

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/domain"
)

type endpointSnapshot struct {
	input domain.RuntimeConfigInput
}

func (s endpointSnapshot) Snapshot(context.Context) (domain.RuntimeConfigInput, error) {
	return s.input, nil
}

type endpointRuntime struct {
	status caddyruntime.RuntimeSnapshot
}

func (r endpointRuntime) RuntimeStatusSnapshot(context.Context) caddyruntime.RuntimeSnapshot {
	return r.status
}

type endpointRevisions struct {
	pending bool
}

func (r endpointRevisions) State(context.Context) (RevisionState, error) {
	return RevisionState{Pending: r.pending}, nil
}

type endpointProbe struct {
	err      error
	requests []string
}

func (p *endpointProbe) Probe(_ context.Context, scheme string, port int, host, path string) error {
	p.requests = append(p.requests, scheme+":"+host+":"+strconv.Itoa(port)+path)
	return p.err
}

func endpointInput(t *testing.T, tlsProfile *domain.TLSProfile) domain.RuntimeConfigInput {
	t.Helper()
	stamp, err := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return domain.RuntimeConfigInput{
		ServerSettings: domain.ServerSettings{
			ID:             "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
			PublicBasePath: "/dav",
			HTTPPort:       8080,
			HTTPSPort:      8443,
			RuntimeMode:    domain.RuntimeModePortable,
			CreatedAt:      stamp,
			UpdatedAt:      stamp,
		},
		TLSProfile: tlsProfile,
	}
}

func TestEndpointServiceSeparatesUnconfiguredHTTPS(t *testing.T) {
	probe := &endpointProbe{}
	service := NewEndpointService(
		endpointSnapshot{input: endpointInput(t, nil)},
		endpointRuntime{status: caddyruntime.RuntimeSnapshot{Caddy: caddyruntime.RuntimeRunning}},
		endpointRevisions{},
		probe,
	)

	endpoints, err := service.Endpoints(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if endpoints.HTTP.URL != "http://localhost:8080/dav/" || !endpoints.HTTP.Copyable {
		t.Fatalf("HTTP endpoint = %#v", endpoints.HTTP)
	}
	if endpoints.HTTPS.Configured || endpoints.HTTPS.URL != "" || endpoints.HTTPS.Copyable || endpoints.HTTPS.State != EndpointStateNotConfigured {
		t.Fatalf("HTTPS endpoint = %#v", endpoints.HTTPS)
	}
	if len(probe.requests) != 1 {
		t.Fatalf("probe requests = %d, want 1", len(probe.requests))
	}
}

func TestEndpointServiceUsesTLSHostnameAndDisablesCopyWhilePending(t *testing.T) {
	stamp, _ := domain.NewTimestamp(time.Date(2026, 8, 20, 1, 0, 0, 0, time.UTC))
	profile := &domain.TLSProfile{ID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", Mode: domain.TLSModeInternal, Hostname: "dav.local", CreatedAt: stamp, UpdatedAt: stamp}
	probe := &endpointProbe{}
	service := NewEndpointService(
		endpointSnapshot{input: endpointInput(t, profile)},
		endpointRuntime{status: caddyruntime.RuntimeSnapshot{Caddy: caddyruntime.RuntimeRunning}},
		endpointRevisions{pending: true},
		probe,
	)

	endpoints, err := service.Endpoints(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if endpoints.HTTP.URL != "http://dav.local:8080/dav/" || endpoints.HTTPS.URL != "https://dav.local:8443/dav/" {
		t.Fatalf("endpoints = %#v", endpoints)
	}
	if endpoints.HTTP.State != EndpointStatePending || endpoints.HTTPS.State != EndpointStatePending || endpoints.HTTP.Copyable || endpoints.HTTPS.Copyable {
		t.Fatalf("pending endpoints = %#v", endpoints)
	}
	if len(probe.requests) != 0 {
		t.Fatalf("pending configuration was probed: %#v", probe.requests)
	}
}

func TestEndpointServiceMarksProbeFailureUnavailable(t *testing.T) {
	probe := &endpointProbe{err: errors.New("connection refused")}
	service := NewEndpointService(
		endpointSnapshot{input: endpointInput(t, nil)},
		endpointRuntime{status: caddyruntime.RuntimeSnapshot{Caddy: caddyruntime.RuntimeRunning}},
		endpointRevisions{},
		probe,
	)

	endpoints, err := service.Endpoints(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if endpoints.HTTP.State != EndpointStateDegraded || endpoints.HTTP.Copyable || endpoints.HTTP.ErrorCode != "ENDPOINT_UNAVAILABLE" {
		t.Fatalf("HTTP endpoint = %#v", endpoints.HTTP)
	}
}
