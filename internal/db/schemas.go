package db

import "embed"

// sqlSchemas is an embedded file system containing the SQL migration files.
// The migrations are embedded at compile time for portability.
//
//go:embed migrations/*.sql
var sqlSchemas embed.FS
