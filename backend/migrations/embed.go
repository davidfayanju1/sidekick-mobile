// Package migrations embeds the ordered SQL files in this directory so the
// Go binary carries them (single Postgres connection string, no external
// migration runner). Files apply in filename order.
package migrations

import "embed"

// FS holds *.sql in filename order (0001, 0002, ...).
//
//go:embed *.sql
var FS embed.FS
