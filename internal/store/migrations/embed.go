package migrations

import "embed"

// FS contains SQLite migrations for assistant-local event indexes.
//
//go:embed *.sql
var FS embed.FS
