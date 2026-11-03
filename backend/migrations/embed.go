// Package migrations embeds the .sql Goose migration files so they can be run
// from the application binary, no separate goose CLI needed.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
