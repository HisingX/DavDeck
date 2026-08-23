//go:build !windows

package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenSecuresDatabasePathAndRejectsSymlink(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "data")
	path := filepath.Join(directory, "davdeck.db")
	database, _, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	database.Close()

	directoryInfo, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("database directory mode = %o, want 700", directoryInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("database file mode = %o, want 600", fileInfo.Mode().Perm())
	}

	target := filepath.Join(t.TempDir(), "target.db")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "linked.db")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Open(context.Background(), link); err == nil {
		t.Fatal("expected database symlink to be rejected")
	}
	contents, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "target" {
		t.Fatal("symlink target was modified")
	}
}
