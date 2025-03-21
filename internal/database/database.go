package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/shogo82148/schemalex-deploy/internal/util"
)

// SetupTestDB creates a database for testing.
// Run the following command to run this on your local machine.
//
//	docker run -d -p 127.0.0.1:3306:3306 -e MYSQL_ROOT_PASSWORD=verysecret -e MYSQL_ROOT_HOST='%' mysql/mysql-server:8.0
func SetupTestDB() (*sql.DB, func()) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	user := os.Getenv("SCHEMALEX_DATABASE_USER")
	password := os.Getenv("SCHEMALEX_DATABASE_PASSWORD")
	host := os.Getenv("SCHEMALEX_DATABASE_HOST")
	port := os.Getenv("SCHEMALEX_DATABASE_PORT")
	if port == "" {
		port = "3306"
	}
	addr := net.JoinHostPort(host, port)

	config := mysql.NewConfig()
	config.User = user
	config.Passwd = password
	config.Addr = addr
	config.ParseTime = true
	config.RejectReadOnly = true
	config.Params = map[string]string{
		"charset": "utf8mb4",
		// kamipo TRADITIONAL http://www.songmu.jp/riji/entry/2015-07-08-kamipo-traditional.html
		"sql_mode": "'TRADITIONAL,NO_AUTO_VALUE_ON_ZERO,ONLY_FULL_GROUP_BY'",
	}

	// create temporary database for testing
	db1, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		panic(err)
	}
	defer db1.Close()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	dbName := "schemalex_test_" + hex.EncodeToString(b[:])
	_, err = db1.ExecContext(ctx, "CREATE DATABASE "+util.Backquote(dbName))
	if err != nil {
		panic(err)
	}

	// create a new connection pool for testing
	config.DBName = dbName
	db2, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		panic(err)
	}

	return db2, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// clean up
		db2.ExecContext(ctx, "DROP DATABASE "+util.Backquote(dbName))
		db2.Close()
	}
}

// HasTestDatabase returns whether a database for testing is configured.
func HasTestDatabase() bool {
	return os.Getenv("SCHEMALEX_DATABASE_HOST") != ""
}

// SkipIfNoTestDatabase skips tests if the database for testing is not configured.
func SkipIfNoTestDatabase(t *testing.T) {
	if !HasTestDatabase() {
		t.Skip("SCHEMALEX_DATABASE_HOST is not set. skip this test.")
	}
}

func ListTables(ctx context.Context, db *sql.DB) (tables, views []string, err error) {
	rows, err := db.QueryContext(ctx, "SHOW FULL TABLES")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, tableType string
		if err := rows.Scan(&tableName, &tableType); err != nil {
			return nil, nil, err
		}
		switch tableType {
		case "BASE TABLE":
			tables = append(tables, tableName)
		case "VIEW":
			views = append(views, tableName)
		}
	}
	sort.Strings(tables)
	sort.Strings(views)
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return
}

// TruncateAll truncates all tables in the database.
func TruncateAll(ctx context.Context, db *sql.DB) error {
	tables, _, err := ListTables(ctx, db)
	if err != nil {
		return err
	}

	// we run all queries in the same database session.
	// because we need to change some session system variables.
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// disable foreign key checks during the truncation.
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1")

	for _, tableName := range tables {
		if _, err := conn.ExecContext(ctx, "TRUNCATE TABLE "+util.Backquote(tableName)); err != nil {
			return err
		}
	}

	return nil
}

// DropAll drops all tables in the database.
func DropAll(ctx context.Context, db *sql.DB) error {
	tables, views, err := ListTables(ctx, db)
	if err != nil {
		return err
	}

	// we run all queries in the same database session.
	// because we need to change some session system variables.
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// disable foreign key checks during the dropping.
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return err
	}
	defer conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1")

	if len(views) > 0 {
		for i := range views {
			views[i] = util.Backquote(views[i])
		}

		if _, err := conn.ExecContext(ctx, "DROP VIEW "+strings.Join(views, ", ")); err != nil {
			return err
		}
	}

	if len(tables) > 0 {
		for i := range tables {
			tables[i] = util.Backquote(tables[i])
		}

		if _, err := conn.ExecContext(ctx, "DROP TABLE "+strings.Join(tables, ", ")); err != nil {
			return err
		}
	}

	return nil
}
