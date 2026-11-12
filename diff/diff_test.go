package diff_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy/diff"
	"github.com/shogo82148/schemalex-deploy/internal/database"
	"github.com/shogo82148/schemalex-deploy/internal/util"
	"github.com/shogo82148/schemalex-deploy/model"
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
		Name: "add generated column",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER GENERATED ALWAYS AS (`a` + 1) STORED, `c` INTEGER AS (`a` * 2) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` ADD COLUMN `b` INT (11) GENERATED ALWAYS AS (`a` + 1) STORED AFTER `a`, ADD COLUMN `c` INT (11) GENERATED ALWAYS AS (`a` * 2) VIRTUAL AFTER `b`",
		},
	},
	{
		Name: "change generated column expression",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER GENERATED ALWAYS AS (`a` + 1) VIRTUAL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER GENERATED ALWAYS AS (`a` + 2) VIRTUAL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` CHANGE COLUMN `b` `b` INT (11) GENERATED ALWAYS AS (`a` + 2) VIRTUAL",
		},
	},
	{
		Name: "generated column with redundant parentheses",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER GENERATED ALWAYS AS ((`a` + 1)) VIRTUAL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER AS (`a` + 1) )",
		},
		Expect: []string{},
	},
	{
		Name: "generated column with redundant parentheses and quoted parentheses",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` VARCHAR(16) GENERATED ALWAYS AS ((concat(`a`, ')'))) VIRTUAL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` VARCHAR(16) AS (concat(`a`, ')')) )",
		},
		Expect: []string{},
	},
	{
		// MySQL cannot change VIRTUAL <-> STORED by CHANGE COLUMN.
		// the indexes on the column are kept when it is dropped and added in the same ALTER TABLE.
		Name: "change generated column from virtual to stored",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER AS (`a` + 1) VIRTUAL, `c` INTEGER NOT NULL, INDEX `idx_b` (`b`) )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER AS (`a` + 1) STORED, `c` INTEGER NOT NULL, INDEX `idx_b` (`b`) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` DROP COLUMN `b`, ADD COLUMN `b` INT (11) GENERATED ALWAYS AS (`a` + 1) STORED AFTER `a`",
		},
	},
	{
		Name: "change generated column from stored to virtual",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER AS (`a` + 1) STORED, INDEX `idx_ab` (`a`, `b`) )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER AS (`a` + 2) VIRTUAL, INDEX `idx_ab` (`a`, `b`) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` DROP COLUMN `b`, ADD COLUMN `b` INT (11) GENERATED ALWAYS AS (`a` + 2) VIRTUAL AFTER `a`",
		},
	},
	{
		Name: "change non-generated column to virtual generated column",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER AS (`a` + 1) VIRTUAL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` DROP COLUMN `b`, ADD COLUMN `b` INT (11) GENERATED ALWAYS AS (`a` + 1) VIRTUAL AFTER `a`",
		},
	},
	{
		// MySQL can change non-generated <-> STORED by CHANGE COLUMN.
		Name: "change non-generated column to stored generated column",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER AS (`a` + 1) STORED )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` CHANGE COLUMN `b` `b` INT (11) GENERATED ALWAYS AS (`a` + 1) STORED",
		},
	},
	{
		Name: "change stored generated column to non-generated column",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER AS (`a` + 1) STORED )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` CHANGE COLUMN `b` `b` INT (11) DEFAULT NULL",
		},
	},
	{
		Name: "change generated column from virtual to stored after a new column",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER AS (`a` + 1) VIRTUAL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `x` INTEGER NOT NULL, `b` INTEGER AS (`a` + 1) STORED, `y` INTEGER NOT NULL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` DROP COLUMN `b`, ADD COLUMN `x` INT (11) NOT NULL AFTER `a`, ADD COLUMN `b` INT (11) GENERATED ALWAYS AS (`a` + 1) STORED AFTER `x`, ADD COLUMN `y` INT (11) NOT NULL AFTER `b`",
		},
	},
	{
		Name: "change first generated column from virtual to stored",
		Before: []string{
			"CREATE TABLE `fuga` ( `b` INTEGER AS (1) VIRTUAL, `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `b` INTEGER AS (1) STORED, `id` INTEGER NOT NULL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` DROP COLUMN `b`, ADD COLUMN `b` INT (11) GENERATED ALWAYS AS (1) STORED FIRST",
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
		Name:   "create FOREIGN KEY",
		Before: []string{},
		After: []string{
			"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`), `fid` INTEGER NOT NULL, CONSTRAINT `fsym` FOREIGN KEY `fk` (fid) REFERENCES f (id) )",
		},
		Expect: []string{
			"CREATE TABLE `f` (\n`id` INT (11) NOT NULL AUTO_INCREMENT,\nPRIMARY KEY (`id`)\n)",
			"CREATE TABLE `fuga` (\n`id` INT (11) NOT NULL AUTO_INCREMENT,\n`fid` INT (11) NOT NULL,\nPRIMARY KEY (`id`),\n" +
				"INDEX `fsym` (`fid`),\nCONSTRAINT `fsym` FOREIGN KEY `fk` (`fid`) REFERENCES `f` (`id`)\n)",
		},
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
		Name: "change CONSTRAINT symbol name",
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

	// geometry types
	{
		Name: "add columns with srid",
		Tests: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `point` GEOMETRY NOT NULL )",
		},
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `id` INTEGER NOT NULL, `point` GEOMETRY NOT NULL )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		Expect: []string{
			"CREATE TABLE `hoge` (\n" +
				"`id` INT (11) NOT NULL,\n" +
				"`point` GEOMETRY NOT NULL\n)",
		},
	},
	{
		Name: "add columns with srid",
		Tests: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `point` POINT NOT NULL SRID 4326 )",
		},
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `id` INTEGER NOT NULL, `point` POINT NOT NULL SRID 4326 )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		Expect: []string{
			"CREATE TABLE `hoge` (\n" +
				"`id` INT (11) NOT NULL,\n" +
				"`point` POINT NOT NULL SRID 4326\n)",
		},
	},
	{
		Name: "change columns with srid",
		Tests: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `point` POINT NOT NULL SRID 4326 )",
		},
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `point` POINT NOT NULL )",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `point` POINT NOT NULL SRID 4326 )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` CHANGE COLUMN `point` `point` POINT NOT NULL SRID 4326",
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
CONSTRAINT bar_fk FOREIGN KEY (integer_s) REFERENCES bar (id),
INDEX foo_idx (int_s)
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
KEY bar_fk (integer_s),
KEY foo_idx (int_s),
CONSTRAINT bar_fk FOREIGN KEY (integer_s) REFERENCES bar (id)
)`},
		Expect: []string{},
	},
	{
		Name: "add an invisible index",
		Tests: []string{
			// MariaDB does not support invisible indexes.
			"CREATE TABLE `hoge` ( `a` INTEGER, INDEX `idx_a` (`a`) INVISIBLE )",
		},
		Before: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER )",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, INDEX `idx_a` (`a`) INVISIBLE )",
		},
		Expect: []string{
			"ALTER TABLE `hoge` ADD INDEX `idx_a` (`a`) INVISIBLE",
		},
	},
	{
		Name: "make an index invisible",
		Tests: []string{
			// MariaDB does not support invisible indexes.
			"CREATE TABLE `hoge` ( `a` INTEGER, INDEX `idx_a` (`a`) INVISIBLE )",
		},
		Before: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, INDEX `idx_a` (`a`) )",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, INDEX `idx_a` (`a`) INVISIBLE )",
		},
		Expect: []string{
			"ALTER TABLE `hoge` ALTER INDEX `idx_a` INVISIBLE",
		},
	},
	{
		Name: "make an index visible",
		Tests: []string{
			// MariaDB does not support invisible indexes.
			"CREATE TABLE `hoge` ( `a` INTEGER, INDEX `idx_a` (`a`) INVISIBLE )",
		},
		Before: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, `b` INTEGER, UNIQUE INDEX `uniq_a` (`a`) INVISIBLE, INDEX `idx_b` (`b`) INVISIBLE )",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, `b` INTEGER, UNIQUE INDEX `uniq_a` (`a`), INDEX `idx_b` (`b`) VISIBLE )",
		},
		Expect: []string{
			"ALTER TABLE `hoge` ALTER INDEX `idx_b` VISIBLE, ALTER INDEX `uniq_a` VISIBLE",
		},
	},
	{
		Name: "change the columns and the visibility of an index",
		Tests: []string{
			// MariaDB does not support invisible indexes.
			"CREATE TABLE `hoge` ( `a` INTEGER, INDEX `idx_a` (`a`) INVISIBLE )",
		},
		Before: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, `b` INTEGER, INDEX `idx` (`a`) )",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, `b` INTEGER, INDEX `idx` (`a`, `b`) INVISIBLE )",
		},
		Expect: []string{
			"ALTER TABLE `hoge` DROP INDEX `idx`, ADD INDEX `idx` (`a`, `b`) INVISIBLE",
		},
	},
	{
		Name: "add an invisible column",
		Before: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER )",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, `b` INTEGER INVISIBLE )",
		},
		Expect: []string{
			"ALTER TABLE `hoge` ADD COLUMN `b` INT (11) DEFAULT NULL INVISIBLE AFTER `a`",
		},
	},
	{
		Name: "make a column invisible",
		Before: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, `b` INTEGER )",
		},
		After: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, `b` INTEGER INVISIBLE )",
		},
		Expect: []string{
			"ALTER TABLE `hoge` CHANGE COLUMN `b` `b` INT (11) DEFAULT NULL INVISIBLE",
		},
	},
	{
		Name: "make a column visible",
		Before: []string{
			"CREATE TABLE `hoge` ( `a` INTEGER, `b` INTEGER INVISIBLE )",
		},
		After: []string{
			// MariaDB does not support the VISIBLE keyword for columns.
			"CREATE TABLE `hoge` ( `a` INTEGER, `b` INTEGER )",
		},
		Expect: []string{
			"ALTER TABLE `hoge` CHANGE COLUMN `b` `b` INT (11) DEFAULT NULL",
		},
	},

	// https://github.com/shogo82148/schemalex-deploy/issues/323
	{
		Name:   "create a partitioned table",
		Before: []string{},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL ) PARTITION BY HASH (`id`) PARTITIONS 2",
		},
		Expect: []string{
			"CREATE TABLE `fuga` (\n`id` INT (11) NOT NULL\n)\nPARTITION BY HASH (`id`) PARTITIONS 2",
		},
	},
	{
		Name: "add a column to a partitioned table",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL ) PARTITION BY HASH (`id`) PARTITIONS 2",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL ) PARTITION BY HASH (`id`) PARTITIONS 2",
		},
		Expect: []string{
			"ALTER TABLE `fuga` ADD COLUMN `a` INT (11) NOT NULL AFTER `id`",
		},
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

func TestDiff_VirtualToNonGenerated(t *testing.T) {
	// MySQL cannot change a VIRTUAL generated column into a non-generated column by CHANGE COLUMN,
	// and dropping and adding the column again loses its values.
	before := "CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER AS (`a` + 1) VIRTUAL );"
	after := "CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `a` INTEGER NOT NULL, `b` INTEGER );"

	var buf bytes.Buffer
	err := diff.Strings(&buf, before, after)
	if err == nil {
		t.Fatalf("want error, got nil: %s", buf.String())
	}
	if !strings.Contains(err.Error(), "virtual generated column") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDiff_Integrated(t *testing.T) {
	database.SkipIfNoTestDatabase(t)

	ctx := t.Context()

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
				if diff := cmp.Diff(create1, create2); diff != "" {
					t.Errorf("table %s definition is unmatched: (-migrated/+directly)\n%s", tbl, diff)
				}
			}
		})
	}
}

type AutoNamedSpec struct {
	Name    string
	Tests   []string
	Before  []string
	Current []string
	After   []string
	Expect  []string
}

var autoSpecs = []AutoNamedSpec{
	{
		Name: "drop anonymous unique key",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, UNIQUE KEY (`id`) )",
		},
		Current: []string{"CREATE TABLE `fuga` (" +
			"`id` int NOT NULL," +
			"UNIQUE KEY `id` (`id`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` DROP INDEX `id`",
		},
	},
	{
		Name: "remove anonymous FOREIGN KEY",
		Before: []string{
			"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `fid` INTEGER NOT NULL, FOREIGN KEY (fid) REFERENCES f (id) )",
		},
		Current: []string{
			"CREATE TABLE `f` ( `id` int NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
			"CREATE TABLE `fuga` (" +
				"`id` int NOT NULL," +
				"`fid` int NOT NULL," +
				"KEY `fid` (`fid`)," +
				"CONSTRAINT `fuga_ibfk_1` FOREIGN KEY (`fid`) REFERENCES `f` (`id`)" +
				") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
		},
		After: []string{
			"CREATE TABLE `f` ( `id` INTEGER NOT NULL AUTO_INCREMENT, PRIMARY KEY (`id`) )",
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, `fid` INTEGER NOT NULL, INDEX fid (fid) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` " +
				"DROP FOREIGN KEY `fuga_ibfk_1`, " +
				"ADD INDEX `fid` (`fid`)",
		},
	},
	{
		Name: "make an anonymous index invisible",
		Tests: []string{
			// MariaDB does not support invisible indexes.
			"CREATE TABLE `fuga` ( `id` INTEGER, INDEX (`id`) INVISIBLE )",
		},
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, INDEX (`id`) )",
		},
		Current: []string{"CREATE TABLE `fuga` (" +
			"`id` int NOT NULL," +
			"KEY `id` (`id`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, INDEX (`id`) INVISIBLE )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` ALTER INDEX `id` INVISIBLE",
		},
	},

	// https://github.com/shogo82148/schemalex-deploy/issues/322
	// MySQL allows duplicate indexes with a warning, and names them `id`, `id_2`, ...
	{
		Name: "add a duplicate anonymous index",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, INDEX (`id`) )",
		},
		Current: []string{"CREATE TABLE `fuga` (" +
			"`id` int NOT NULL," +
			"KEY `id` (`id`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, INDEX (`id`), INDEX (`id`) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` ADD INDEX (`id`)",
		},
	},
	{
		Name: "drop a duplicate anonymous index",
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, INDEX (`id`), INDEX (`id`), INDEX (`id`) )",
		},
		Current: []string{"CREATE TABLE `fuga` (" +
			"`id` int NOT NULL," +
			"KEY `id` (`id`)," +
			"KEY `id_2` (`id`)," +
			"KEY `id_3` (`id`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, INDEX (`id`) )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` DROP INDEX `id_2`, DROP INDEX `id_3`",
		},
	},
	{
		Name: "make a duplicate anonymous index invisible",
		Tests: []string{
			// MariaDB does not support invisible indexes.
			"CREATE TABLE `fuga` ( `id` INTEGER, INDEX (`id`) INVISIBLE )",
		},
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, INDEX (`id`) INVISIBLE, INDEX (`id`) )",
		},
		Current: []string{"CREATE TABLE `fuga` (" +
			"`id` int NOT NULL," +
			"KEY `id` (`id`) /*!80000 INVISIBLE */," +
			"KEY `id_2` (`id`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, INDEX (`id`), INDEX (`id`) INVISIBLE )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` ALTER INDEX `id` VISIBLE, ALTER INDEX `id_2` INVISIBLE",
		},
	},
	{
		// the indexes must be added in order, because MySQL names them in the order they are added.
		// the 10th one must not be added before the 2nd one.
		Name: "add ten duplicate anonymous indexes",
		Tests: []string{
			// MariaDB does not support invisible indexes.
			"CREATE TABLE `fuga` ( `id` INTEGER, INDEX (`id`) INVISIBLE )",
		},
		Before: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, INDEX (`id`) )",
		},
		Current: []string{"CREATE TABLE `fuga` (" +
			"`id` int NOT NULL," +
			"KEY `id` (`id`)" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
		},
		After: []string{
			"CREATE TABLE `fuga` ( `id` INTEGER NOT NULL, INDEX (`id`), INDEX (`id`) INVISIBLE" +
				strings.Repeat(", INDEX (`id`)", 8) + " )",
		},
		Expect: []string{
			"ALTER TABLE `fuga` ADD INDEX (`id`) INVISIBLE" +
				strings.Repeat(", ADD INDEX (`id`)", 8),
		},
	},
}

