package deploy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// DB is the target of deploying a DDL schema.
type DB struct {
	db *sql.DB
}

// Open opens a database specified by its database driver name.
func Open(driverName string, dataSourceName string) (*DB, error) {
	db, err := sql.Open(driverName, dataSourceName)
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
	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	latest, err := getLatestVersion(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to get the latest version: %w", err)
	}

	_ = latest

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	return nil
}

type schemalexRevision struct {
	ID         uint64
	SQLText    string
	UpgradedAt time.Time
}

// get the latest version of schema.
func getLatestVersion(ctx context.Context, tx *sql.Tx) (*schemalexRevision, error) {
	var rev schemalexRevision
	row := tx.QueryRowContext(ctx, "SELECT `id`, `sql_text`, `upgraded_at` FROM `schemalex_revision` ORDER BY `id` DESC LIMIT 1")
	err := row.Scan(&rev.ID, &rev.SQLText, &rev.UpgradedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// the database is not initialized.
			return &schemalexRevision{}, nil
		}

		var myerr *mysql.MySQLError
		if errors.As(err, &myerr) {
			// https://dev.mysql.com/doc/mysql-errors/8.0/en/server-error-reference.html#error_er_no_such_table
			if myerr.Number == 1146 { // = ER_NO_SUCH_TABLE: Table 'schemalex_revision' doesn't exist
				// the database is not initialized.
				return &schemalexRevision{}, nil
			}
		}

		return nil, err
	}
	return &rev, nil
}

func updateLatestVersion(ctx context.Context, tx *sql.Tx) error {
	createTable := "CREATE TABLE IF NOT EXISTS `schemalex_revision` ( " +
		"`id` BIGINT unsigned NOT NULL AUTO_INCREMENT, " +
		"`sql_text` TEXT NOT NULL, " +
		"`upgraded_at` DATETIME(6) NOT NULL, " +
		"PRIMARY KEY (`id`) " +
		") ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4"
	_, err := tx.ExecContext(ctx, createTable)
	return err
}
