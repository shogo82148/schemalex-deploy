package deploy

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy/internal/database"
	"github.com/shogo82148/schemalex-deploy/internal/util"
)

func TestDeploy(t *testing.T) {
	database.SkipIfNoTestDatabase(t)

	ctx := t.Context()
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
			t.Errorf("schema mismatch (-want,+got):\n%s", diff)
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
			t.Errorf("schema mismatch (-want,+got):\n%s", diff)
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

func TestImport(t *testing.T) {
	database.SkipIfNoTestDatabase(t)

	ctx := t.Context()
	rawDB, cleanup := database.SetupTestDB()
	defer cleanup()
	db := &DB{
		db: rawDB,
	}

	t.Run("no table is detected", func(t *testing.T) {
		sqlText, err := db.LoadSchema(ctx)
		if err != nil {
			t.Fatalf("failed to empty import: %v", err)
		}
		if sqlText != "" {
			t.Errorf("want empty string \"\", but not: %q", sqlText)
		}
	})

	t.Run("after create table", func(t *testing.T) {
		tb1 := `CREATE TABLE hoge (
			id INTEGER NOT NULL AUTO_INCREMENT,
			c VARCHAR (20) NOT NULL DEFAULT "hoge",
			PRIMARY KEY (id)
		);
`
		if _, err := db.db.ExecContext(ctx, tb1); err != nil {
			t.Fatalf("failed to create `hoge` table: %v", err)
		}

		tb2 := `CREATE TABLE fuga (
			id INTEGER NOT NULL AUTO_INCREMENT,
			PRIMARY KEY (id)
		);
`
		if _, err := db.db.ExecContext(ctx, tb2); err != nil {
			t.Fatalf("failed to create `fuga` table: %v", err)
		}

		sqlText, err := db.LoadSchema(ctx)
		if err != nil {
			t.Fatalf("failed to load schema: %v", err)
		}

		if err := db.Import(ctx, sqlText); err != nil {
			t.Fatalf("failed to import: %v", err)
		}

		latest, err := getLatestVersion(ctx, db.db)
		if err != nil {
			t.Fatalf("failed to get the latest version: %v", err)
		}
		if latest.ID == 0 {
			t.Fatal("want schemalex revision exists, but not")
		}

		plan, err := db.Plan(ctx, tb1+tb2)
		if err != nil {
			t.Fatalf("failed to plan: %v", err)
		}
		if len(plan.Stmts) > 0 {
			t.Errorf("want no diff is detected, but not: %v", plan.Stmts)
		}
	})
}

// https://github.com/shogo82148/schemalex-deploy/issues/309
// MySQL rewrites the expressions of generated columns into its own form,
// e.g. `a + 1` into `(`a` + 1)`.
// It should not produce CHANGE COLUMN for equivalent expressions after importing the schema.
func TestImportGeneratedColumns(t *testing.T) {
	database.SkipIfNoTestDatabase(t)

	tests := []struct {
		name string
		ddl  string
	}{
		{
			name: "arithmetic",
			ddl: `CREATE TABLE t (
				id INTEGER NOT NULL AUTO_INCREMENT,
				a INTEGER NOT NULL,
				b INTEGER AS (a + 1),
				c INTEGER AS (b * 2) STORED,
				PRIMARY KEY (id),
				INDEX idx_b (b)
			);
`,
		},
		{
			name: "string literals",
			ddl: `CREATE TABLE t (
				s VARCHAR(10) NOT NULL,
				x VARCHAR(20) AS (concat(s, 'x')) STORED,
				y VARCHAR(20) AS (CONCAT(s, "y", ')'))
			);
`,
		},
		{
			// MariaDB adds CHECK (json_valid(...)) to JSON columns, which is not supported.
			// json_extract accepts strings, so use TEXT instead of JSON.
			// It is NOT NULL, because MariaDB shows DEFAULT NULL for nullable TEXT columns.
			name: "json functions",
			ddl: `CREATE TABLE t (
				doc TEXT NOT NULL,
				id INTEGER AS (json_unquote(json_extract(doc, '$.id')))
			);
`,
		},
		{
			// MariaDB does not support the ->> operator.
			name: "json operator",
			ddl: `CREATE TABLE t (
				doc JSON,
				id INTEGER AS (doc->>'$.id')
			);
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			rawDB, cleanup := database.SetupTestDB()
			defer cleanup()
			db := &DB{
				db: rawDB,
			}

			if _, err := db.db.ExecContext(ctx, tt.ddl); err != nil {
				t.Skipf("skip because the error: %v", err)
			}

			sqlText, err := db.LoadSchema(ctx)
			if err != nil {
				t.Fatalf("failed to load schema: %v", err)
			}
			if err := db.Import(ctx, sqlText); err != nil {
				t.Fatalf("failed to import: %v", err)
			}

			plan, err := db.Plan(ctx, tt.ddl)
			if err != nil {
				t.Fatalf("failed to plan: %v", err)
			}
			if len(plan.Stmts) > 0 {
				t.Errorf("want no diff is detected, but not: %v", plan.Stmts)
			}
		})
	}

	t.Run("changed expression", func(t *testing.T) {
		ctx := t.Context()
		rawDB, cleanup := database.SetupTestDB()
		defer cleanup()
		db := &DB{
			db: rawDB,
		}

		const before = `CREATE TABLE t (
			a INTEGER NOT NULL,
			b INTEGER AS (a + 1),
			c INTEGER AS (a + 2) STORED
		);
`
		if _, err := db.db.ExecContext(ctx, before); err != nil {
			t.Fatal(err)
		}
		sqlText, err := db.LoadSchema(ctx)
		if err != nil {
			t.Fatalf("failed to load schema: %v", err)
		}
		if err := db.Import(ctx, sqlText); err != nil {
			t.Fatalf("failed to import: %v", err)
		}

		// change only the expression of c.
		const after = `CREATE TABLE t (
			a INTEGER NOT NULL,
			b INTEGER AS (a + 1),
			c INTEGER AS (a + 3) STORED
		);
`
		plan, err := db.Plan(ctx, after)
		if err != nil {
			t.Fatalf("failed to plan: %v", err)
		}
		want := "ALTER TABLE `t` CHANGE COLUMN `c` `c` INT (11) GENERATED ALWAYS AS (a + 3) STORED"
		if len(plan.Stmts) != 1 || string(plan.Stmts[0]) != want {
			t.Errorf("want %q, got %q", want, plan.Stmts)
		}

		// deploy the plan, and then no diff is detected.
		if err := db.Deploy(ctx, plan); err != nil {
			t.Fatalf("failed to deploy: %v", err)
		}
		plan, err = db.Plan(ctx, after)
		if err != nil {
			t.Fatalf("failed to plan: %v", err)
		}
		if len(plan.Stmts) > 0 {
			t.Errorf("want no diff is detected, but not: %v", plan.Stmts)
		}
	})
}

// https://github.com/shogo82148/schemalex-deploy/issues/323
func TestPlanPartitionedTable(t *testing.T) {
	database.SkipIfNoTestDatabase(t)

	t.Run("partitioned after import", func(t *testing.T) {
		ctx := t.Context()
		rawDB, cleanup := database.SetupTestDB()
		defer cleanup()
		db := &DB{
			db: rawDB,
		}

		const before = `CREATE TABLE t (
			id INTEGER NOT NULL,
			PRIMARY KEY (id)
		);
`
		if _, err := db.db.ExecContext(ctx, before); err != nil {
			t.Fatal(err)
		}
		sqlText, err := db.LoadSchema(ctx)
		if err != nil {
			t.Fatalf("failed to load schema: %v", err)
		}
		if err := db.Import(ctx, sqlText); err != nil {
			t.Fatalf("failed to import: %v", err)
		}

		// partition the table outside of schemalex-deploy.
		if _, err := db.db.ExecContext(ctx, "ALTER TABLE t PARTITION BY HASH(id) PARTITIONS 2"); err != nil {
			t.Fatal(err)
		}

		// the changes of other tables can be deployed.
		const after = `CREATE TABLE t (
			id INTEGER NOT NULL,
			PRIMARY KEY (id)
		);
CREATE TABLE u (
			id INTEGER NOT NULL
		);
`
		plan, err := db.Plan(ctx, after)
		if err != nil {
			t.Fatalf("failed to plan: %v", err)
		}
		want := "CREATE TABLE `u` (\n  `id` INT (11) NOT NULL\n)"
		if len(plan.Stmts) != 1 || string(plan.Stmts[0]) != want {
			t.Errorf("want %q, got %q", want, plan.Stmts)
		}
	})

	t.Run("import a partitioned table", func(t *testing.T) {
		ctx := t.Context()
		rawDB, cleanup := database.SetupTestDB()
		defer cleanup()
		db := &DB{
			db: rawDB,
		}

		const before = `CREATE TABLE t (
			id INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			PRIMARY KEY (id, created_at)
		)
		PARTITION BY RANGE COLUMNS(created_at) (
			PARTITION p2025 VALUES LESS THAN ('2026-01-01'),
			PARTITION p2026 VALUES LESS THAN ('2027-01-01')
		);
`
		if _, err := db.db.ExecContext(ctx, before); err != nil {
			t.Fatal(err)
		}
		sqlText, err := db.LoadSchema(ctx)
		if err != nil {
			t.Fatalf("failed to load schema: %v", err)
		}
		if err := db.Import(ctx, sqlText); err != nil {
			t.Fatalf("failed to import: %v", err)
		}

		// SHOW CREATE TABLE rewrites the partitioning clause, but no diff is detected.
		plan, err := db.Plan(ctx, before)
		if err != nil {
			t.Fatalf("failed to plan: %v", err)
		}
		if len(plan.Stmts) > 0 {
			t.Errorf("want no diff is detected, but not: %v", plan.Stmts)
		}

		// add a partition outside of schemalex-deploy, and add a column.
		if _, err := db.db.ExecContext(ctx, "ALTER TABLE t ADD PARTITION (PARTITION p2027 VALUES LESS THAN ('2028-01-01'))"); err != nil {
			t.Fatal(err)
		}
		const after = `CREATE TABLE t (
			id INTEGER NOT NULL,
			created_at DATETIME NOT NULL,
			a INTEGER NOT NULL,
			PRIMARY KEY (id, created_at)
		)
		PARTITION BY RANGE COLUMNS(created_at) (
			PARTITION p2025 VALUES LESS THAN ('2026-01-01'),
			PARTITION p2026 VALUES LESS THAN ('2027-01-01')
		);
`
		plan, err = db.Plan(ctx, after)
		if err != nil {
			t.Fatalf("failed to plan: %v", err)
		}
		want := "ALTER TABLE `t` ADD COLUMN `a` INT (11) NOT NULL AFTER `created_at`"
		if len(plan.Stmts) != 1 || string(plan.Stmts[0]) != want {
			t.Errorf("want %q, got %q", want, plan.Stmts)
		}
		if err := db.Deploy(ctx, plan); err != nil {
			t.Fatalf("failed to deploy: %v", err)
		}

		// the partition added outside of schemalex-deploy remains.
		var name, ddl string
		if err := db.db.QueryRowContext(ctx, "SHOW CREATE TABLE t").Scan(&name, &ddl); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(ddl, "p2027") {
			t.Errorf("want the partition p2027 remains, but not: %s", ddl)
		}
	})
}
