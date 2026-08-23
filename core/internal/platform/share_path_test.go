package platform

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"davdeck.dev/davdeck/core/internal/app"
)

func TestSharePathValidator(t *testing.T) {
	validator := SharePathValidator{}
	directory := t.TempDir()
	if err := validator.ValidateSharePath(directory); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("validation probe was not removed: %#v", entries)
	}
	if err := validator.ValidateSharePath(filepath.Join(directory, "missing")); !errors.Is(err, app.ErrSharePathNotFound) {
		t.Fatalf("missing error = %v", err)
	}
	file := filepath.Join(directory, "file")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := validator.ValidateSharePath(file); !errors.Is(err, app.ErrSharePathNotFound) {
		t.Fatalf("file error = %v", err)
	}
}

func TestSharePathValidatorRejectsSymlinkRoot(t *testing.T) {
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "share-link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := (SharePathValidator{}).ValidateSharePath(link); !errors.Is(err, app.ErrSharePathUnreadable) {
		t.Fatalf("symlink root error = %v", err)
	}
}
