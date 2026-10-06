package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The current orchestrator owns local goroutines and workspaces. A dedicated
// PostgreSQL session rejects a second service instance for the same schema.
func AcquireInstance(ctx context.Context, pool *pgxpool.Pool) (*pgxpool.Conn, error) {
	conn, e := pool.Acquire(ctx)
	if e != nil {
		return nil, e
	}
	var acquired bool
	e = conn.QueryRow(ctx, "select pg_try_advisory_lock(hashtext(current_database()),hashtext(current_schema() || '.devsquad'))").Scan(&acquired)
	if e != nil || !acquired {
		conn.Release()
		if e != nil {
			return nil, e
		}
		return nil, fmt.Errorf("another devsquad instance owns this database schema")
	}
	return conn, nil
}
