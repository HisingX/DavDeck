package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateTokenIsStable(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config", "management.token")
	first, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || len(first) < 43 {
		t.Fatalf("token was not stable 256-bit material")
	}
}

func TestLoadOrCreateTokenRejectsInvalidExistingMaterial(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config", "management.token")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("short\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateToken(path); err == nil {
		t.Fatal("expected invalid existing token to be rejected")
	}
}
