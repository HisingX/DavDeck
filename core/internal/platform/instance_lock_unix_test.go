//go:build darwin || linux

package platform

import (
	"errors"
	"testing"
)

func TestAcquireInstanceLockExcludesSecondOwner(t *testing.T) {
	directory := t.TempDir()
	first, err := AcquireInstanceLock(directory)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Release()
	second, err := AcquireInstanceLock(directory)
	if second != nil || !errors.Is(err, ErrInstanceAlreadyRunning) {
		t.Fatalf("second = %#v, err = %v", second, err)
	}
}
