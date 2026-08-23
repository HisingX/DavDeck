//go:build darwin || linux

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

type fileInstanceLock struct{ file *os.File }

// AcquireInstanceLock obtains an advisory lock scoped to one DavDeck runtime directory.
func AcquireInstanceLock(runtimeDir string) (InstanceLock, error) {
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return nil, fmt.Errorf("create runtime directory: %w", err)
	}
	path := filepath.Join(runtimeDir, "davd.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open daemon lock: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("secure daemon lock: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrInstanceAlreadyRunning
		}
		return nil, fmt.Errorf("lock daemon runtime: %w", err)
	}
	return &fileInstanceLock{file: file}, nil
}

func (l *fileInstanceLock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}
