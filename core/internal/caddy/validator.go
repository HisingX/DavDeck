package caddy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Validator interface {
	Validate(context.Context, []byte) error
}

type BinaryValidator struct {
	BinaryPath    string
	TempDirectory string
}

func (v BinaryValidator) Validate(ctx context.Context, configuration []byte) error {
	return v.ValidateWithEnvironment(ctx, configuration, nil)
}

// ValidateWithEnvironment validates a configuration using the same secret
// environment that will be inherited by the managed Caddy process.
func (v BinaryValidator) ValidateWithEnvironment(ctx context.Context, configuration []byte, environment map[string]string) error {
	if err := validateBinary(v.BinaryPath); err != nil {
		return err
	}
	directory := v.TempDirectory
	if directory == "" {
		directory = os.TempDir()
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return &RuntimeError{Code: CodeCaddyValidateFailed, Message: "Unable to prepare Caddy validation", Cause: err}
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return &RuntimeError{Code: CodeCaddyValidateFailed, Message: "Unable to secure Caddy validation directory", Cause: err}
	}
	file, err := os.CreateTemp(directory, "davdeck-caddy-validate-*.json")
	if err != nil {
		return &RuntimeError{Code: CodeCaddyValidateFailed, Message: "Unable to prepare Caddy validation", Cause: err}
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err == nil {
		_, err = file.Write(configuration)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return &RuntimeError{Code: CodeCaddyValidateFailed, Message: "Unable to prepare Caddy validation", Cause: err}
	}
	command := exec.CommandContext(ctx, v.BinaryPath, "validate", "--config", path)
	command.Env = environmentWithOverrides(environment)
	output, err := command.CombinedOutput()
	if err != nil {
		return &RuntimeError{Code: CodeCaddyValidateFailed, Message: "Caddy rejected the generated configuration", Cause: fmt.Errorf("%w: %s", err, safeCommandOutput(output))}
	}
	return nil
}

func safeCommandOutput(output []byte) string {
	value := strings.TrimSpace(string(output))
	if len(value) > 2048 {
		value = value[:2048]
	}
	return value
}

func writeConfigAtomically(path string, configuration []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".davdeck-caddy-*.json")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o600); err == nil {
		_, err = file.Write(configuration)
	}
	if syncErr := file.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporary, path)
}
