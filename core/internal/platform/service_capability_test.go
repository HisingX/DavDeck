package platform

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestUnsupportedServiceManagerReturnsStablePlatformError(t *testing.T) {
	manager, err := newUnsupportedServiceManager(ServiceConfig{
		Executable: filepath.Join(t.TempDir(), "davd"),
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	operations := []struct {
		name string
		call func() error
	}{
		{name: "install", call: func() error { return manager.Install(ctx) }},
		{name: "uninstall", call: func() error { return manager.Uninstall(ctx) }},
		{name: "start", call: func() error { return manager.Start(ctx) }},
		{name: "stop", call: func() error { return manager.Stop(ctx) }},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			err := operation.call()
			var serviceErr *ServiceError
			if !errors.As(err, &serviceErr) {
				t.Fatalf("expected ServiceError, got %T", err)
			}
			if serviceErr.Code != CodePlatformUnsupported {
				t.Fatalf("unexpected error code: %s", serviceErr.Code)
			}
		})
	}

	status, err := manager.Status(ctx)
	if status.State != ServiceStateUnknown {
		t.Fatalf("unexpected status state: %s", status.State)
	}
	var serviceErr *ServiceError
	if !errors.As(err, &serviceErr) || serviceErr.Code != CodePlatformUnsupported {
		t.Fatalf("unexpected status error: %v", err)
	}
}
