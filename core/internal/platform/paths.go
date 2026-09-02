// Package platform owns operating-system-specific local path resolution.
package platform

import "path/filepath"

// Paths contains logical DavDeck storage locations.
type Paths struct {
	DataDir    string
	ConfigDir  string
	RuntimeDir string
	LogDir     string
}

func (p Paths) DatabasePath() string  { return filepath.Join(p.DataDir, "davdeck.db") }
func (p Paths) TokenPath() string     { return filepath.Join(p.ConfigDir, "management.token") }
func (p Paths) SecretKeyPath() string { return filepath.Join(p.ConfigDir, "davdeck.secret.key") }
func (p Paths) EndpointPath() string  { return filepath.Join(p.RuntimeDir, "management.endpoint") }

// SystemPaths returns the platform's system-wide service locations when the
// platform has a supported native service layout. Desktop defaults remain
// separate so a normal user process never silently switches to system paths.
func SystemPaths() Paths { return systemPaths() }
