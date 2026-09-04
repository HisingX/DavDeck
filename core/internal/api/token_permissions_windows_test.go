//go:build windows

package api

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestManagementTokenPermissions(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "management.token")
	if _, err := LoadOrCreateToken(path); err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatal(err)
	}
	if dacl, _, err := descriptor.DACL(); err != nil {
		t.Fatal(err)
	} else if dacl == nil {
		t.Fatal("management token ACL is missing")
	}
	control, _, err := descriptor.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("management token ACL is inheritable")
	}
}
