//go:build darwin

package platform

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeServiceRunner struct {
	calls  []string
	output []byte
	err    error
}

func (r *fakeServiceRunner) Run(_ context.Context, name string, arguments ...string) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(arguments, " "))
	return r.output, r.err
}

func TestLaunchdServiceManagerLifecycle(t *testing.T) {
	runner := &fakeServiceRunner{}
	definition := filepath.Join(t.TempDir(), "dev.davdeck.davd.plist")
	manager := &launchdServiceManager{config: ServiceConfig{Executable: "/opt/DavDeck/davd", Arguments: []string{"--listen", "127.0.0.1:8090"}}, definitionPath: definition, runner: runner, privileged: func() bool { return true }}
	if err := manager.Install(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(definition); err != nil || len(runner.calls) != 1 || !strings.Contains(runner.calls[0], "bootstrap system") {
		t.Fatalf("calls = %#v, stat error = %v", runner.calls, err)
	}
	runner.output = []byte("state = running")
	status, err := manager.Status(context.Background())
	if err != nil || !status.Installed || status.State != ServiceStateRunning {
		t.Fatalf("status = %#v, err = %v", status, err)
	}
	if err := manager.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Uninstall(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(definition); !os.IsNotExist(err) {
		t.Fatalf("definition remains: %v", err)
	}
}

func TestLaunchdServiceManagerRequiresPrivilegeAndPreservesFailure(t *testing.T) {
	manager := &launchdServiceManager{config: ServiceConfig{Executable: "/opt/DavDeck/davd"}, definitionPath: filepath.Join(t.TempDir(), "service.plist"), runner: &fakeServiceRunner{}, privileged: func() bool { return false }}
	var serviceError *ServiceError
	if err := manager.Install(context.Background()); !errors.As(err, &serviceError) || serviceError.Code != CodePrivilegeRequired {
		t.Fatalf("error = %v", err)
	}
	runner := &fakeServiceRunner{err: errors.New("launchctl failed")}
	manager.runner, manager.privileged = runner, func() bool { return true }
	if err := manager.Install(context.Background()); !errors.As(err, &serviceError) || serviceError.Code != CodeServiceInstallFailed {
		t.Fatalf("error = %v", err)
	}
	if _, err := os.Stat(manager.definitionPath); !os.IsNotExist(err) {
		t.Fatalf("failed install left definition: %v", err)
	}
}
