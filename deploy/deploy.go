package deploy

import (
	"context"
	"database/sql"
)

// DB is the target of deploying a DDL schema.
type DB struct {
	db *sql.DB
}

// Open opens a database specified by its database driver name.
func Open(driverName string, dataSourceName string) (*DB, error) {
	db, err := sql.Open("mysql", "")
	if err != nil {
		return nil, err
	}
	return &DB{
		db: db,
	}, nil
}

// Close closes the database.
func (db *DB) Close() error {
	return db.db.Close()
}

// Deploy deploys the schema.
func (db *DB) Deploy(ctx context.Context) error {
	return nil
}
