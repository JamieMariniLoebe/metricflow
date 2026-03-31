// Package store handles database logic
package store

import "database/sql"

func NewPool(databaseURL string) (*sql.DB, error) {

	db, err := sql.Open("postgres", databaseURL)

	if err != nil {
		return nil, err
	}

	if failErr := db.Ping(); failErr != nil {
		return nil, failErr
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	return db, err
}
