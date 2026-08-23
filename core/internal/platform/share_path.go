package platform

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"davdeck.dev/davdeck/core/internal/app"
)

// SharePathValidator performs local filesystem preflight without retaining files.
type SharePathValidator struct{}

func (SharePathValidator) ValidateSharePath(path string) error {
	linkInfo, err := os.Lstat(path)
	if err == nil && linkInfo.Mode()&os.ModeSymlink != 0 {
		return app.ErrSharePathUnreadable
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return app.ErrSharePathUnreadable
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) || (err == nil && !info.IsDir()) {
		return app.ErrSharePathNotFound
	}
	if err != nil {
		return app.ErrSharePathUnreadable
	}
	directory, err := os.Open(path)
	if err != nil {
		return app.ErrSharePathUnreadable
	}
	if _, err = directory.Readdirnames(1); err != nil && !errors.Is(err, io.EOF) {
		directory.Close()
		return app.ErrSharePathUnreadable
	}
	if err := directory.Close(); err != nil {
		return app.ErrSharePathUnreadable
	}
	probe, err := os.CreateTemp(path, ".davdeck-write-check-*")
	if err != nil {
		return app.ErrSharePathUnwritable
	}
	probePath := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(probePath)
		return app.ErrSharePathUnwritable
	}
	if err := os.Remove(probePath); err != nil {
		return app.ErrSharePathUnwritable
	}
	if filepath.Dir(probePath) != filepath.Clean(path) {
		return app.ErrSharePathUnwritable
	}
	return nil
}
