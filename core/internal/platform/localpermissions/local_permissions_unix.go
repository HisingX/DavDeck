//go:build !windows

package localpermissions

import "os"

// SecureDirectory restricts a DavDeck-owned local directory to the current
// account using Unix file permissions.
func SecureDirectory(path string) error {
	return os.Chmod(path, 0o700)
}

// SecureFile restricts a DavDeck-owned local file to the current account
// using Unix file permissions.
func SecureFile(path string) error {
	return os.Chmod(path, 0o600)
}
