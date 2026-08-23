//go:build darwin || linux

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

func installServiceFile(path string, body []byte) error {
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("service definition path must be a regular file")
		}
		return fmt.Errorf("service definition already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect service definition: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create service definition directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create service definition: %w", err)
	}
	remove := true
	defer func() {
		file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if _, err := file.Write(body); err != nil {
		return fmt.Errorf("write service definition: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync service definition: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close service definition: %w", err)
	}
	remove = false
	return nil
}
