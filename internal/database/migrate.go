// Package database provides schema migration for PostgreSQL
package database

import (
	"database/sql"
	"errors"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations applies any pending new database schema changes
func RunMigrations(db *sql.DB, sourceURL string) error {

	driver, err := pgxv5.WithInstance(db, &pgxv5.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(sourceURL, "pgx5", driver)
	if err != nil {
		return err
	}

	err = m.Up()

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}
