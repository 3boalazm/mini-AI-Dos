// Package migrations embeds the gateway's SQL migration files so the
// migrate command (and tests) carry them inside the binary — no
// working-directory assumptions. The .sql files in this directory are
// the source of truth for the schema; this file only exposes them.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
