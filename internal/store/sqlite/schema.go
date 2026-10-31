package sqlite

import _ "embed"

//go:embed migrations/001_core.sql
var coreMigration string

func CoreMigration() string { return coreMigration }
