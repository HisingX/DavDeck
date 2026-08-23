//go:build darwin

package platform

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

// DefaultPaths resolves native per-user macOS locations.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		current, currentErr := user.Current()
		if currentErr != nil || current.HomeDir == "" {
			if currentErr != nil {
				return Paths{}, fmt.Errorf("resolve user home: %w", err)
			}
			return Paths{}, fmt.Errorf("resolve user home: current user has no home directory")
		}
		home = current.HomeDir
	}
	return Paths{
		DataDir:    filepath.Join(home, "Library", "Application Support", "DavDeck"),
		ConfigDir:  filepath.Join(home, "Library", "Application Support", "DavDeck"),
		RuntimeDir: filepath.Join(home, "Library", "Caches", "DavDeck", "run"),
		LogDir:     filepath.Join(home, "Library", "Logs", "DavDeck"),
	}, nil
}
