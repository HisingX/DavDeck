//go:build !windows

package storage

import "os"

func secureSecretKeyDirectory(path string) error {
	return os.Chmod(path, 0o700)
}

func secureSecretKeyFile(path string) error {
	return os.Chmod(path, 0o600)
}
