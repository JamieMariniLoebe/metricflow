// Package store provides database operations
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool creates a new PostgreSQL connection pool
func NewPool(databaseURL string) (*pgxpool.Pool, error) {

	db, err := pgxpool.New(context.Background(), databaseURL)

	if err != nil {
		return nil, err
	}

	return db, nil
}
