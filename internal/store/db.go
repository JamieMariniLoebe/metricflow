// Package store provides database operations
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a new PostgreSQL connection pool
func NewPool(cfg *pgxpool.Config) (*pgxpool.Pool, error) {

	db, err := pgxpool.NewWithConfig(context.Background(), cfg)

	if err != nil {
		return nil, err
	}

	return db, nil
}
