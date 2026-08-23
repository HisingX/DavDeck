// Copyright 2026 DavDeck contributors
// SPDX-License-Identifier: Apache-2.0

package webdav

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/net/webdav"
)

// rootedFileSystem adapts os.Root to the WebDAV filesystem interface. os.Root
// keeps an OS file descriptor/handle for the configured share and prevents
// operations from traversing a symlink outside it, including when the
// filesystem changes while the server is running.
type rootedFileSystem struct {
	root *os.Root
}

func openRootedFileSystem(root string) (*rootedFileSystem, error) {
	if root == "" || strings.Contains(root, "{") {
		return nil, errors.New("webdav root must be a concrete directory path")
	}
	opened, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	return &rootedFileSystem{root: opened}, nil
}

func (f *rootedFileSystem) Close() error { return f.root.Close() }

func (f *rootedFileSystem) Mkdir(_ context.Context, name string, perm os.FileMode) error {
	resolved, err := f.resolve(name)
	if err != nil {
		return err
	}
	return f.root.Mkdir(resolved, perm)
}

func (f *rootedFileSystem) OpenFile(_ context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	resolved, err := f.resolve(name)
	if err != nil {
		return nil, err
	}
	return f.root.OpenFile(resolved, flag, perm)
}

func (f *rootedFileSystem) RemoveAll(_ context.Context, name string) error {
	resolved, err := f.resolve(name)
	if err != nil {
		return err
	}
	if resolved == "." {
		return os.ErrInvalid
	}
	return f.root.RemoveAll(resolved)
}

func (f *rootedFileSystem) Rename(_ context.Context, oldName, newName string) error {
	oldResolved, err := f.resolve(oldName)
	if err != nil {
		return err
	}
	newResolved, err := f.resolve(newName)
	if err != nil {
		return err
	}
	if oldResolved == "." || newResolved == "." {
		return os.ErrInvalid
	}
	return f.root.Rename(oldResolved, newResolved)
}

func (f *rootedFileSystem) Stat(_ context.Context, name string) (os.FileInfo, error) {
	resolved, err := f.resolve(name)
	if err != nil {
		return nil, err
	}
	return f.root.Stat(resolved)
}

func (f *rootedFileSystem) resolve(name string) (string, error) {
	if strings.ContainsRune(name, 0) || (filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator)) {
		return "", os.ErrNotExist
	}
	cleaned := path.Clean("/" + name)
	if cleaned == "/" {
		return ".", nil
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}
