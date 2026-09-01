// Package persistence is the framework-layer home of the pgx pool, the
// migrations runner, and the UnitOfWork implementation. It is the only
// package allowed to construct a pgxpool.Pool or a pgx.Tx.
package persistence

import (
	"context"
	"fmt"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the pgx pool. The field stays named Pool — renaming it to avoid
// an ambiguous DB.DB would ripple through roughly sixty call sites that
// already read db.Pool.Exec / db.Pool.QueryRow.
type DB struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, url string) (*DB, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("ouverture du pool : %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("connexion à la base : %w", err)
	}
	return &DB{Pool: pool}, nil
}

func (d *DB) Close() { d.Pool.Close() }
