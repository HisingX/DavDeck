package app

import (
	"context"
	"fmt"
	"strings"

	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
)

const (
	EndpointStateNotConfigured = "NOT_CONFIGURED"
	EndpointStatePending       = "PENDING"
	EndpointStateRunning       = "RUNNING"
	EndpointStateStopped       = "STOPPED"
	EndpointStateDegraded      = "DEGRADED"
	EndpointStateFailed        = "FAILED"
	EndpointStateUnknown       = "UNKNOWN"
)

// Endpoint describes a user-facing WebDAV endpoint. Copyable is deliberately
// stricter than Configured: a configured desired endpoint is not necessarily
// the endpoint currently active in Caddy.
type Endpoint struct {
	Protocol    string `json:"protocol"`
	URL         string `json:"url"`
	Port        int    `json:"port"`
	State       string `json:"state"`
	Configured  bool   `json:"configured"`
	Active      bool   `json:"active"`
	Copyable    bool   `json:"copyable"`
	ErrorCode   string `json:"error_code,omitempty"`
	Description string `json:"description,omitempty"`
}

// EndpointSnapshot is the API representation used by the dashboard. HTTP is
// always configured in the current product; HTTPS is optional.
type EndpointSnapshot struct {
	HTTP  Endpoint `json:"http"`
	HTTPS Endpoint `json:"https"`
}

type EndpointRuntimeProvider interface {
	RuntimeStatusSnapshot(context.Context) caddyruntime.RuntimeSnapshot
}

type EndpointRevisionProvider interface {
	State(context.Context) (RevisionState, error)
}

// EndpointProbe checks the local listener and protocol handshake. It must not
// be used as a replacement for client trust validation: internal CA trust is
// necessarily a client-side requirement.
type EndpointProbe interface {
	Probe(context.Context, string, int, string, string) error
}

type EndpointService struct {
	snapshots SnapshotProvider
	runtime   EndpointRuntimeProvider
	revisions EndpointRevisionProvider
	probe     EndpointProbe
}

func NewEndpointService(snapshots SnapshotProvider, runtime EndpointRuntimeProvider, revisions EndpointRevisionProvider, probe EndpointProbe) *EndpointService {
	return &EndpointService{snapshots: snapshots, runtime: runtime, revisions: revisions, probe: probe}
}

func (s *EndpointService) Endpoints(ctx context.Context) (EndpointSnapshot, error) {
	input, err := s.snapshots.Snapshot(ctx)
	if err != nil {
		return EndpointSnapshot{}, databaseError(err)
	}

	pending := false
	if s.revisions != nil {
		state, err := s.revisions.State(ctx)
		if err != nil {
			return EndpointSnapshot{}, databaseError(err)
		}
		pending = state.Pending
	}

	runtimeStatus := caddyruntime.RuntimeSnapshot{Caddy: caddyruntime.RuntimeUnknown, WebDAV: caddyruntime.RuntimeUnknown}
	if s.runtime != nil {
		runtimeStatus = s.runtime.RuntimeStatusSnapshot(ctx)
	}

	path := input.ServerSettings.PublicBasePath
	if path != "/" {
		path = strings.TrimSuffix(path, "/") + "/"
	}

	host := "localhost"
	if input.TLSProfile != nil {
		host = input.TLSProfile.Hostname
	}

	httpEndpoint := Endpoint{
		Protocol:    "HTTP",
		URL:         endpointURL("http", host, input.ServerSettings.HTTPPort, path),
		Port:        input.ServerSettings.HTTPPort,
		State:       endpointState(runtimeStatus.Caddy, pending),
		Configured:  true,
		Description: "HTTP listener",
	}
	httpsEndpoint := Endpoint{
		Protocol:    "HTTPS",
		Port:        input.ServerSettings.HTTPSPort,
		State:       EndpointStateNotConfigured,
		Description: "HTTPS is not configured",
	}
	if input.TLSProfile != nil {
		httpsEndpoint.URL = endpointURL("https", input.TLSProfile.Hostname, input.ServerSettings.HTTPSPort, path)
		httpsEndpoint.Configured = true
		httpsEndpoint.State = endpointState(runtimeStatus.Caddy, pending)
		httpsEndpoint.Description = "HTTPS listener"
	}

	if !pending && runtimeStatus.Caddy == caddyruntime.RuntimeRunning && s.probe != nil {
		if err := s.probe.Probe(ctx, "http", httpEndpoint.Port, host, path); err != nil {
			httpEndpoint.State = EndpointStateDegraded
			httpEndpoint.ErrorCode = "ENDPOINT_UNAVAILABLE"
			httpEndpoint.Description = "HTTP endpoint is not reachable"
		}
		if httpsEndpoint.Configured {
			if err := s.probe.Probe(ctx, "https", httpsEndpoint.Port, input.TLSProfile.Hostname, path); err != nil {
				httpsEndpoint.State = EndpointStateDegraded
				httpsEndpoint.ErrorCode = "ENDPOINT_UNAVAILABLE"
				httpsEndpoint.Description = "HTTPS endpoint is not reachable"
			}
		}
	}

	httpEndpoint.Active = httpEndpoint.State == EndpointStateRunning
	httpEndpoint.Copyable = httpEndpoint.Active
	httpsEndpoint.Active = httpsEndpoint.Configured && httpsEndpoint.State == EndpointStateRunning
	httpsEndpoint.Copyable = httpsEndpoint.Active
	return EndpointSnapshot{HTTP: httpEndpoint, HTTPS: httpsEndpoint}, nil
}

func endpointState(runtimeState caddyruntime.RuntimeState, pending bool) string {
	if pending {
		return EndpointStatePending
	}
	switch runtimeState {
	case caddyruntime.RuntimeRunning:
		return EndpointStateRunning
	case caddyruntime.RuntimeStopped:
		return EndpointStateStopped
	case caddyruntime.RuntimeFailed:
		return EndpointStateFailed
	case caddyruntime.RuntimeDegraded:
		return EndpointStateDegraded
	default:
		return EndpointStateUnknown
	}
}

func endpointURL(scheme, host string, port int, path string) string {
	return fmt.Sprintf("%s://%s:%d%s", scheme, host, port, path)
}

var _ EndpointRevisionProvider = (*ApplyService)(nil)
