package diff_test

import (
	"bytes"
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy/diff"
	"github.com/shogo82148/schemalex-deploy/internal/database"
	"github.com/shogo82148/schemalex-deploy/internal/util"
)

type Spec struct {
	Name   string
	Tests  []string
	Before []string
	After  []string
	Expect []string
}

var specs = []Spec{
	{
		Name: "drop table",
		Before: []string{
			"CREATE TABLE `hoge` ( `id` integer not null )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		Expect: []string{
			"DROP TABLE `hoge`",
		},
	},
	{
		Name: "create table",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `id` INTEGER NOT NULL ) ENGINE=InnoDB DEFAULT CHARACTER SET utf8mb4 COMMENT 'table comment'",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		Expect: []string{
			"CREATE TABLE `hoge` (\n`id` INT (11) NOT NULL\n) ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4, COMMENT = 'table comment'",
		},
	},
	{
		Name: "drop column",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `c` VARCHAR (20) NOT NULL DEFAULT 'xxx' )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` DROP COLUMN `c`",
		},
	},
	{
		Name: "add columns (after)",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER NOT NULL, `c` INTEGER NOT NULL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` " +
				"ADD COLUMN `a` INT (11) NOT NULL AFTER `id`, " +
				"ADD COLUMN `b` INT (11) NOT NULL AFTER `a`, " +
				"ADD COLUMN `c` INT (11) NOT NULL AFTER `b`",
		},
	},
	{
		Name: "add columns (first)",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `a` INTEGER NOT NULL, `b` INTEGER NOT NULL, `c` INTEGER NOT NULL, `id` INTEGER NOT NULL)",
		},
		Expect: []string{
			"ALTER TABLE `fuga` " +
				"ADD COLUMN `a` INT (11) NOT NULL FIRST, " +
				"ADD COLUMN `b` INT (11) NOT NULL AFTER `a`, " +
				"ADD COLUMN `c` INT (11) NOT NULL AFTER `b`",
		},
	},
	{
		Name: "add columns (last)",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `c` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER NOT NULL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` " +
				"ADD COLUMN `c` INT (11) NOT NULL AFTER `id`, " +
				"ADD COLUMN `a` INT (11) NOT NULL AFTER `c`, " +
				"ADD COLUMN `b` INT (11) NOT NULL AFTER `a`",
		},
	},
	{
		Name: "change column",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` BIGINT NOT NULL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` CHANGE COLUMN `id` `id` BIGINT (20) NOT NULL",
		},
	},
	{
		Name: "change column with comment",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL COMMENT 'fuga is good' )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` CHANGE COLUMN `id` `id` INT (11) NOT NULL COMMENT 'fuga is good'",
		},
	},
	{
		Name: "drop primary key",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, PRIMARY KEY (`id`) )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` DROP PRIMARY KEY",
		},
	},
	{
		Name: "add primary key",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, PRIMARY KEY (`id`) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` ADD PRIMARY KEY (`id`)",
		},
	},
	{
		Name: "drop unique key",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, CONSTRAINT `symbol` UNIQUE KEY `uniq_id` USING BTREE (`id`) )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` DROP INDEX `uniq_id`",
		},
	},
	{
		Name: "add unique key",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, CONSTRAINT `symbol` UNIQUE KEY `uniq_id` USING BTREE (`id`) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` ADD CONSTRAINT `symbol` UNIQUE INDEX `uniq_id` USING BTREE (`id`)",
		},
	},
	{
		Name: "not change index",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, CONSTRAINT `symbol` UNIQUE KEY `uniq_id` USING BTREE (`id`) )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, CONSTRAINT `symbol` UNIQUE KEY `uniq_id` USING BTREE (`id`) )",
		},
		Expect: []string{},
	},
	{
		Name: "not change FOREIGN KEY",
		Before: []string{
			"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`), `fid` INTEGER NOT NULL, FOREIGN KEY fk (fid) REFERENCES f (id) )",
		},
		After: []string{
			"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`), `fid` INTEGER NOT NULL, FOREIGN KEY fk (fid) REFERENCES f (id) )",
		},
		Expect: []string{},
	},
	{
		Name: "change CONSTRAINT symbol naml",
		Before: []string{
			"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, `fid` INTEGER NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `fsym` FOREIGN KEY (`fid`) REFERENCES f (`id`) )",
		},
		After: []string{
			"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, `fid` INTEGER NOT NULL, PRIMARY KEY (`id`), CONSTRAINT `ksym` FOREIGN KEY (`fid`) REFERENCES f (`id`) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` " +
				"DROP FOREIGN KEY `fsym`, " +
				"DROP INDEX `fsym`, " +
				"ADD INDEX `ksym` (`fid`), " +
				"ADD CONSTRAINT `ksym` FOREIGN KEY (`fid`) REFERENCES `f` (`id`)",
		},
	},
	{
		Name: "remove FOREIGN KEY",
		Before: []string{
			"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `fid` INTEGER NOT NULL, CONSTRAINT `fk` FOREIGN KEY (fid) REFERENCES f (id) )",
		},
		After: []string{
			"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `fid` INTEGER NOT NULL, INDEX fid (fid) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` " +
				"DROP FOREIGN KEY `fk`, " +
				"DROP INDEX `fk`, " +
				"ADD INDEX `fid` (`fid`)",
		},
	},
	{
		Name: "full text key",
		Tests: []string{
			"CREATE TABLE `hoge` ( `txt` TEXT, FULLTEXT INDEX `ft_idx` (`txt`) WITH PARSER `ngram`)",
		},
		Before: []string{
			"CREATE TABLE `hoge` ( `txt` TEXT )",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `txt` TEXT, FULLTEXT INDEX `ft_idx` (`txt`) WITH PARSER `ngram`)",
		},
		Expect: []string{
			"ALTER TABLE `hoge` ADD FULLTEXT INDEX `ft_idx` (`txt`) WITH PARSER `ngram`",
		},
	},
	{
		Name: "drop fulltext key",
		Tests: []string{
			"CREATE TABLE `hoge` ( `txt` TEXT, FULLTEXT INDEX `ft_idx` (`txt`) WITH PARSER `ngram`)",
		},
		Before: []string{
			"CREATE TABLE `hoge` ( `txt` TEXT, FULLTEXT INDEX `ft_idx` (`txt`) WITH PARSER `ngram`)",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `txt` TEXT )",
		},
		Expect: []string{
			"ALTER TABLE `hoge` DROP INDEX `ft_idx`",
		},
	},
	{
		Name: "multi modify",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `aid` INTEGER NOT NULL, `bid` INTEGER NOT NULL, INDEX `ab` (`aid`, `bid`) )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `aid` INTEGER NOT NULL, `cid` INTEGER NOT NULL, INDEX `ac` (`aid`, `cid`) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` " +
				"DROP INDEX `ab`, " +
				"DROP COLUMN `bid`, " +
				"ADD COLUMN `cid` INT (11) NOT NULL AFTER `aid`, " +
				"ADD INDEX `ac` (`aid`, `cid`)",
		},
	},
	{
		Name: "not change to query what generated by show create table",
		// human input
		Before: []string{`
