//go:build windows

package storage

import "testing"

func TestSQLiteDSNUsesAbsoluteWindowsFileURI(t *testing.T) {
	got := sqliteDSN(`C:\Users\DavDeck Test\davdeck.db`)
	want := "file:///C:/Users/DavDeck%20Test/davdeck.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	if got != want {
		t.Fatalf("sqliteDSN() = %q, want %q", got, want)
	}
}
