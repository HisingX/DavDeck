// Copyright 2015 Matthew Holt
// Copyright 2026 DavDeck contributors
// SPDX-License-Identifier: Apache-2.0

// Package webdav implements a root-confined WebDAV handler module for Caddy.
package webdav

import (
	"context"
	"errors"
	"io/fs"
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
	"golang.org/x/net/webdav"
)

func init() { caddy.RegisterModule(WebDAV{}) }

// WebDAV implements an HTTP handler for responding to WebDAV clients.
type WebDAV struct {
	Root   string `json:"root,omitempty"`
	Prefix string `json:"prefix,omitempty"`

	lockSystem webdav.LockSystem
	fileSystem *rootedFileSystem
	logger     *zap.Logger
}

func (WebDAV) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "http.handlers.webdav", New: func() caddy.Module { return new(WebDAV) }}
}

// Provision pins the configured root with an OS descriptor/handle. This avoids
// resolving the root path again for each request and closes the root-symlink
// TOCTOU gap in addition to confining paths beneath the root.
func (wd *WebDAV) Provision(ctx caddy.Context) error {
	wd.logger = ctx.Logger(wd)
	wd.lockSystem = webdav.NewMemLS()
	fileSystem, err := openRootedFileSystem(wd.Root)
	if err != nil {
		return err
	}
	wd.fileSystem = fileSystem
	return nil
}

func (wd *WebDAV) Cleanup() error {
	if wd.fileSystem == nil {
		return nil
	}
	return wd.fileSystem.Close()
}

func (wd WebDAV) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	wdHandler := webdav.Handler{
		Prefix:     wd.Prefix,
		FileSystem: wd.fileSystem,
		LockSystem: wd.lockSystem,
		Logger: func(req *http.Request, err error) {
			if err == nil || errors.Is(err, fs.ErrNotExist) {
				return
			}
			if errors.Is(err, webdav.ErrConfirmationFailed) || errors.Is(err, webdav.ErrForbidden) || errors.Is(err, webdav.ErrLocked) || errors.Is(err, webdav.ErrNoSuchLock) {
				wd.logger.Debug("webdav request error", zap.Error(err), zap.Object("request", caddyhttp.LoggableHTTPRequest{Request: req}))
				return
			}
			wd.logger.Error("internal handler error", zap.Error(err), zap.Object("request", caddyhttp.LoggableHTTPRequest{Request: req}))
		},
	}

	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		info, err := wdHandler.FileSystem.Stat(context.TODO(), r.URL.Path)
		if err == nil && info.IsDir() {
			r.Method = "PROPFIND"
			if r.Header.Get("Depth") == "" {
				r.Header.Add("Depth", "1")
			}
		}
	}
	if r.Method == http.MethodHead {
		w = emptyBodyResponseWriter{w}
	}
	wdHandler.ServeHTTP(w, r)
	return nil
}

type emptyBodyResponseWriter struct{ http.ResponseWriter }

func (w emptyBodyResponseWriter) Write(data []byte) (int, error) { return 0, nil }

var (
	_ caddyhttp.MiddlewareHandler = (*WebDAV)(nil)
	_ caddy.CleanerUpper          = (*WebDAV)(nil)
	_ caddyfile.Unmarshaler       = (*WebDAV)(nil)
)
