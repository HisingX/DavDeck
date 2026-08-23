//go:build windows

package platform

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

type mutexInstanceLock struct{ handle windows.Handle }

// AcquireInstanceLock uses a per-user Windows mutex keyed by the runtime directory.
func AcquireInstanceLock(runtimeDir string) (InstanceLock, error) {
	digest := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(runtimeDir))))
	name, err := windows.UTF16PtrFromString(fmt.Sprintf("Local\\DavDeck-davd-%x", digest[:]))
	if err != nil {
		return nil, fmt.Errorf("encode daemon lock name: %w", err)
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		return nil, fmt.Errorf("create daemon lock: %w", err)
	}
	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		_ = windows.CloseHandle(handle)
		return nil, ErrInstanceAlreadyRunning
	}
	return &mutexInstanceLock{handle: handle}, nil
}

func (l *mutexInstanceLock) Release() error {
	if l == nil || l.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(l.handle)
	l.handle = 0
	return err
}
