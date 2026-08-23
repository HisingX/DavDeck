//go:build linux

package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeSystemdRunner struct {
	calls  []string
	output []byte
	err    error
}

func (r *fakeSystemdRunner) Run(_ context.Context, name string, arguments ...string) ([]byte, error) {
	r.calls = append(r.calls, strings.Join(append([]string{name}, arguments...), " "))
	return r.output, r.err
}

func TestSystemdServiceManagerRequiresPrivilege(t *testing.T) {
	manager := &systemdServiceManager{
		config:         ServiceConfig{Executable: filepath.Join(t.TempDir(), "davd")},
		definitionPath: filepath.Join(t.TempDir(), "davdeck.service"),
		runner:         &fakeSystemdRunner{},
		privileged:     func() bool { return false },
	}
	for _, operation := range []struct {
		name string
		call func(context.Context) error
	}{
		{"install", manager.Install},
		{"uninstall", manager.Uninstall},
		{"start", manager.Start},
		{"stop", manager.Stop},
	} {
		t.Run(operation.name, func(t *testing.T) {
			var serviceError *ServiceError
			if err := operation.call(context.Background()); !errors.As(err, &serviceError) || serviceError.Code != CodePrivilegeRequired {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestSystemdInstallFailureRemovesDefinition(t *testing.T) {
	definitionPath := filepath.Join(t.TempDir(), "davdeck.service")
	runner := &fakeSystemdRunner{err: errors.New("daemon-reload failed")}
	manager := &systemdServiceManager{
		config:         ServiceConfig{Executable: filepath.Join(t.TempDir(), "davd")},
		definitionPath: definitionPath,
		runner:         runner,
		privileged:     func() bool { return true },
	}
	var serviceError *ServiceError
	if err := manager.Install(context.Background()); !errors.As(err, &serviceError) || serviceError.Code != CodeServiceInstallFailed {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(definitionPath); !os.IsNotExist(err) {
		t.Fatalf("service definition remains after failed install: %v", err)
	}
}

func TestSystemdStatusAndUninstallPreserveApplicationData(t *testing.T) {
	testDirectory := t.TempDir()
	definitionPath := filepath.Join(testDirectory, "davdeck.service")
	dataPath := filepath.Join(testDirectory, "data", "davdeck.db")
	if err := os.MkdirAll(filepath.Dir(dataPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(definitionPath, []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dataPath, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	runner := &fakeSystemdRunner{output: []byte("loaded\nactive\n")}
	manager := &systemdServiceManager{
		config:         ServiceConfig{Executable: filepath.Join(testDirectory, "davd")},
		definitionPath: definitionPath,
		runner:         runner,
		privileged:     func() bool { return true },
	}
	serviceStatus, err := manager.Status(context.Background())
	if err != nil || !serviceStatus.Installed || serviceStatus.State != ServiceStateRunning {
		t.Fatalf("status = %#v, err = %v", serviceStatus, err)
	}
	if err := manager.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(definitionPath); !os.IsNotExist(err) {
		t.Fatalf("service definition was not removed: %v", err)
	}
	if body, err := os.ReadFile(dataPath); err != nil || string(body) != "keep" {
		t.Fatalf("application data changed: body = %q, err = %v", body, err)
	}
}

func TestSystemdMapsFailedAndBootConfiguration(t *testing.T) {
	definitionPath := filepath.Join(t.TempDir(), "davdeck.service")
	if err := os.WriteFile(definitionPath, []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := &systemdServiceManager{
		config:         ServiceConfig{Executable: filepath.Join(t.TempDir(), "davd")},
		definitionPath: definitionPath,
		runner:         &fakeSystemdRunner{output: []byte("loaded\nfailed\nenabled\n")},
		privileged:     func() bool { return true },
	}
	value, err := manager.Status(context.Background())
	if err != nil || value.State != ServiceStateFailed || !value.StartsAtBoot {
		t.Fatalf("status = %#v, err = %v", value, err)
	}
}
