// Package buildinfo exposes release metadata embedded by the build pipeline.
package buildinfo

import "runtime"

// These values are replaced with -ldflags for release builds. Development
// builds intentionally remain explicit instead of guessing from the checkout.
var (
	Version        = "dev"
	GitCommit      = "unknown"
	BuildDate      = "unknown"
	FlutterVersion = "unknown"
	CaddyVersion   = "unknown"
	WebDAVVersion  = "unknown"
)

// Info is safe to expose through the CLI and sanitized diagnostics.
type Info struct {
	Product        string `json:"product"`
	Version        string `json:"version"`
	GitCommit      string `json:"git_commit"`
	BuildDate      string `json:"build_date"`
	GoVersion      string `json:"go_version"`
	FlutterVersion string `json:"flutter_version"`
	CaddyVersion   string `json:"caddy_version"`
	WebDAVVersion  string `json:"caddy_webdav_version"`
	TargetOS       string `json:"target_os"`
	TargetArch     string `json:"target_arch"`
}

// Current returns metadata for this executable and target platform.
func Current() Info {
	return Info{
		Product:        "DavDeck",
		Version:        Version,
		GitCommit:      GitCommit,
		BuildDate:      BuildDate,
		GoVersion:      runtime.Version(),
		FlutterVersion: FlutterVersion,
		CaddyVersion:   CaddyVersion,
		WebDAVVersion:  WebDAVVersion,
		TargetOS:       runtime.GOOS,
		TargetArch:     runtime.GOARCH,
	}
}
