//go:build linux

package platform

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestDefaultPathsUsesPasswdHomeWhenHOMEIsUnset(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	if current.HomeDir == "" {
		t.Fatal("current user has no home directory")
	}
	t.Setenv("HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	paths, err := DefaultPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.DataDir != filepath.Join(current.HomeDir, ".local", "share", "DavDeck") {
		t.Fatalf("data directory = %q", paths.DataDir)
	}
	if paths.ConfigDir != filepath.Join(current.HomeDir, ".config", "DavDeck") {
		t.Fatalf("config directory = %q", paths.ConfigDir)
	}
	if paths.RuntimeDir != filepath.Join(current.HomeDir, ".cache", "DavDeck", "run") {
		t.Fatalf("runtime directory = %q", paths.RuntimeDir)
	}
	if _, err := os.Stat(current.HomeDir); err != nil {
		t.Fatalf("home directory unavailable: %v", err)
	}
}

func TestSystemPathsUseLinuxServerLayout(t *testing.T) {
	paths := SystemPaths()
	if paths.DataDir != "/var/lib/davdeck" || paths.ConfigDir != "/etc/davdeck" || paths.RuntimeDir != "/run/davdeck" {
		t.Fatalf("system paths = %#v", paths)
	}
}
