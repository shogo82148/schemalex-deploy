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
			"ALTER TABLE `fuga` ADD COLUMN `a` INT (11) NOT NULL AFTER `id`",
			"ALTER TABLE `fuga` ADD COLUMN `b` INT (11) NOT NULL AFTER `a`",
			"ALTER TABLE `fuga` ADD COLUMN `c` INT (11) NOT NULL AFTER `b`",
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
			"ALTER TABLE `fuga` ADD COLUMN `a` INT (11) NOT NULL FIRST",
			"ALTER TABLE `fuga` ADD COLUMN `b` INT (11) NOT NULL AFTER `a`",
			"ALTER TABLE `fuga` ADD COLUMN `c` INT (11) NOT NULL AFTER `b`",
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
			"ALTER TABLE `fuga` ADD COLUMN `c` INT (11) NOT NULL AFTER `id`",
			"ALTER TABLE `fuga` ADD COLUMN `a` INT (11) NOT NULL AFTER `c`",
			"ALTER TABLE `fuga` ADD COLUMN `b` INT (11) NOT NULL AFTER `a`",
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
	// 	// change CONSTRAINT symbol naml
	// 	{
	// 		Before: "CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, `fid` INTEGER NOT NULL, CONSTRAINT `fsym` FOREIGN KEY (fid) REFERENCES f (id) );",
	// 		After:  "CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, `fid` INTEGER NOT NULL, CONSTRAINT `ksym` FOREIGN KEY (fid) REFERENCES f (id) );",
	// 		Expect: "ALTER TABLE `fuga` DROP FOREIGN KEY `fsym`;\nALTER TABLE `fuga` DROP INDEX `fsym`;\nALTER TABLE `fuga` ADD INDEX `ksym` (`fid`);\nALTER TABLE `fuga` ADD CONSTRAINT `ksym` FOREIGN KEY (`fid`) REFERENCES `f` (`id`);\n",
	// 	},
	// FIX ME!
	// {
	// 	Name: "remove FOREIGN KEY",
	// 	Before: []string{
	// 		"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
	// 		"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `fid` INTEGER NOT NULL, CONSTRAINT `fk` FOREIGN KEY (fid) REFERENCES f (id) )",
	// 	},
	// 	After: []string{
	// 		"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
	// 		"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `fid` INTEGER NOT NULL, INDEX fid (fid) )",
	// 	},
	// 	Expect: []string{
	// 		"ALTER TABLE `fuga` DROP FOREIGN KEY `fk`",
	// 		"ALTER TABLE `fuga` ADD INDEX `fid` (`fid`)",
	// 	},
	// },
	{
		Name: "full text key",
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
			"ALTER TABLE `fuga` DROP INDEX `ab`",
			"ALTER TABLE `fuga` DROP COLUMN `bid`",
			"ALTER TABLE `fuga` ADD COLUMN `cid` INT (11) NOT NULL AFTER `aid`",
			"ALTER TABLE `fuga` ADD INDEX `ac` (`aid`, `cid`)",
		},
	},
	{
		Name: "not change to query what generated by show create table",
		// human input
		Before: []string{`
CREATE TABLE bar ( id INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (id) )`, `
create table foo (
id int not null AUTO_INCREMENT PRIMARY KEY,
tinyints tinyint,
tinyintu tinyint unsigned,
smallints smallint,
smallintu smallint unsigned,
mediumints mediumint,
mediumintu mediumint unsigned,
ints int comment 'this is sined int nullable',
intu int unsigned,
integers integer null default null,
integeru integer unsigned null,
bigins bigint UNIQUE KEY,
bigintu bigint unsigned,
floats float,
floaru float unsigned,
doubles double,
doubleu double unsigned,
decimals decimal,
decimalu decimal unsigned,
varcharn varchar (10) null,
varcharnn varchar (10) not null,
textn text,
textnn text not null,
blobn blob,
blobnn blob,
intsd int default 0,
intud int unsigned default 0,
CONSTRAINT bar_fk FOREIGN KEY (integers) REFERENCES bar (id),
INDEX foo_idx (ints)
)`},
		// show create table foo
		After: []string{`
CREATE TABLE bar (
id int NOT NULL AUTO_INCREMENT,
PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`, `
CREATE TABLE foo (
id int(11) NOT NULL AUTO_INCREMENT,
tinyints tinyint(4) DEFAULT NULL,
tinyintu tinyint(3) unsigned DEFAULT NULL,
smallints smallint(6) DEFAULT NULL,
smallintu smallint(5) unsigned DEFAULT NULL,
mediumints mediumint(9) DEFAULT NULL,
mediumintu mediumint(8) unsigned DEFAULT NULL,
ints int(11) DEFAULT NULL COMMENT 'this is sined int nullable',
intu int(10) unsigned DEFAULT NULL,
integers int(11) DEFAULT NULL,
integeru int(10) unsigned DEFAULT NULL,
bigins bigint(20) DEFAULT NULL,
bigintu bigint(20) unsigned DEFAULT NULL,
floats float DEFAULT NULL,
floaru float unsigned DEFAULT NULL,
doubles double DEFAULT NULL,
doubleu double unsigned DEFAULT NULL,
decimals decimal(10,0) DEFAULT NULL,
decimalu decimal(10,0) unsigned DEFAULT NULL,
varcharn varchar(10) DEFAULT NULL,
varcharnn varchar(10) NOT NULL,
textn text,
textnn text NOT NULL,
blobn blob,
blobnn blob,
intsd int(11) DEFAULT '0',
intud int(10) unsigned DEFAULT '0',
PRIMARY KEY (id),
UNIQUE KEY bigins (bigins),
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
					"spec %s missmatch (-want/+got)\n%s\n"+
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
