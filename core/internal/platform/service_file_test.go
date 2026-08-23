//go:build darwin || linux

package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallServiceFileRejectsExistingAndSymlinkTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "service", "davdeck.service")
	if err := installServiceFile(path, []byte("definition")); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(path); err != nil || string(body) != "definition" {
		t.Fatalf("body = %q, err = %v", body, err)
	}
	if err := installServiceFile(path, []byte("replacement")); err == nil {
		t.Fatal("existing definition was overwritten")
	}
	symlink := filepath.Join(t.TempDir(), "davdeck.service")
	if err := os.Symlink(path, symlink); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := installServiceFile(symlink, []byte("replacement")); err == nil {
		t.Fatal("symlink definition target was accepted")
	}
}
