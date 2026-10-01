// Package migrations embeds the SQL schema history for server startup.
package migrations

import "embed"

// Files contains the versioned schema migrations.
//
//go:embed *.sql
var Files embed.FS
