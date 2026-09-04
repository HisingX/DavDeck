//go:build windows

package storage

import "davdeck.dev/davdeck/core/internal/platform/localpermissions"

func secureSecretKeyDirectory(path string) error {
	return localpermissions.SecureDirectory(path)
}

func secureSecretKeyFile(path string) error {
	return localpermissions.SecureFile(path)
}
