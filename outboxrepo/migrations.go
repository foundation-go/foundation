package outboxrepo

import "embed"

// Migrations holds the SQL that creates the outbox table.
//
// Embedding them means `foundation init-outbox` copies the migrations of the
// Foundation version the service actually depends on. It used to scan
// $GOMODCACHE/github.com/foundation-go for any directory containing
// outboxrepo/migrations and take the first one os.ReadDir returned — which is
// alphabetical, so a machine with several versions cached would hand over the
// oldest one.
//
//go:embed migrations/*.sql
var Migrations embed.FS

// MigrationsDir is the directory the migrations are embedded under.
const MigrationsDir = "migrations"
