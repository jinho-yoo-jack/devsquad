package store

import (
	"context"
	"embed"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate requires an empty database or this service's goose history. It never
// drops/adopts legacy Flyway or Temporal tables implicitly.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	source, e := fs.Sub(migrations, "migrations")
	if e != nil {
		return e
	}
	provider, e := goose.NewProvider(goose.DialectPostgres, db, source)
	if e != nil {
		return e
	}
	_, e = provider.Up(ctx)
	return e
}
