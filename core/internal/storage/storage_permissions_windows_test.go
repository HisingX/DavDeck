//go:build windows

package storage

import (
	"context"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestOpenSecuresDatabasePath(t *testing.T) {
	t.Parallel()
	directory := filepath.Join(t.TempDir(), "data")
	path := filepath.Join(directory, "davdeck.db")
	database, _, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}

	assertProtectedDACL(t, directory, "database directory")
	assertProtectedDACL(t, path, "database file")
}

func assertProtectedDACL(t *testing.T, path, description string) {
	t.Helper()
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("%s security info: %v", description, err)
	}
	if dacl, _, err := descriptor.DACL(); err != nil {
		t.Fatalf("%s DACL: %v", description, err)
	} else if dacl == nil {
		t.Fatalf("%s ACL is missing", description)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatalf("%s ACL control: %v", description, err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("%s ACL is inheritable", description)
	}
}
