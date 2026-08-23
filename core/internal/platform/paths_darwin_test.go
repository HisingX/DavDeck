//go:build darwin

package platform

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestDefaultPathsFallsBackWhenHomeEnvironmentIsUnset(t *testing.T) {
	t.Setenv("HOME", "")
	current, err := user.Current()
	if err != nil {
		t.Fatalf("resolve current user: %v", err)
	}

	paths, err := DefaultPaths()
	if err != nil {
		t.Fatalf("DefaultPaths() error = %v", err)
	}
	wantData := filepath.Join(current.HomeDir, "Library", "Application Support", "DavDeck")
	if paths.DataDir != wantData || paths.ConfigDir != wantData {
		t.Fatalf("paths = %#v, want data/config under %q", paths, wantData)
	}
	if paths.RuntimeDir != filepath.Join(current.HomeDir, "Library", "Caches", "DavDeck", "run") {
		t.Fatalf("runtime path = %q", paths.RuntimeDir)
	}
	if paths.LogDir != filepath.Join(current.HomeDir, "Library", "Logs", "DavDeck") {
		t.Fatalf("log path = %q", paths.LogDir)
	}
	if _, err := os.Stat(current.HomeDir); err != nil {
		t.Fatalf("current home directory is unavailable: %v", err)
	}
}
