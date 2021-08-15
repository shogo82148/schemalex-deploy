package deploy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/shogo82148/schemalex-deploy"
	"github.com/shogo82148/schemalex-deploy/diff"
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

type Plan struct {
	From  string
	To    string
	Stmts diff.Stmts
}

// Plan generates a series statements to migrate from the current one to the new schema.
func (db *DB) Plan(ctx context.Context, schema string) (*Plan, error) {
	latest, err := getLatestVersion(ctx, db.db)
	if err != nil {
		return nil, fmt.Errorf("failed to get the current schema: %w", err)
	}

	p := schemalex.New()
	stmts1, err := p.ParseString(latest.SQLText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the current schema: %w", err)
	}

	stmts2, err := p.ParseString(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the new schema: %w", err)
	}

	stmts, err := diff.Diff(stmts1, stmts2, diff.WithTransaction(false))
	if err != nil {
		return nil, fmt.Errorf("failed to plan: %w", err)
	}

	return &Plan{
		From:  latest.SQLText,
		To:    schema,
		Stmts: stmts,
	}, nil
}

func (plan *Plan) Preview(w io.Writer) error {
	for _, stmt := range plan.Stmts {
		_, err := fmt.Fprintf(w, "%s;\n", stmt.String())
		if err != nil {
			return err
		}
	}
	return nil
}

// Deploy deploys the new schema according to the plan.
func (db *DB) Deploy(ctx context.Context, plan *Plan) error {
	log.Printf("starting to deploy")

	tx, err := db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	latest, err := getLatestVersionTx(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to get the latest version: %w", err)
	}
	if latest.SQLText != plan.From {
		return errors.New("detected unexpected change")
	}

	// disable foreign key checks during the migration.
	if _, err := tx.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return fmt.Errorf("failed to disable foreign key checks: %w", err)
	}
	// making share that foreign key checks are enabled after the migration.
	defer tx.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1")

	// migration
	for _, stmt := range plan.Stmts {
		log.Printf("executing: %s", stmt.String())
		if _, err := tx.ExecContext(ctx, stmt.String()); err != nil {
			return fmt.Errorf("failed to execute %q: %w", stmt.String(), err)
		}
	}
	log.Printf("updating the schema information")
	err = updateLatestVersion(ctx, tx, &schemalexRevision{
		SQLText:    plan.To,
		UpgradedAt: time.Now(),
	})
	if err != nil {
		return fmt.Errorf("failed to update the schema information: %w", err)
	}

	// enable foreign key checks
	if _, err := tx.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1"); err != nil {
		return fmt.Errorf("failed to disable foreign key checks: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}
	log.Printf("done")
	return nil
}

type schemalexRevision struct {
	ID         uint64
	SQLText    string
	UpgradedAt time.Time
}

// get the latest version of schema out of a transaction.
func getLatestVersion(ctx context.Context, db *sql.DB) (*schemalexRevision, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		ReadOnly: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Commit()

	latest, err := getLatestVersionTx(ctx, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to get the latest version: %w", err)
	}
	return latest, nil
}

// get the latest version of schema in a transaction.
func getLatestVersionTx(ctx context.Context, tx *sql.Tx) (*schemalexRevision, error) {
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

// update the schema information.
func updateLatestVersion(ctx context.Context, tx *sql.Tx, rev *schemalexRevision) error {
	createTable := "CREATE TABLE IF NOT EXISTS `schemalex_revision` ( " +
		"`id` BIGINT unsigned NOT NULL AUTO_INCREMENT, " +
		"`sql_text` TEXT NOT NULL, " +
		"`upgraded_at` DATETIME(6) NOT NULL, " +
		"PRIMARY KEY (`id`) " +
		") ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4"
	if _, err := tx.ExecContext(ctx, createTable); err != nil {
		return err
	}

	query := "INSERT INTO `schemalex_revision` (`sql_text`, `upgraded_at`) VALUES (?, ?)"
	if _, err := tx.ExecContext(ctx, query, rev.SQLText, rev.UpgradedAt); err != nil {
		return err
	}
	return nil
}
