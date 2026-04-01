// Package store handles database logic
package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(databaseURL string) (*pgxpool.Pool, error) {

	db, err := pgxpool.New(context.Background(), databaseURL)

	if err != nil {
		return nil, err
	}

	return db, nil
}
