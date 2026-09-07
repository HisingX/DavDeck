//go:build !windows

package storage

import (
	"os"
	"testing"
)

func assertSecretKeyPermissions(t *testing.T, path string) {
	t.Helper()
	keyInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if keyInfo.Mode().Perm() != 0o600 {
		t.Fatalf("key permissions = %o, want 600", keyInfo.Mode().Perm())
	}
}
