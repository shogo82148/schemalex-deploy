package deploy

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy/internal/database"
	"github.com/shogo82148/schemalex-deploy/internal/util"
)

func TestDeploy(t *testing.T) {
	database.SkipIfNoTestDatabase(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rawDB, cleanup := database.SetupTestDB()
	defer cleanup()
	db := &DB{
		db: rawDB,
	}

	t.Run("first deployment", func(t *testing.T) {
		const sql = `CREATE TABLE hoge (
			id INTEGER NOT NULL AUTO_INCREMENT,
			PRIMARY KEY (id)
		);`

		plan, err := db.Plan(ctx, sql)
		if err != nil {
			t.Fatalf("failed to plan: %v", err)
		}

		if err := db.Deploy(ctx, plan); err != nil {
			t.Fatalf("failed to deploy: %v", err)
		}

		hoge, err := showColumns(ctx, db.db, "hoge")
		if err != nil {
			t.Fatal(err)
		}
		if len(hoge) != 1 {
			t.Errorf("want `hoge` has one column, but %d columns", len(hoge))
		}

		latest, err := getLatestVersion(ctx, db.db)
		if err != nil {
			t.Fatalf("failed to get the latest version: %v", err)
		}
		if diff := cmp.Diff(sql, latest.SQLText); diff != "" {
			t.Errorf("schema missmatch (-want,+got):\n%s", diff)
		}
	})

	t.Run("after first deployment", func(t *testing.T) {
		const sql = `CREATE TABLE hoge (
			id INTEGER NOT NULL AUTO_INCREMENT,
			c VARCHAR (20) NOT NULL DEFAULT "hoge",
			PRIMARY KEY (id)
		);
		
		CREATE TABLE fuga (
			id INTEGER NOT NULL AUTO_INCREMENT,
			PRIMARY KEY (id)
		);`

		plan, err := db.Plan(ctx, sql)
		if err != nil {
			t.Fatalf("failed to plan: %v", err)
		}

		if err := db.Deploy(ctx, plan); err != nil {
			t.Fatalf("failed to deploy: %v", err)
		}

		hoge, err := showColumns(ctx, db.db, "hoge")
		if err != nil {
			t.Fatal(err)
		}
		if len(hoge) != 2 {
			t.Errorf("want `hoge` has two columns, but %d columns", len(hoge))
		}

		latest, err := getLatestVersion(ctx, db.db)
		if err != nil {
			t.Fatalf("failed to get the latest version: %v", err)
		}
		if diff := cmp.Diff(sql, latest.SQLText); diff != "" {
			t.Errorf("schema missmatch (-want,+got):\n%s", diff)
		}
	})
}

type column struct {
	Field   string
	Type    string
	Null    string
	Key     string
	Default sql.NullString
	Extra   string
}

func showColumns(ctx context.Context, db *sql.DB, table string) ([]*column, error) {
	rows, err := db.QueryContext(ctx, "SHOW COLUMNS FROM "+util.Backquote(table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []*column
	for rows.Next() {
		var c column
		err := rows.Scan(&c.Field, &c.Type, &c.Null, &c.Key, &c.Default, &c.Extra)
		if err != nil {
			return nil, err
		}
		columns = append(columns, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return columns, nil
}
