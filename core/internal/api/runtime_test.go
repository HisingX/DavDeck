package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	caddyruntime "davdeck.dev/davdeck/core/internal/caddy"
	"davdeck.dev/davdeck/core/internal/status"
)

type apiRuntime struct {
	state caddyruntime.RuntimeState
	calls []string
}

func (r *apiRuntime) Start(context.Context) error {
	r.calls = append(r.calls, "start")
	r.state = caddyruntime.RuntimeRunning
	return nil
}
func (r *apiRuntime) Stop(context.Context) error {
	r.calls = append(r.calls, "stop")
	r.state = caddyruntime.RuntimeStopped
	return nil
}
func (r *apiRuntime) Restart(context.Context) error {
	r.calls = append(r.calls, "restart")
	r.state = caddyruntime.RuntimeRunning
	return nil
}
func (r *apiRuntime) RuntimeStatus(context.Context) caddyruntime.RuntimeState { return r.state }
func (r *apiRuntime) RuntimeStatusSnapshot(context.Context) caddyruntime.RuntimeSnapshot {
	return caddyruntime.RuntimeSnapshot{Caddy: r.state, WebDAV: r.state}
}

func TestRuntimeAPIControlsManagedCaddy(t *testing.T) {
	runtime := &apiRuntime{state: caddyruntime.RuntimeRunning}
	server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithRuntimeService(runtime))
	if err != nil {
		t.Fatal(err)
	}
	statusResponse := apiRequest(t, server, http.MethodGet, "/api/v1/server/status", "")
	if statusResponse.Code != http.StatusOK || !strings.Contains(statusResponse.Body.String(), `"caddy":"RUNNING"`) {
		t.Fatalf("status = %d: %s", statusResponse.Code, statusResponse.Body.String())
	}
	for _, path := range []string{"/api/v1/server/stop", "/api/v1/server/start", "/api/v1/server/restart"} {
		response := apiRequest(t, server, http.MethodPost, path, "")
		if response.Code != http.StatusOK {
			t.Fatalf("%s = %d: %s", path, response.Code, response.Body.String())
		}
	}
	if strings.Join(runtime.calls, ",") != "stop,start,restart" {
		t.Fatalf("calls = %v", runtime.calls)
	}
}

func TestRuntimeAPIReportsEveryContractState(t *testing.T) {
	for _, state := range []caddyruntime.RuntimeState{
		caddyruntime.RuntimeNotInstalled,
		caddyruntime.RuntimeStopped,
		caddyruntime.RuntimeStarting,
		caddyruntime.RuntimeRunning,
		caddyruntime.RuntimeStopping,
		caddyruntime.RuntimeDegraded,
		caddyruntime.RuntimeFailed,
		caddyruntime.RuntimeUnknown,
	} {
		t.Run(string(state), func(t *testing.T) {
			runtime := &apiRuntime{state: state}
			server, err := NewServer("127.0.0.1:0", "secret", status.Snapshot{}, nil, WithRuntimeService(runtime))
			if err != nil {
				t.Fatal(err)
			}
			response := apiRequest(t, server, http.MethodGet, "/api/v1/server/status", "")
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"caddy":"`+string(state)+`"`) || !strings.Contains(response.Body.String(), `"webdav":"`+string(state)+`"`) {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
