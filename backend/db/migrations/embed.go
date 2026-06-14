// Package migrations embeds the goose .sql migration files so cmd/migrate can
// apply them from the compiled binary (no filesystem dependency in prod).
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
