//go:build windows

package localpermissions

import (
	"runtime"

	"golang.org/x/sys/windows"
)

// SecureDirectory restricts a DavDeck-owned local directory to the current
// Windows account using a protected ACL.
func SecureDirectory(path string) error {
	return securePathACL(path)
}

// SecureFile restricts a DavDeck-owned local file to the current Windows
// account using a protected ACL.
func SecureFile(path string) error {
	return securePathACL(path)
}

func securePathACL(path string) error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()

	user, err := token.GetTokenUser()
	if err != nil {
		return err
	}

	var pinner runtime.Pinner
	defer pinner.Unpin()
	pinner.Pin(user.User.Sid)
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	)
}
