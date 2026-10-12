// Package migrations embeds the SQL files so the binary can migrate itself on
// startup (see internal/database.Migrate). The same files work with the
// golang-migrate CLI: migrate -path migrations -database $DATABASE_URL up
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
