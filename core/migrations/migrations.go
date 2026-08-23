// Package migrations exposes the immutable, embedded SQLite migrations.
package migrations

import "embed"

// Files contains every released migration. Existing files must never be edited
// after release; add a new, monotonically numbered file instead.
//
//go:embed *.sql
var Files embed.FS
