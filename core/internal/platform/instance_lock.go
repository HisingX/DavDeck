package platform

import "errors"

// ErrInstanceAlreadyRunning indicates that another davd process owns this runtime directory.
var ErrInstanceAlreadyRunning = errors.New("another DavDeck daemon is already running")

// InstanceLock is held for the lifetime of a daemon process.
type InstanceLock interface {
	Release() error
}
