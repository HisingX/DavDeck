//go:build windows

package caddy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBinaryAcceptsRegularWindowsExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "caddy.exe")
	if err := os.WriteFile(path, []byte("test executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validateBinary(path); err != nil {
		t.Fatalf("validateBinary(%q): %v", path, err)
	}
}
