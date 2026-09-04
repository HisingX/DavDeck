package api

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"davdeck.dev/davdeck/core/internal/platform/localpermissions"
)

// LoadOrCreateToken reads an existing management token or atomically creates a
// new 256-bit token with owner-only file permissions.
func LoadOrCreateToken(path string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create token directory: %w", err)
	}
	if err := localpermissions.SecureDirectory(filepath.Dir(path)); err != nil {
		return "", fmt.Errorf("secure token directory: %w", err)
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("management token path must be a regular file")
		}
		if err := localpermissions.SecureFile(path); err != nil {
			return "", fmt.Errorf("secure management token: %w", err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read management token: %w", err)
		}
		token := strings.TrimSpace(string(body))
		if err := validateToken(token); err != nil {
			return "", err
		}
		return token, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read management token: %w", err)
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", fmt.Errorf("generate management token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	temporary, err := os.CreateTemp(filepath.Dir(path), ".management-token-*")
	if err != nil {
		return "", fmt.Errorf("create management token: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := localpermissions.SecureFile(temporaryPath); err != nil {
		temporary.Close()
		return "", fmt.Errorf("secure temporary management token: %w", err)
	}
	if _, err := temporary.WriteString(token + "\n"); err != nil {
		temporary.Close()
		return "", fmt.Errorf("write management token: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return "", fmt.Errorf("close management token: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return "", fmt.Errorf("install management token: %w", err)
	}
	if err := localpermissions.SecureFile(path); err != nil {
		return "", fmt.Errorf("secure management token: %w", err)
	}
	return token, nil
}

func validateToken(token string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf("management token must contain 256 bits of encoded random data")
	}
	return nil
}
