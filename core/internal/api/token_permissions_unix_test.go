//go:build !windows

package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagementTokenPermissions(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "management.token")
	if _, err := LoadOrCreateToken(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("token mode = %o, want 600", info.Mode().Perm())
	}
}

func TestManagementTokenRejectsSymlink(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "management.token")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateToken(link); err == nil {
		t.Fatal("expected symlink token path to be rejected")
	}
}