CREATE TABLE bar ( id INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (id) )`, `
create table foo (
id int not null AUTO_INCREMENT PRIMARY KEY,
tinyint_s tinyint,
tinyint_u tinyint unsigned,
smallint_s smallint,
smallint_u smallint unsigned,
mediumint_s mediumint,
mediumint_u mediumint unsigned,
int_s int comment 'this is singed int nullable',
int_u int unsigned,
integer_s integer null default null,
integer_u integer unsigned null,
bigint_s bigint UNIQUE KEY,
bigint_u bigint unsigned,
float_s float,
float_u float unsigned,
double_s double,
double_u double unsigned,
decimal_s decimal,
decimal_u decimal unsigned,
varchar_n varchar (10) null,
varchar_nn varchar (10) not null,
text_n text,
text_nn text not null,
blob_n blob,
blob_nn blob,
int_sd int default 0,
int_ud int unsigned default 0,
CONSTRAINT bar_fk FOREIGN KEY (integers) REFERENCES bar (id),
INDEX foo_idx (ints)
)`},
		// show create table foo
		After: []string{`
CREATE TABLE bar (
id int NOT NULL AUTO_INCREMENT,
PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`, `
CREATE TABLE foo (
id int(11) NOT NULL AUTO_INCREMENT,
tinyint_s tinyint(4) DEFAULT NULL,
tinyint_u tinyint(3) unsigned DEFAULT NULL,
smallint_s smallint(6) DEFAULT NULL,
smallint_u smallint(5) unsigned DEFAULT NULL,
mediumint_s mediumint(9) DEFAULT NULL,
mediumint_u mediumint(8) unsigned DEFAULT NULL,
int_s int(11) DEFAULT NULL COMMENT 'this is singed int nullable',
int_u int(10) unsigned DEFAULT NULL,
integer_s int(11) DEFAULT NULL,
integer_u int(10) unsigned DEFAULT NULL,
bigint_s bigint(20) DEFAULT NULL,
bigint_u bigint(20) unsigned DEFAULT NULL,
float_s float DEFAULT NULL,
float_u float unsigned DEFAULT NULL,
double_s double DEFAULT NULL,
double_u double unsigned DEFAULT NULL,
decimal_s decimal(10,0) DEFAULT NULL,
decimal_u decimal(10,0) unsigned DEFAULT NULL,
varchar_n varchar(10) DEFAULT NULL,
varchar_nn varchar(10) NOT NULL,
text_n text,
text_nn text NOT NULL,
blob_n blob,
blob_nn blob,
int_sd int(11) DEFAULT '0',
int_ud int(10) unsigned DEFAULT '0',
PRIMARY KEY (id),
UNIQUE KEY bigint_s (bigint_s),
KEY bar_fk (integers),
KEY foo_idx (ints),
CONSTRAINT bar_fk FOREIGN KEY (integers) REFERENCES bar (id)
)`},
		Expect: []string{},
	},
}

func joinQueries(queries []string) string {
	var buf strings.Builder
	for _, q := range queries {
		buf.WriteString(q)
		buf.WriteString(";\n")
	}
	return buf.String()
}

func TestDiff(t *testing.T) {
	var buf bytes.Buffer
	for _, spec := range specs {
		t.Run(spec.Name, func(t *testing.T) {
			buf.Reset()
			before := joinQueries(spec.Before)
			after := joinQueries(spec.After)
			expect := joinQueries(spec.Expect)

			err := diff.Strings(&buf, before, after)
			if err != nil {
				t.Errorf("spec %s failed: %v", spec.Name, err)
				return
			}

			actual := buf.String()
			if diff := cmp.Diff(expect, actual); diff != "" {
				t.Errorf(
					"spec %s mismatch (-want/+got)\n%s\n"+
						"before = %q\n"+
						"after  = %q",
					spec.Name, diff, spec.Before, spec.After,
				)
			}
		})
	}
}

func TestDiff_Integrated(t *testing.T) {
	database.SkipIfNoTestDatabase(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buf bytes.Buffer
	for _, spec := range specs {
		t.Run(spec.Name, func(t *testing.T) {
			if len(spec.Tests) > 0 {
				test, cleanup := database.SetupTestDB()
				defer cleanup()
				for _, q := range spec.Tests {
					if _, err := test.ExecContext(ctx, q); err != nil {
						t.Skipf("skip because the error: %v", err)
						break
					}
				}
			}

			buf.Reset()
			before := joinQueries(spec.Before)
			after := joinQueries(spec.After)

			err := diff.Strings(&buf, before, after)
			if err != nil {
				t.Errorf("spec %s failed: %v", spec.Name, err)
				return
			}
			queries := strings.Split(buf.String(), ";\n")

			// apply the before, and then apply the diff
			db1, cleanup := database.SetupTestDB()
			defer cleanup()
			for _, q := range spec.Before {
				if q == "" {
					continue
				}
				if _, err := db1.ExecContext(ctx, q); err != nil {
					t.Fatal(err)
				}
			}
			for _, q := range queries {
				if q == "" {
					continue
				}
				if _, err := db1.ExecContext(ctx, q); err != nil {
					t.Fatal(err)
				}
			}

			// apply the after directly
			db2, cleanup := database.SetupTestDB()
			defer cleanup()
			for _, q := range spec.After {
				if q == "" {
					continue
				}
				if _, err := db2.ExecContext(ctx, q); err != nil {
					t.Fatal(err)
				}
			}

			// compare the results
			tables1, views1, err := database.ListTables(ctx, db1)
			if err != nil {
				t.Fatal(err)
			}
			tables2, views2, err := database.ListTables(ctx, db2)
			if err != nil {
				t.Fatal(err)
			}

			if diff := cmp.Diff(tables1, tables2); diff != "" {
				t.Errorf("tables are unmatched: (-migrated/+directly)\n%s", diff)
			}
			if diff := cmp.Diff(views1, views2); diff != "" {
				t.Errorf("tables are unmatched: (-migrated/+directly)\n%s", diff)
			}

			for _, tbl := range tables1 {
				var row *sql.Row
				var tmp string
				var create1, create2 string
				row = db1.QueryRowContext(ctx, "SHOW CREATE TABLE "+util.Backquote(tbl))
				if err := row.Scan(&tmp, &create1); err != nil {
					t.Fatal(err)
				}
				row = db2.QueryRowContext(ctx, "SHOW CREATE TABLE "+util.Backquote(tbl))
				if err := row.Scan(&tmp, &create2); err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(tables1, tables2); diff != "" {
					t.Errorf("table %s definition is unmatched: (-migrated/+directly)\n%s", tbl, diff)
				}
			}
		})
	}
}
