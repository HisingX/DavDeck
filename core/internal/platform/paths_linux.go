//go:build linux

package platform

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

// DefaultPaths resolves XDG-compatible per-user Linux locations.
func DefaultPaths() (Paths, error) {
	home, err := linuxUserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	dataHome := envOr("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	configHome := envOr("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	cacheHome := envOr("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	runtimeDir := filepath.Join(cacheHome, "DavDeck", "run")
	if value := os.Getenv("XDG_RUNTIME_DIR"); value != "" {
		runtimeDir = filepath.Join(value, "DavDeck")
	}
	return Paths{
		DataDir:    filepath.Join(dataHome, "DavDeck"),
		ConfigDir:  filepath.Join(configHome, "DavDeck"),
		RuntimeDir: runtimeDir,
		LogDir:     filepath.Join(cacheHome, "DavDeck", "logs"),
	}, nil
}

func linuxUserHomeDir() (string, error) {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home, nil
	}
	current, err := user.Current()
	if err != nil || current.HomeDir == "" {
		if err == nil {
			err = fmt.Errorf("home directory is empty")
		}
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return current.HomeDir, nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
