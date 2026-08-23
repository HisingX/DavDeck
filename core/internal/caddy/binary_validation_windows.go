//go:build windows

package caddy

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

// Windows executables do not use Unix execute permission bits. The subsequent
// Caddy command invocation is the authoritative executable validation.
func validateBinary(path string) error {
	if !strings.ContainsAny(path, `\\/`) {
		if _, err := exec.LookPath(path); err != nil {
			return &RuntimeError{Code: CodeCaddyNotFound, Message: "Caddy binary was not found", Cause: err}
		}
		return nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return &RuntimeError{Code: CodeCaddyNotFound, Message: "Caddy binary was not found", Cause: err}
	}
	if err != nil || !info.Mode().IsRegular() {
		return &RuntimeError{Code: CodeCaddyNotFound, Message: "Caddy binary is not a regular file", Cause: err}
	}
	return nil
}
