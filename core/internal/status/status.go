// Package status defines the stable status contract shared by davd, davctl,
// and the GUI.
package status

// State is the state vocabulary used for daemon components and native
// services. Keep these values stable: they are part of the Management API.
type State string

const (
	StateNotInstalled State = "NOT_INSTALLED"
	StateStopped      State = "STOPPED"
	StateStarting     State = "STARTING"
	StateRunning      State = "RUNNING"
	StateStopping     State = "STOPPING"
	StateDegraded     State = "DEGRADED"
	StateFailed       State = "FAILED"
	StateUnknown      State = "UNKNOWN"

	DaemonRunning = string(StateRunning)
	DatabaseReady = "READY"
)

func (s State) Valid() bool {
	switch s {
	case StateNotInstalled, StateStopped, StateStarting, StateRunning,
		StateStopping, StateDegraded, StateFailed, StateUnknown:
		return true
	default:
		return false
	}
}

// ServiceStatus describes the native service independently from the daemon
// process and managed Caddy runtime.
type ServiceStatus struct {
	Installed     bool   `json:"installed"`
	State         string `json:"state"`
	StartsAtBoot  bool   `json:"starts_at_boot"`
	LastErrorCode string `json:"last_error_code,omitempty"`
}

// RuntimeStatus describes the managed Caddy/WebDAV runtime and its last
// known safe error code.
type RuntimeStatus struct {
	Caddy         string `json:"caddy"`
	WebDAV        string `json:"webdav"`
	LastErrorCode string `json:"last_error_code,omitempty"`
}

// Snapshot is safe to expose through the local Management API.
type Snapshot struct {
	Name                string        `json:"name"`
	Version             string        `json:"version"`
	Daemon              string        `json:"daemon"`
	Database            string        `json:"database"`
	SchemaVersion       int           `json:"schema_version"`
	Caddy               string        `json:"caddy"`
	WebDAV              string        `json:"webdav"`
	Service             ServiceStatus `json:"service"`
	LastErrorCode       string        `json:"last_error_code,omitempty"`
	PortableDaemonOwned bool          `json:"portable_daemon_owned"`
	DesiredRevision     *uint64       `json:"desired_revision"`
	ActiveRevision      *uint64       `json:"active_revision"`
	PendingChanges      bool          `json:"pending_changes"`
}
