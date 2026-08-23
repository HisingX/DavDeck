//go:build darwin || linux

package caddy

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

func validateBinary(path string) error {
	if !strings.ContainsRune(path, os.PathSeparator) {
		if _, err := exec.LookPath(path); err != nil {
			return &RuntimeError{Code: CodeCaddyNotFound, Message: "Caddy binary was not found", Cause: err}
		}
		return nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return &RuntimeError{Code: CodeCaddyNotFound, Message: "Caddy binary was not found", Cause: err}
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return &RuntimeError{Code: CodeCaddyNotFound, Message: "Caddy binary is not executable", Cause: err}
	}
	return nil
}
