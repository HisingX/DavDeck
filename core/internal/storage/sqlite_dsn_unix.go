//go:build !windows

package storage

import "net/url"

func sqliteDSN(path string) string {
	return (&url.URL{Scheme: "file", Path: path}).String() + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}
