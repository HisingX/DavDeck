package platform

import (
	"os"
	"path/filepath"
)

// ResolveCaddyBinary prefers an explicit override, then the Caddy binary
// bundled beside davd or in a release package's libexec directory.
func ResolveCaddyBinary(override string) string {
	return resolveCaddyBinary(override, os.Executable, regularFile)
}

func resolveCaddyBinary(override string, executable func() (string, error), exists func(string) bool) string {
	if override != "" {
		return override
	}
	if path, err := executable(); err == nil {
		directory := filepath.Dir(path)
		for _, candidate := range []string{
			filepath.Join(directory, caddyExecutableName()),
			filepath.Join(directory, "..", "libexec", caddyExecutableName()),
		} {
			if exists(candidate) {
				return candidate
			}
		}
	}
	return caddyExecutableName()
}

func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
