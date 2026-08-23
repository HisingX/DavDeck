//go:build windows

package storage

import (
	"net/url"
	"path/filepath"
)

// sqliteDSN emits file:///C:/... so SQLite does not interpret the drive letter
// as a URI authority.
func sqliteDSN(path string) string {
	return (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(path)}).String() + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}
