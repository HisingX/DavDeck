//go:build linux

package platform

// systemPaths is the fixed layout used by the Linux server installer. It is
// intentionally kept in the platform package rather than in CLI code.
func systemPaths() Paths {
	return Paths{
		DataDir:    "/var/lib/davdeck",
		ConfigDir:  "/etc/davdeck",
		RuntimeDir: "/run/davdeck",
	}
}
