// Copyright 2026 DavDeck contributors
// SPDX-License-Identifier: Apache-2.0

package webdav

import (
	"errors"
	"html"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

var discoverySlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// DiscoveryEntry describes one Share exposed as a child collection of the
// authenticated WebDAV discovery root. It intentionally contains no physical
// filesystem path.
type DiscoveryEntry struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Discovery serves the authenticated virtual collection at the configured
// WebDAV base path. It only discovers Share URLs; the individual Share routes
// continue to enforce file access and write permissions.
type Discovery struct {
	BasePath string                      `json:"base_path,omitempty"`
	Entries  map[string][]DiscoveryEntry `json:"entries,omitempty"`
}

func init() { caddy.RegisterModule(Discovery{}) }

func (Discovery) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{ID: "http.handlers.davdeck_index", New: func() caddy.Module { return new(Discovery) }}
}

func (d Discovery) Validate() error {
	if d.BasePath == "" || !strings.HasPrefix(d.BasePath, "/") || strings.ContainsAny(d.BasePath, "?#\\") || path.Clean(d.BasePath) != d.BasePath {
		return errors.New("discovery base path must be a canonical absolute URL path")
	}
	for username, entries := range d.Entries {
		if username == "" || strings.IndexFunc(username, unicode.IsControl) >= 0 {
			return errors.New("discovery username must be non-empty and contain no control characters")
		}
		seen := make(map[string]struct{}, len(entries))
		for _, entry := range entries {
			if !discoverySlugPattern.MatchString(entry.Slug) {
				return errors.New("discovery entry slug is invalid")
			}
			if entry.Name == "" {
				return errors.New("discovery entry name must not be empty")
			}
			if _, ok := seen[entry.Slug]; ok {
				return errors.New("discovery entry slug is duplicated")
			}
			seen[entry.Slug] = struct{}{}
		}
	}
	return nil
}

func (d Discovery) ServeHTTP(w http.ResponseWriter, r *http.Request, _ caddyhttp.Handler) error {
	repl, ok := r.Context().Value(caddy.ReplacerCtxKey).(*caddy.Replacer)
	if !ok || repl == nil {
		return caddyhttp.Error(http.StatusForbidden, errors.New("WebDAV discovery authentication context is missing"))
	}
	value, _ := repl.Get("http.auth.user.id")
	username, _ := value.(string)
	entries, ok := d.Entries[username]
	if !ok || len(entries) == 0 {
		return caddyhttp.Error(http.StatusForbidden, errors.New("no WebDAV shares available"))
	}

	setDiscoveryHeaders(w)
	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
		return nil
	case http.MethodGet, http.MethodHead:
		body := d.htmlListing(entries)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len([]byte(body))))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(body))
		}
		return nil
	case "PROPFIND":
		depth := strings.TrimSpace(r.Header.Get("Depth"))
		if depth == "" {
			depth = "1"
		}
		if depth != "0" && depth != "1" {
			return caddyhttp.Error(http.StatusForbidden, errors.New("WebDAV discovery depth is limited to zero or one"))
		}
		body := d.propfindListing(entries, depth == "1")
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len([]byte(body))))
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write([]byte(body))
		return nil
	default:
		return caddyhttp.Error(http.StatusMethodNotAllowed, errors.New("WebDAV discovery root is read-only"))
	}
}

func (d Discovery) htmlListing(entries []DiscoveryEntry) string {
	var builder strings.Builder
	builder.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>WebDAV</title></head><body><h1>WebDAV</h1><ul>")
	for _, entry := range entries {
		builder.WriteString("<li><a href=\"")
		builder.WriteString(html.EscapeString(d.entryPath(entry.Slug)))
		builder.WriteString("\">")
		builder.WriteString(html.EscapeString(entry.Name))
		builder.WriteString("</a></li>")
	}
	builder.WriteString("</ul></body></html>\n")
	return builder.String()
}

func (d Discovery) propfindListing(entries []DiscoveryEntry, includeEntries bool) string {
	var builder strings.Builder
	builder.WriteString("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<D:multistatus xmlns:D=\"DAV:\">")
	writePropfindResponse(&builder, d.rootPath(), "WebDAV", true)
	if includeEntries {
		for _, entry := range entries {
			writePropfindResponse(&builder, d.entryPath(entry.Slug), entry.Name, true)
		}
	}
	builder.WriteString("</D:multistatus>\n")
	return builder.String()
}

func writePropfindResponse(builder *strings.Builder, href, displayName string, collection bool) {
	builder.WriteString("<D:response><D:href>")
	writeXMLEscaped(builder, href)
	builder.WriteString("</D:href><D:propstat><D:prop><D:displayname>")
	writeXMLEscaped(builder, displayName)
	builder.WriteString("</D:displayname><D:resourcetype>")
	if collection {
		builder.WriteString("<D:collection/>")
	}
	builder.WriteString("</D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>")
}

func writeXMLEscaped(builder *strings.Builder, value string) {
	for _, character := range value {
		switch character {
		case '&':
			builder.WriteString("&amp;")
		case '<':
			builder.WriteString("&lt;")
		case '>':
			builder.WriteString("&gt;")
		case '"':
			builder.WriteString("&quot;")
		case '\'':
			builder.WriteString("&apos;")
		default:
			builder.WriteRune(character)
		}
	}
}

func (d Discovery) rootPath() string {
	if d.BasePath == "/" {
		return "/"
	}
	return strings.TrimSuffix(d.BasePath, "/") + "/"
}

func (d Discovery) entryPath(slug string) string { return d.rootPath() + slug + "/" }

func setDiscoveryHeaders(w http.ResponseWriter) {
	w.Header().Set("DAV", "1")
	w.Header().Set("Allow", "OPTIONS, GET, HEAD, PROPFIND")
}

var (
	_ caddy.Validator             = Discovery{}
	_ caddyhttp.MiddlewareHandler = Discovery{}
)
