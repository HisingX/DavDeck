package webdav

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRootedFileSystemAllowsInRootFilesAndRejectsExternalSymlink(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inside.txt")
	if err := os.WriteFile(inside, []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	filesystem, err := openRootedFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	defer filesystem.Close()
	file, err := filesystem.OpenFile(context.Background(), "/inside.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("open in-root file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := filesystem.OpenFile(context.Background(), "/escape/secret.txt", os.O_RDONLY, 0); err == nil {
		t.Fatal("external symlink was accessible")
	}
	if _, err := filesystem.Stat(context.Background(), "/escape/secret.txt"); err == nil {
		t.Fatal("external symlink target was statable")
	}
}

func TestRootedFileSystemKeepsOriginalRootHandleAfterPathReplacement(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "share")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("inside"), 0o600); err != nil {
		t.Fatal(err)
	}
	filesystem, err := openRootedFileSystem(root)
	if err != nil {
		t.Fatal(err)
	}
	defer filesystem.Close()
	if err := os.Rename(root, filepath.Join(parent, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), root); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	file, err := filesystem.OpenFile(context.Background(), "/inside.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("root handle no longer references original share: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