func TestDiffWithAutoNamedObjects(t *testing.T) {
	var buf bytes.Buffer
	for _, spec := range autoSpecs {
		t.Run(spec.Name, func(t *testing.T) {
			buf.Reset()
			before := joinQueries(spec.Before)
			after := joinQueries(spec.After)
			current := joinQueries(spec.Current)
			expect := joinQueries(spec.Expect)

			err := diff.Strings(&buf, before, after, diff.WithCurrentSchema(current))
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

func TestDiffWithAutoNamedObjects_Integrated(t *testing.T) {
	database.SkipIfNoTestDatabase(t)

	ctx := t.Context()

	var buf bytes.Buffer
	for _, spec := range autoSpecs {
		t.Run(spec.Name, func(t *testing.T) {
			// check the MySQL support the feature.
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

			// apply the before
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

			// calculate the difference
			before := joinQueries(spec.Before)
			after := joinQueries(spec.After)
			current, err := LoadSchema(ctx, db1)
			if err != nil {
				t.Fatal(err)
			}
			err = diff.Strings(&buf, before, after, diff.WithCurrentSchema(current))
			if err != nil {
				t.Errorf("spec %s failed: %v", spec.Name, err)
				return
			}
			queries := strings.SplitSeq(buf.String(), ";\n")

			// apply
			for q := range queries {
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
				if diff := cmp.Diff(create1, create2); diff != "" {
					t.Errorf("table %s definition is unmatched: (-migrated/+directly)\n%s", tbl, diff)
				}
			}
		})
	}
}

// LoadSchema loads existing table schemas from running database.
func LoadSchema(ctx context.Context, db *sql.DB) (string, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		ReadOnly: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Commit()

	tables, err := showTables(ctx, tx)
	if err != nil {
		return "", err
	}
	statements := make([]string, 0, len(tables))
	for _, tbl := range tables {
		row := tx.QueryRowContext(ctx, fmt.Sprintf("SHOW CREATE TABLE %s", util.Backquote(tbl)))
		var tmp, sqlText string
		if err := row.Scan(&tmp, &sqlText); err != nil {
			return "", fmt.Errorf("failed to get create table %q: %w", tbl, err)
		}

		if !strings.HasSuffix(sqlText, ";") {
			sqlText = sqlText + ";"
		}
		sqlText += "\n"

		statements = append(statements, sqlText)
	}

	return strings.Join(statements, ""), nil
}

func showTables(ctx context.Context, tx *sql.Tx) ([]string, error) {
	rows, err := tx.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("failed to get table list: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tables = append(tables, table)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("some error occurred during iteration: %w", err)
	}
	return tables, nil
}

// The changes of the partitioning are ignored,
// because SHOW CREATE TABLE rewrites the partitioning clause,
// and partitions may be added or removed outside of the schema, e.g. the partitions by date.
// https://github.com/shogo82148/schemalex-deploy/issues/323
func TestDiff_IgnorePartitionChanges(t *testing.T) {
	tests := []struct {
		name   string
		before string
		after  string
	}{
		{
			name:   "partition a table",
			before: "CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
			after:  "CREATE TABLE `fuga` ( `id` INTEGER NOT NULL ) PARTITION BY HASH (`id`) PARTITIONS 2",
		},
		{
			name:   "remove partitioning",
			before: "CREATE TABLE `fuga` ( `id` INTEGER NOT NULL ) PARTITION BY HASH (`id`) PARTITIONS 2",
			after:  "CREATE TABLE `fuga` ( `id` INTEGER NOT NULL )",
		},
		{
			name: "rewritten by SHOW CREATE TABLE",
			before: "CREATE TABLE `fuga` ( `id` INTEGER NOT NULL ) " +
				"PARTITION BY RANGE (id) (PARTITION p0 VALUES LESS THAN (10), PARTITION p1 VALUES LESS THAN MAXVALUE)",
			after: "CREATE TABLE `fuga` ( `id` int NOT NULL ) ENGINE=InnoDB\n" +
				"/*!50100 PARTITION BY RANGE (`id`)\n" +
				"(PARTITION p0 VALUES LESS THAN (10) ENGINE = InnoDB,\n" +
				" PARTITION p1 VALUES LESS THAN MAXVALUE ENGINE = InnoDB) */",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := diff.Strings(&buf, tt.before, tt.after); err != nil {
				t.Fatal(err)
			}
			if buf.Len() > 0 {
				t.Errorf("want no diff, but got %q", buf.String())
			}
		})
	}
}

// https://github.com/shogo82148/schemalex-deploy/issues/309
func TestDiff_TableNormalizer(t *testing.T) {
	// normalize the expressions by removing spaces and backquotes, and lowering them.
	normalize := func(table *model.Table) (*model.Table, error) {
		tbl := *table
		tbl.Columns = nil
		for _, col := range table.Columns {
			c := *col
			c.Generated.Expr = strings.ToLower(strings.NewReplacer(" ", "", "`", "").Replace(c.Generated.Expr))
			tbl.Columns = append(tbl.Columns, &c)
		}
		return &tbl, nil
	}

	tests := []struct {
		name       string
		before     string
		after      string
		normalizer diff.TableNormalizer
		want       string
	}{
		{
			name:   "no normalizer",
			before: "CREATE TABLE t (a INT, b INT AS ((`a` + 1)));",
			after:  "CREATE TABLE t (a INT, b INT AS (a+1));",
			want:   "ALTER TABLE `t` CHANGE COLUMN `b` `b` INT (11) GENERATED ALWAYS AS (a+1) VIRTUAL;\n",
		},
		{
			name:       "equivalent expressions",
			before:     "CREATE TABLE t (a INT, b INT AS ((`a` + 1)));",
			after:      "CREATE TABLE t (a INT, b INT AS (A+1));",
			normalizer: normalize,
			want:       "",
		},
		{
			name:       "different expressions",
			before:     "CREATE TABLE t (a INT, b INT AS ((`a` + 1)));",
			after:      "CREATE TABLE t (a INT, b INT AS (a + 2));",
			normalizer: normalize,
			want:       "ALTER TABLE `t` CHANGE COLUMN `b` `b` INT (11) GENERATED ALWAYS AS (a + 2) VIRTUAL;\n",
		},
		{
			name:       "equivalent expressions with other changes",
			before:     "CREATE TABLE t (a INT, b INT AS ((`a` + 1)));",
			after:      "CREATE TABLE t (a INT, b INT AS (a + 1) COMMENT 'b');",
			normalizer: normalize,
			want:       "ALTER TABLE `t` CHANGE COLUMN `b` `b` INT (11) GENERATED ALWAYS AS (a + 1) VIRTUAL COMMENT 'b';\n",
		},
		{
			name:   "cannot normalize",
			before: "CREATE TABLE t (a INT, b INT AS ((`a` + 1)));",
			after:  "CREATE TABLE t (a INT, b INT AS (a + 1));",
			normalizer: func(table *model.Table) (*model.Table, error) {
				return nil, nil
			},
			want: "ALTER TABLE `t` CHANGE COLUMN `b` `b` INT (11) GENERATED ALWAYS AS (a + 1) VIRTUAL;\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []diff.Option
			if tt.normalizer != nil {
				opts = append(opts, diff.WithTableNormalizer(tt.normalizer))
			}
			var buf strings.Builder
			if err := diff.Strings(&buf, tt.before, tt.after, opts...); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, buf.String()); diff != "" {
				t.Errorf("(-want/+got)\n%s", diff)
			}
		})
	}
}

func TestDiff_TableNormalizerIsCalledLazily(t *testing.T) {
	var called int
	normalizer := func(table *model.Table) (*model.Table, error) {
		called++
		return table, nil
	}

	// the expressions are same as written, so the normalizer is not necessary.
	var buf strings.Builder
	err := diff.Strings(&buf,
		"CREATE TABLE t (a INT, b INT AS (a + 1), c INT);",
		"CREATE TABLE t (a INT, b INT AS (a + 1), c BIGINT);",
		diff.WithTableNormalizer(normalizer),
	)
	if err != nil {
		t.Fatal(err)
	}
	if called != 0 {
		t.Errorf("want the normalizer is not called, but called %d times", called)
	}

	// the normalizer is called once for each of the old and new tables.
	buf.Reset()
	err = diff.Strings(&buf,
		"CREATE TABLE t (a INT, b INT AS (a + 1), c INT AS (a + 2));",
		"CREATE TABLE t (a INT, b INT AS (a+1), c INT AS (a+2));",
		diff.WithTableNormalizer(normalizer),
	)
	if err != nil {
		t.Fatal(err)
	}
	if called != 2 {
		t.Errorf("want the normalizer is called twice, but called %d times", called)
	}
}

func TestDiff_TableNormalizerError(t *testing.T) {
	normalizer := func(table *model.Table) (*model.Table, error) {
		return nil, errors.New("something wrong")
	}
	var buf strings.Builder
	err := diff.Strings(&buf,
		"CREATE TABLE t (a INT, b INT AS (a + 1));",
		"CREATE TABLE t (a INT, b INT AS (a+1));",
		diff.WithTableNormalizer(normalizer),
	)
	if err == nil {
		t.Error("want error, but not")
	}
}
