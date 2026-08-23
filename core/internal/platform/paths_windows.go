//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPaths resolves native per-user Windows locations.
func DefaultPaths() (Paths, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user config directory: %w", err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve user cache directory: %w", err)
	}
	base := filepath.Join(config, "DavDeck")
	return Paths{
		DataDir:    base,
		ConfigDir:  base,
		RuntimeDir: filepath.Join(cache, "DavDeck", "run"),
		LogDir:     filepath.Join(cache, "DavDeck", "logs"),
	}, nil
}
