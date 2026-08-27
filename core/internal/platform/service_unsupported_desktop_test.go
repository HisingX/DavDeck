//go:build darwin || windows

package platform

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestNewServiceManagerIsUnsupportedOnDesktopPlatforms(t *testing.T) {
	manager, err := NewServiceManager(ServiceConfig{
		Executable: filepath.Join(t.TempDir(), "davd"),
	})
	if err != nil {
		t.Fatal(err)
	}
	var serviceErr *ServiceError
	if err := manager.Install(context.Background()); !errors.As(err, &serviceErr) || serviceErr.Code != CodePlatformUnsupported {
		t.Fatalf("install error = %v", err)
	}
}
