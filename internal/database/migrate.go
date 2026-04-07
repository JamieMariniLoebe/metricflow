// Package database provides schema migration for PostgreSQL
package database

import (
	"errors"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies any pending new database schema changes
func RunMigrations(databaseURL string, sourceURL string) error {

	m, err := migrate.New(sourceURL, databaseURL)

	if err != nil {
		return err
	}

	err = m.Up()

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}
