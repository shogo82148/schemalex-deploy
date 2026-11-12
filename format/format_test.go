package format

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy"
	"github.com/shogo82148/schemalex-deploy/model"
)

type Spec struct {
	Input  string
	Error  bool
	Expect string
}

func testParse(t *testing.T, spec *Spec) {
	t.Helper()

	p := schemalex.New()
	stmts, err := p.ParseString(spec.Input)
	if spec.Error {
		if err == nil {
			t.Error("expected an error, but not")
		}
		return
	}
	if err != nil {
		t.Errorf("got unexpected error: %v", err.Error())
		return
	}

	var buf strings.Builder
	if err := SQL(&buf, stmts); err != nil {
		t.Errorf("format.SQL returns unexpected error: %v", err)
		return
	}

	got := buf.String()
	if diff := cmp.Diff(spec.Expect, got); diff != "" {
		t.Errorf("(-want/+got):\n%s", diff)
	}
}

func TestFormat(t *testing.T) {
	parse := func(title string, spec *Spec) {
		t.Helper()
		t.Run(title, func(t *testing.T) {
			t.Helper()
			testParse(t, spec)
		})
	}

	// create database are ignored
	parse("CreateDatabase", &Spec{
		Input: "create DATABASE hoge",
	})
	parse("CreateDatabaseIfNotExists", &Spec{
		Input: "create DATABASE IF NOT EXISTS hoge",
	})
	parse("CreateDatabase17", &Spec{
		Input: "create DATABASE 17",
		Error: true,
	})
	parse("MultipleCreateDatabase", &Spec{
		Input: "create DATABASE hoge; create database fuga;",
	})

	parse("CreateTableIntegerNoWidth", &Spec{
		Input: "create table hoge_table ( id integer unsigned not null)",
		Expect: "CREATE TABLE `hoge_table` (\n" +
			"`id` INT (10) UNSIGNED NOT NULL\n" +
			");\n",
	})
	parse("CStyleComment", &Spec{
		Input: "create table hoge ( /* id integer unsigned not null */ c varchar not null )",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`c` VARCHAR NOT NULL\n" +
			");\n",
	})
	parse("DoubleDashComment", &Spec{
		Input: "create table hoge ( -- id integer unsigned not null;\n" +
			" c varchar not null )",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`c` VARCHAR NOT NULL\n" +
			");\n",
	})
	parse("TrailingComma", &Spec{
		Input: `create table hoge (
a varchar(20) default "hoge",
b varchar(20) default 'hoge',
c int not null default 10,
);
`,
		Error: true,
	})

	parse("VariousDefaultTypes", &Spec{
		Input: `create table hoge (
a varchar(20) default "hoge",
b varchar(20) default 'hoge',
c int not null default 10
);
`,
		Expect: "CREATE TABLE `hoge` (\n" +
			"`a` VARCHAR (20) DEFAULT 'hoge',\n" +
			"`b` VARCHAR (20) DEFAULT 'hoge',\n" +
			"`c` INT (11) NOT NULL DEFAULT 10\n" +
			");\n",
	})

	parse("WithSRID", &Spec{
		Input: `create table hoge (a geometry not null srid 0);`,
		Expect: "CREATE TABLE `hoge` (\n" +
			"`a` GEOMETRY NOT NULL SRID 0\n" +
			");\n",
	})

	parse("WithPrimaryKey", &Spec{
		Input: `create table hoge (
id bigint unsigned not null auto_increment,
c varchar(20) not null default "hoge",
primary key (id, c)
);
`,
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
			"`c` VARCHAR (20) NOT NULL DEFAULT 'hoge',\n" +
			"PRIMARY KEY (`id`, `c`)\n" +
			");\n",
	})

	parse("WithTableOptions", &Spec{
		Input: "create table hoge (" +
			"id bigint unsigned not null auto_increment" +
			") ENGINE=InnoDB AUTO_INCREMENT 10 DEFAULT CHARACTER SET = utf8 COMMENT = 'hoge comment';",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT\n" +
			") ENGINE = InnoDB, AUTO_INCREMENT = 10, DEFAULT CHARACTER SET = utf8, COMMENT = 'hoge comment';\n",
	})

	parse("NormalizeCharacterSetToCharset", &Spec{
		Input: "create table hoge (" +
			"id bigint unsigned not null auto_increment" +
			") ENGINE=InnoDB AUTO_INCREMENT 10 DEFAULT CHARSET = utf8 COMMENT = 'hoge comment';",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT\n" +
			") ENGINE = InnoDB, AUTO_INCREMENT = 10, DEFAULT CHARACTER SET = utf8, COMMENT = 'hoge comment';\n",
	})

	parse("WithKeyAndIndex", &Spec{
		Input: "create table hoge (" +
			"`id` bigint unsigned not null auto_increment,\n" +
			"`c` varchar(20) not null,\n" +
			"KEY (`id`), INDEX (`c`)\n" +
			");\n",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
			"`c` VARCHAR (20) NOT NULL,\n" +
			"INDEX (`id`),\n" +
			"INDEX (`c`)\n" +
			");\n",
	})
	parse("WithUniqueKeyPrimaryKey", &Spec{
		Input: "create table hoge (" +
			"`id` bigint unsigned not null auto_increment,\n" +
			"`c` varchar(20) not null,\n" +
			"UNIQUE INDEX `uniq_id` (`id`, `c`),\n" +
			"PRIMARY KEY (`id`)\n" +
			");\n",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
			"`c` VARCHAR (20) NOT NULL,\n" +
			"UNIQUE INDEX `uniq_id` (`id`, `c`),\n" +
			"PRIMARY KEY (`id`)\n" +
			");\n",
	})
	parse("WithBasicForeignKey", &Spec{
		Input: "create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
			"`c` varchar(20) not null,\n" +
			"FOREIGN KEY `fk_c` (`c`) )",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
			"`c` VARCHAR (20) NOT NULL,\n" +
			"FOREIGN KEY `fk_c` (`c`)\n" +
			");\n",
	})
	parse("WithFulltextIndex1", &Spec{
		Input: "create table hoge (txt TEXT, fulltext ft_idx(txt))",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`txt` TEXT,\n" +
			"FULLTEXT INDEX `ft_idx` (`txt`)\n" +
			");\n",
	})
	parse("WithFulltextIndex2", &Spec{
		Input: "create table hoge (txt TEXT, fulltext index ft_idx(txt))",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`txt` TEXT,\n" +
			"FULLTEXT INDEX `ft_idx` (`txt`)\n" +
			");\n",
	})
	parse("WithFulltextIndex3", &Spec{
		Input: "create table hoge (txt TEXT, fulltext key ft_idx(txt))",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`txt` TEXT,\n" +
			"FULLTEXT INDEX `ft_idx` (`txt`)\n" +
			");\n",
	})
	parse("WithFullTextIndexWithParser", &Spec{
		Input: "create table hoge (txt TEXT, fulltext ft_idx(txt) with parser ngram)",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`txt` TEXT,\n" +
			"FULLTEXT INDEX `ft_idx` (`txt`) WITH PARSER `ngram`" +
			"\n);\n",
	})
	parse("WithSimpleReferenceForeignKey", &Spec{
		Input: "create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
			"`c` varchar(20) not null,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) )",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
			"`c` VARCHAR (20) NOT NULL,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`)\n" +
			");\n",
	})
	parse("WithMatchReferenceForeignKey", &Spec{
		Input: "create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
			"`c` varchar(20) not null,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) MATCH SIMPLE )",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
			"`c` VARCHAR (20) NOT NULL,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) MATCH SIMPLE\n" +
			");\n",
	})
	parse("WithOnDeleteReferenceForeignKey", &Spec{
		Input: "create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
			"`c` varchar(20) not null,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) ON DELETE SET NULL)",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
			"`c` VARCHAR (20) NOT NULL,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) ON DELETE SET NULL\n" +
			");\n",
	})
	parse("WithMatchAndOnDeleteReferenceForeignKey", &Spec{
		Input: "create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
			"`c` varchar(20) not null,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) MATCH PARTIAL ON DELETE NO ACTION)",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
			"`c` VARCHAR (20) NOT NULL,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) MATCH PARTIAL ON DELETE NO ACTION\n" +
			");\n",
	})
	parse("WithOnDeleteOnUpdateReferenceForeignKey", &Spec{
		Input: "create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
			"`c` varchar(20) not null,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) ON DELETE NO ACTION ON UPDATE CASCADE)",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`id` BIGINT (20) UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
			"`c` VARCHAR (20) NOT NULL,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) ON DELETE NO ACTION ON UPDATE CASCADE\n" +
			");\n",
	})
	parse("UnexpectedIndentShownAfterReferencesFuga", &Spec{
		Input: "create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
			"`c` varchar(20) not null,\n" +
			"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) HOGE )",
		Error: true,
	})
	parse("DecimalNotDefault", &Spec{
		Input: "create table hoge (`foo` DECIMAL(32,30))",
		Expect: "CREATE TABLE `hoge` (\n" +
			"`foo` DECIMAL (32,30) DEFAULT NULL\n" +
			");\n",
	})
	parse("UniqueKeyWithConstraint", &Spec{
		Input: "CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, CONSTRAINT `symbol` UNIQUE KEY `uniq_id` USING BTREE (`id`) )",
		Expect: "CREATE TABLE `fuga` (\n" +
			"`id` INT (11) NOT NULL AUTO_INCREMENT,\n" +
			"CONSTRAINT `symbol` UNIQUE INDEX `uniq_id` USING BTREE (`id`)\n" +
			");\n",
	})
	parse("DropTableIfExists", &Spec{
		Input: "DROP TABLE IF EXISTS `konboi_bug`; CREATE TABLE foo(`id` INT)",
		Expect: "CREATE TABLE `foo` (\n" +
			"`id` INT (11) DEFAULT NULL\n" +
			");\n",
	})
	parse("ColumnCharacterSet", &Spec{
		Input: "CREATE TABLE `foo` (col TEXT CHARACTER SET latin1)",
		Expect: "CREATE TABLE `foo` (\n" +
			"`col` TEXT CHARACTER SET `latin1`\n" +
			");\n",
	})
	parse("OnUpdateCurrentTimestampNoDefault", &Spec{
		Input: "CREATE TABLE `foo` (col DATETIME ON UPDATE CURRENT_TIMESTAMP)",
		Expect: "CREATE TABLE `foo` (\n" +
			"`col` DATETIME ON UPDATE CURRENT_TIMESTAMP DEFAULT NULL\n" +
			");\n",
	})
	parse("KeyNormalizedToIndex", &Spec{
		Input: "CREATE TABLE `foo` (col TEXT, KEY col_idx (col(196)))",
		Expect: "CREATE TABLE `foo` (\n" +
			"`col` TEXT,\n" +
			"INDEX `col_idx` (`col`(196))\n" +
			");\n",
	})
	parse("CreateTableLike", &Spec{
		Input:  "CREATE TABLE foo LIKE bar",
		Expect: "CREATE TABLE `foo` LIKE `bar`;\n",
	})

	// https://github.com/shogo82148/schemalex-deploy/issues/325
	parse("CreateTemporaryTable", &Spec{
		Input: "CREATE TEMPORARY TABLE foo (a INT)",
		Expect: "CREATE TEMPORARY TABLE `foo` (\n" +
			"`a` INT (11) DEFAULT NULL\n" +
			");\n",
	})
	parse("CreateTemporaryTableIfNotExistsLike", &Spec{
		Input:  "create temporary table if not exists foo like bar",
		Expect: "CREATE TEMPORARY TABLE IF NOT EXISTS `foo` LIKE `bar`;\n",
	})
	parse("CreateTemporaryTableInExecutableComment", &Spec{
		Input: "CREATE /*!32300 TEMPORARY */ TABLE foo (a INT)",
		Expect: "CREATE TEMPORARY TABLE `foo` (\n" +
			"`a` INT (11) DEFAULT NULL\n" +
			");\n",
	})
	parse("CreateTemporaryWithoutTable", &Spec{
		Input: "CREATE TEMPORARY foo (a INT)",
		Error: true,
	})
	parse("CreateTableTemporary", &Spec{
		// TEMPORARY must be placed before TABLE.
		Input: "CREATE TABLE TEMPORARY foo (a INT)",
		Error: true,
	})
	// https://github.com/shogo82148/schemalex-deploy/issues/323
	parse("PartitionInExecutableComment", &Spec{
		// SHOW CREATE TABLE returns the partitioning clause in an executable comment.
		Input: "CREATE TABLE `t` (\n  `id` int NOT NULL\n) ENGINE=InnoDB\n" +
			"/*!50100 PARTITION BY HASH (`id`)\nPARTITIONS 2 */",
		Expect: "CREATE TABLE `t` (\n" +
			"`id` INT (11) NOT NULL\n" +
			") ENGINE = InnoDB\n" +
			"PARTITION BY HASH (`id`)\nPARTITIONS 2;\n",
	})
	parse("PartitionWithoutTableOptions", &Spec{
		Input: "CREATE TABLE t (id INT) PARTITION BY RANGE (id) (PARTITION p0 VALUES LESS THAN (10), PARTITION p1 VALUES LESS THAN MAXVALUE);" +
			"CREATE TABLE u (id INT)",
		Expect: "CREATE TABLE `t` (\n" +
			"`id` INT (11) DEFAULT NULL\n" +
			")\n" +
			"PARTITION BY RANGE (id) (PARTITION p0 VALUES LESS THAN (10), PARTITION p1 VALUES LESS THAN MAXVALUE);\n" +
			"CREATE TABLE `u` (\n" +
			"`id` INT (11) DEFAULT NULL\n" +
			");\n",
	})
	parse("PartitionWithExecutableComment", &Spec{
		Input: "CREATE TABLE t (id INT) PARTITION BY HASH (id) /*!50100 PARTITIONS 2 */",
		Expect: "CREATE TABLE `t` (\n" +
			"`id` INT (11) DEFAULT NULL\n" +
			")\n" +
			"PARTITION BY HASH (id) PARTITIONS 2;\n",
	})
	parse("PartitionWithTrailingComment", &Spec{
		// the trailing comment is removed, otherwise it comments out the semicolon in the output.
		Input: "CREATE TABLE t (id INT) PARTITION BY HASH (id) -- comment\n;",
		Expect: "CREATE TABLE `t` (\n" +
			"`id` INT (11) DEFAULT NULL\n" +
			")\n" +
			"PARTITION BY HASH (id);\n",
	})
	parse("PartitionWithMultiplication", &Spec{
		// the multiplication operator is permitted in partitioning expressions.
		Input: "CREATE TABLE `t` (\n  `id` int DEFAULT NULL\n) ENGINE=InnoDB\n" +
			"/*!50100 PARTITION BY HASH ((`id` * 2))\nPARTITIONS 2 */",
		Expect: "CREATE TABLE `t` (\n" +
			"`id` INT (11) DEFAULT NULL\n" +
			") ENGINE = InnoDB\n" +
			"PARTITION BY HASH ((`id` * 2))\nPARTITIONS 2;\n",
	})
	parse("PartitionWithModulo", &Spec{
		// SHOW CREATE TABLE returns `MOD` as `%`.
		Input: "CREATE TABLE `t` (\n  `id` int DEFAULT NULL\n) ENGINE=InnoDB\n" +
			"/*!50100 PARTITION BY HASH ((`id` % 3))\nPARTITIONS 2 */",
		Expect: "CREATE TABLE `t` (\n" +
			"`id` INT (11) DEFAULT NULL\n" +
			") ENGINE = InnoDB\n" +
			"PARTITION BY HASH ((`id` % 3))\nPARTITIONS 2;\n",
	})
	parse("PartitionWithIllegalToken", &Spec{
		// other operators, such as bitwise operators, are not permitted in partitioning expressions,
		// and the lexer does not support them.
		Input: "CREATE TABLE t (id INT) PARTITION BY HASH (id & 1) PARTITIONS 2",
		Error: true,
	})
	parse("PartitionWithoutBy", &Spec{
		Input: "CREATE TABLE t (id INT) PARTITION HASH (id)",
		Error: true,
	})
	parse("PartitionEmpty", &Spec{
		Input: "CREATE TABLE t (id INT) PARTITION BY;",
		Error: true,
	})
	parse("PartitionUnclosedParen", &Spec{
		Input: "CREATE TABLE t (id INT) PARTITION BY HASH (id;",
		Error: true,
	})
	parse("PartitionUnopenedParen", &Spec{
		Input: "CREATE TABLE t (id INT) PARTITION BY HASH (id));",
		Error: true,
	})

	parse("TableNamedTemporary", &Spec{
		// TEMPORARY is not a reserved word, so it can be used as a table name.
		Input: "CREATE TABLE temporary (a INT)",
		Expect: "CREATE TABLE `temporary` (\n" +
			"`a` INT (11) DEFAULT NULL\n" +
			");\n",
	})
	parse("ColumnOptionPrimaryKey", &Spec{
		// see https://github.com/schemalex/schemalex/pull/40
		Input: "CREATE TABLE foo (id INTEGER PRIMARY KEY AUTO_INCREMENT)",
		Expect: "CREATE TABLE `foo` (\n" +
			"`id` INT (11) DEFAULT NULL AUTO_INCREMENT,\n" +
			"PRIMARY KEY (`id`)\n" +
			");\n",
	})
	parse("ColumnOptionCommentPrimaryKey1", &Spec{
		// see https://github.com/schemalex/schemalex/pull/40
		Input: "CREATE TABLE `test` (\n" +
			"`id` int(11) PRIMARY KEY COMMENT 'aaa' NOT NULL,\n" +
			"hoge int default 1 not null COMMENT 'bbb' UNIQUE\n" +
			");",
		Expect: "CREATE TABLE `test` (\n" +
			"`id` INT (11) NOT NULL COMMENT 'aaa',\n" +
			"`hoge` INT (11) NOT NULL DEFAULT 1 COMMENT 'bbb',\n" +
			"PRIMARY KEY (`id`),\n" +
			"UNIQUE INDEX `hoge` (`hoge`)\n" +
			");\n",
	})
	parse("ColumnOptionCommentPrimaryKey2", &Spec{
		// see https://github.com/schemalex/schemalex/pull/40
		Input: "CREATE TABLE `test` (\n" +
			"`id` int(11) COMMENT 'aaa' PRIMARY KEY NOT NULL,\n" +
			"hoge int default 1 UNIQUE not null COMMENT 'bbb'\n" +
			");",
		Expect: "CREATE TABLE `test` (\n" +
			"`id` INT (11) NOT NULL COMMENT 'aaa',\n" +
			"`hoge` INT (11) NOT NULL DEFAULT 1 COMMENT 'bbb',\n" +
			"PRIMARY KEY (`id`),\n" +
			"UNIQUE INDEX `hoge` (`hoge`)\n" +
			");\n",
	})
	parse("Enum", &Spec{
		Input: "CREATE TABLE `test` (\n" +
			"`status` ENUM('on', 'off') NOT NULL DEFAULT 'off'\n" +
			");",
		Expect: "CREATE TABLE `test` (\n" +
			"`status` ENUM ('on','off') NOT NULL DEFAULT 'off'\n" +
			");\n",
	})
	parse("Set", &Spec{
		Input: "CREATE TABLE `test` (\n" +
			"`status` SET('foo', 'bar', 'baz') NOT NULL DEFAULT 'foo,baz'\n" +
			");",
		Expect: "CREATE TABLE `test` (\n" +
			"`status` SET ('foo','bar','baz') NOT NULL DEFAULT 'foo,baz'\n" +
			");\n",
	})
	parse("Flag Set Not found", &Spec{
		Input: "CREATE TABLE``(``SET)",
		Error: true,
	})
	parse("BooleanDefaultTrue", &Spec{
		Input: "CREATE TABLE `test` (\n" +
			"`valid` BOOLEAN not null default true\n" +
			");",
		Expect: "CREATE TABLE `test` (\n" +
			"`valid` TINYINT (1) NOT NULL DEFAULT 1\n" +
			");\n",
	})
	parse("BooleanDefaultFalse", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` BOOLEAN not null default false\n);",
		Expect: "CREATE TABLE `test` (\n`valid` TINYINT (1) NOT NULL DEFAULT 0\n);\n",
	})
	parse("BoolDefaultTrue", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` BOOL not null default true\n);",
		Expect: "CREATE TABLE `test` (\n`valid` TINYINT (1) NOT NULL DEFAULT 1\n);\n",
	})
	parse("BoolDefaultFalse", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` BOOL not null default false\n);",
		Expect: "CREATE TABLE `test` (\n`valid` TINYINT (1) NOT NULL DEFAULT 0\n);\n",
	})
	parse("JSON", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` JSON not null\n);",
		Expect: "CREATE TABLE `test` (\n`valid` JSON NOT NULL\n);\n",
	})
	parse("GEOMETRY", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` GEOMETRY not null\n);",
		Expect: "CREATE TABLE `test` (\n`valid` GEOMETRY NOT NULL\n);\n",
	})
	parse("POINT", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` POINT not null\n);",
		Expect: "CREATE TABLE `test` (\n`valid` POINT NOT NULL\n);\n",
	})
	parse("LINESTRING", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` LINESTRING not null\n);",
		Expect: "CREATE TABLE `test` (\n`valid` LINESTRING NOT NULL\n);\n",
	})
	parse("POLYGON", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` POLYGON not null\n);",
		Expect: "CREATE TABLE `test` (\n`valid` POLYGON NOT NULL\n);\n",
	})
	parse("MULTIPOINT", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` MULTIPOINT not null\n);",
		Expect: "CREATE TABLE `test` (\n`valid` MULTIPOINT NOT NULL\n);\n",
	})
	parse("MULTILINESTRING", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` MULTILINESTRING not null\n);",
		Expect: "CREATE TABLE `test` (\n`valid` MULTILINESTRING NOT NULL\n);\n",
	})
	parse("MULTIPOLYGON", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` MULTIPOLYGON not null\n);",
		Expect: "CREATE TABLE `test` (\n`valid` MULTIPOLYGON NOT NULL\n);\n",
	})
	parse("GEOMETRYCOLLECTION", &Spec{
		Input:  "CREATE TABLE `test` (\n`valid` GEOMETRYCOLLECTION not null\n);",
		Expect: "CREATE TABLE `test` (\n`valid` GEOMETRYCOLLECTION NOT NULL\n);\n",
	})
	parse("CreateTableIfNotExists", &Spec{
		Input:  "CREATE TABLE IF NOT EXISTS `test` (\n`id` INT (10) NOT NULL\n);",
		Expect: "CREATE TABLE IF NOT EXISTS `test` (\n`id` INT (10) NOT NULL\n);\n",
	})
	parse("MultipleTableOptions", &Spec{
		Input:  "CREATE TABLE foo (id INT(10) NOT NULL) ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4",
		Expect: "CREATE TABLE `foo` (\n`id` INT (10) NOT NULL\n) ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4;\n",
	})
	parse("GithubIssue59", &Spec{
		// see https://github.com/schemalex/schemalex/issues/59
		Input: "DROP TABLE IF EXISTS `socialaccount_socialtoken`;\n" +
			"CREATE TABLE `socialaccount_socialtoken` (\n" +
			"`id` int(11) NOT NULL AUTO_INCREMENT,\n" +
			"`token` longtext COLLATE utf8mb4_unicode_ci NOT NULL,\n" +
			"`token_secret` longtext COLLATE utf8mb4_unicode_ci NOT NULL,\n" +
			"`expires_at` datetime(6) DEFAULT NULL,\n" +
			"`account_id` int(11) NOT NULL,\n" +
			"`app_id` int(11) NOT NULL,\n" +
			"PRIMARY KEY (`id`) USING BTREE,\n" +
			"UNIQUE KEY `socialaccount_socialtoken_app_id_account_id_fca4e0ac_uniq` (`app_id`,`account_id`) USING BTREE,\n" +
			"KEY `socialaccount_social_account_id_951f210e_fk_socialacc` (`account_id`) USING BTREE,\n" +
			"CONSTRAINT `socialaccount_social_account_id_951f210e_fk_socialacc` FOREIGN KEY (`account_id`) REFERENCES `socialaccount_socialaccount` (`id`),\n" +
			"CONSTRAINT `socialaccount_social_app_id_636a42d7_fk_socialacc` FOREIGN KEY (`app_id`) REFERENCES `socialaccount_socialapp` (`id`)\n" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;",
		Expect: "CREATE TABLE `socialaccount_socialtoken` (\n" +
			"`id` INT (11) NOT NULL AUTO_INCREMENT,\n" +
			"`token` LONGTEXT COLLATE `utf8mb4_unicode_ci` NOT NULL,\n" +
			"`token_secret` LONGTEXT COLLATE `utf8mb4_unicode_ci` NOT NULL,\n" +
			"`expires_at` DATETIME (6) DEFAULT NULL,\n" +
			"`account_id` INT (11) NOT NULL,\n" +
			"`app_id` INT (11) NOT NULL,\n" +
			"PRIMARY KEY USING BTREE (`id`),\n" +
			"UNIQUE INDEX `socialaccount_socialtoken_app_id_account_id_fca4e0ac_uniq` USING BTREE (`app_id`, `account_id`),\n" +
			"INDEX `socialaccount_social_account_id_951f210e_fk_socialacc` USING BTREE (`account_id`),\n" +
			"CONSTRAINT `socialaccount_social_account_id_951f210e_fk_socialacc` FOREIGN KEY (`account_id`) REFERENCES `socialaccount_socialaccount` (`id`),\n" +
			"INDEX `socialaccount_social_app_id_636a42d7_fk_socialacc` (`app_id`),\n" +
			"CONSTRAINT `socialaccount_social_app_id_636a42d7_fk_socialacc` FOREIGN KEY (`app_id`) REFERENCES `socialaccount_socialapp` (`id`)\n" +
			") ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4, DEFAULT COLLATE = utf8mb4_unicode_ci, ROW_FORMAT = DYNAMIC;\n",
	})
	parse("CommentsEmptyLines", &Spec{
		Input: `/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;`,
		Expect: "",
	})
	parse("CommentsAndStatementsMixedTogether", &Spec{
		Input: "/* hello, world*/;\n" +
			"CREATE TABLE foo (\n" +
			"a int);\n" +
			"/* hello, world again! */;\n" +
			"CREATE TABLE bar (\n" +
			"b int);",
		Expect: "CREATE TABLE `foo` (\n" +
			"`a` INT (11) DEFAULT NULL\n" +
			");\n" +
			"CREATE TABLE `bar` (\n" +
			"`b` INT (11) DEFAULT NULL\n" +
			");\n",
	})
	parse("GithubIssue62", &Spec{
		// see https://github.com/schemalex/schemalex/issues/62
		Input: "DROP TABLE IF EXISTS `some_table`;\r\n" +
			"/*!40101 SET @saved_cs_client     = @@character_set_client */;\r\n" +
			"SET character_set_client = utf8mb4 ;\r\n" +
			"CREATE TABLE `some_table` (\r\n" +
			"  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,\r\n" +
			"  `user_id` varchar(32) DEFAULT NULL,\r\n" +
			"  `context` json DEFAULT NULL,\r\n" +
			"  `created_at` datetime DEFAULT CURRENT_TIMESTAMP,\r\n" +
			"  PRIMARY KEY (`id`),\r\n" +
			"  KEY `created_at` (`created_at` DESC) /*!80000 INVISIBLE */,\r\n" +
			"  KEY `user_id_idx` (`user_id`),\r\n" +
			"  CONSTRAINT `some_table__user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE SET NULL ON UPDATE SET NULL\r\n" +
			") ENGINE=InnoDB AUTO_INCREMENT=19 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;",
		Expect: "CREATE TABLE `some_table` (\n" +
			"`id` INT (10) UNSIGNED NOT NULL AUTO_INCREMENT,\n" +
			"`user_id` VARCHAR (32) DEFAULT NULL,\n" +
			"`context` JSON DEFAULT NULL,\n" +
			"`created_at` DATETIME DEFAULT CURRENT_TIMESTAMP,\n" +
			"PRIMARY KEY (`id`),\n" +
			"INDEX `created_at` (`created_at` DESC) INVISIBLE,\n" +
			"INDEX `user_id_idx` (`user_id`),\n" +
			"INDEX `some_table__user_id` (`user_id`),\n" +
			"CONSTRAINT `some_table__user_id` FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON DELETE SET NULL ON UPDATE SET NULL\n" +
			") ENGINE = InnoDB, AUTO_INCREMENT = 19, DEFAULT CHARACTER SET = utf8mb4, DEFAULT COLLATE = utf8mb4_0900_ai_ci;\n",
	})
	parse("DefaultNow", &Spec{
		Input: "create table `test_log` (`created_at` DATETIME default NOW())",
		Expect: "CREATE TABLE `test_log` (\n" +
			"`created_at` DATETIME DEFAULT NOW()\n" +
			");\n",
	})

	parse("GithubIssue79", &Spec{
		// the partitioning clause in the executable comment is kept.
		// https://github.com/shogo82148/schemalex-deploy/issues/323
		Input: "CREATE TABLE `test_tb` (" +
			"  `t_id` char(17) NOT NULL," +
			"  `t_type` smallint(6) NOT NULL," +
			"  `cur_date` datetime NOT NULL" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8" +
			"/*!50100 PARTITION BY LIST (`t_type`)" +
			"(PARTITION p_1 VALUES IN (1) ENGINE = InnoDB," +
			" PARTITION p_100 VALUES IN (100) ENGINE = InnoDB) */;" +
			"/*!40101 SET character_set_client = @saved_cs_client */;",
		Expect: "CREATE TABLE `test_tb` (\n" +
			"`t_id` CHAR (17) NOT NULL,\n" +
			"`t_type` SMALLINT (6) NOT NULL,\n" +
			"`cur_date` DATETIME NOT NULL\n" +
			") ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8\n" +
			"PARTITION BY LIST (`t_type`)" +
			"(PARTITION p_1 VALUES IN (1) ENGINE = InnoDB," +
			" PARTITION p_100 VALUES IN (100) ENGINE = InnoDB);\n",
	})
	parse("WhiteSpacesBetweenTableOptionsAndSemicolon", &Spec{
		Input: "CREATE TABLE foo (id INT(10) NOT NULL) ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4 \n/**/ ;",
		Expect: "CREATE TABLE `foo` (\n" +
			"`id` INT (10) NOT NULL\n" +
			") ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4;\n",
	})

	parse("GeneratedColumnStored", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT GENERATED ALWAYS AS (a + 1) STORED)",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) GENERATED ALWAYS AS (a + 1) STORED\n" +
			");\n",
	})
	parse("GeneratedColumnShortForm", &Spec{
		// GENERATED ALWAYS is optional, and VIRTUAL is the default.
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (a + 1) NOT NULL COMMENT 'generated')",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) GENERATED ALWAYS AS (a + 1) VIRTUAL NOT NULL COMMENT 'generated'\n" +
			");\n",
	})
	parse("GeneratedColumnShowCreateTable", &Spec{
		// SHOW CREATE TABLE wraps the expression with redundant parentheses.
		Input: "CREATE TABLE `t` (\n" +
			"  `a` int NOT NULL,\n" +
			"  `b` int GENERATED ALWAYS AS ((`a` + 1)) STORED,\n" +
			"  `c` varchar(64) GENERATED ALWAYS AS (concat(`a`,'(',`a`,')')) VIRTUAL,\n" +
			"  `d` int GENERATED ALWAYS AS ((`a` * `a`) % 7) VIRTUAL,\n" +
			"  PRIMARY KEY (`a`)\n" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) GENERATED ALWAYS AS (`a` + 1) STORED,\n" +
			"`c` VARCHAR (64) GENERATED ALWAYS AS (concat(`a`,'(',`a`,')')) VIRTUAL,\n" +
			"`d` INT (11) GENERATED ALWAYS AS ((`a` * `a`) % 7) VIRTUAL,\n" +
			"PRIMARY KEY (`a`)\n" +
			") ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4;\n",
	})
	parse("GeneratedColumnMissingExpression", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT GENERATED ALWAYS AS STORED)",
		Error: true,
	})
	parse("GeneratedColumnUnclosedParen", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (a + 1",
		Error: true,
	})
	parse("GeneratedColumnEmptyExpression", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS ())",
		Error: true,
	})
	parse("GeneratedColumnEmptyExpressionWithSpaces", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT GENERATED ALWAYS AS (  ) STORED)",
		Error: true,
	})
	parse("GeneratedColumnEmptyExpressionInRedundantParens", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (( )))",
		Error: true,
	})
	parse("GeneratedColumnEmptyExpressionWithComment", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (/* comment */))",
		Error: true,
	})
	parse("GeneratedColumnEmptyExpressionWithExecutableComment", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (/*!80000 */))",
		Error: true,
	})
	parse("GeneratedColumnInExecutableComment", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (/*! a + 1 */))",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) GENERATED ALWAYS AS (a + 1) VIRTUAL\n" +
			");\n",
	})
	parse("GeneratedColumnInVersionedExecutableComment", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (/*!80000 (a + 1) */) STORED)",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) GENERATED ALWAYS AS (a + 1) STORED\n" +
			");\n",
	})
	parse("GeneratedColumnPartlyInExecutableComment", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (a /*! + 1 */))",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) GENERATED ALWAYS AS (a + 1) VIRTUAL\n" +
			");\n",
	})
	parse("GeneratedColumnExecutableCommentSeparatesTokens", &Spec{
		// the markers separate the tokens, so `a/*!*/MOD/*!*/2` is `a MOD 2`, not `aMOD2`.
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (a/*!*/MOD/*!*/2))",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) GENERATED ALWAYS AS (a MOD 2) VIRTUAL\n" +
			");\n",
	})
	parse("GeneratedColumnWithExecutableCommentClause", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT /*!50700 AS (a + 1) STORED */)",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) GENERATED ALWAYS AS (a + 1) STORED\n" +
			");\n",
	})

	// options that generated columns cannot have.
	// MySQL rejects them with "Incorrect usage of ... and generated column" or a syntax error.
	parse("GeneratedColumnWithDefault", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (a + 1) DEFAULT 1)",
		Error: true,
	})
	parse("GeneratedColumnWithDefaultBefore", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT DEFAULT 1 AS (a + 1))",
		Error: true,
	})
	parse("GeneratedColumnWithAutoIncrement", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (a + 1) AUTO_INCREMENT)",
		Error: true,
	})
	parse("GeneratedColumnWithAutoIncrementBefore", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AUTO_INCREMENT GENERATED ALWAYS AS (a + 1))",
		Error: true,
	})
	parse("GeneratedColumnWithOnUpdate", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b TIMESTAMP AS (now()) ON UPDATE CURRENT_TIMESTAMP)",
		Error: true,
	})
	parse("GeneratedColumnWithOnUpdateBefore", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b TIMESTAMP ON UPDATE CURRENT_TIMESTAMP AS (now()))",
		Error: true,
	})
	parse("GeneratedColumnDuplicated", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT AS (a + 1) AS (a + 2))",
		Error: true,
	})
	parse("GeneratedColumnDuplicatedLongForm", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, b INT GENERATED ALWAYS AS (a + 1) STORED GENERATED ALWAYS AS (a + 2) VIRTUAL)",
		Error: true,
	})
	parse("GeneratedColumnRedundantParens", &Spec{
		Input: "CREATE TABLE t (" +
			"a INT NOT NULL, " +
			"b INT AS ( ( a + 1 ) ), " +
			"c INT AS ((a) + (1)), " +
			"d INT AS (((a * a) % 7)), " +
			"e INT AS ((a /* comment */))" +
			")",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) GENERATED ALWAYS AS (a + 1) VIRTUAL,\n" +
			"`c` INT (11) GENERATED ALWAYS AS ((a) + (1)) VIRTUAL,\n" +
			"`d` INT (11) GENERATED ALWAYS AS ((a * a) % 7) VIRTUAL,\n" +
			"`e` INT (11) GENERATED ALWAYS AS (a /* comment */) VIRTUAL\n" +
			");\n",
	})
	parse("GeneratedColumnTrailingLineComment", &Spec{
		// trailing line comments are removed, otherwise they comment out the closing parenthesis in the output.
		Input: "CREATE TABLE t (" +
			"a INT NOT NULL, " +
			"b INT AS (a + 1 # comment\n), " +
			"c INT AS (a + 2 -- comment\n)" +
			")",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) GENERATED ALWAYS AS (a + 1) VIRTUAL,\n" +
			"`c` INT (11) GENERATED ALWAYS AS (a + 2) VIRTUAL\n" +
			");\n",
	})
	parse("GeneratedColumnQuotedParens", &Spec{
		// parentheses inside string literals and quoted identifiers are not counted.
		Input: "CREATE TABLE t (" +
			"a INT NOT NULL, " +
			"b VARCHAR(10) AS (('(') = (')')), " +
			"c VARCHAR(10) AS ((concat(a, ')'))), " +
			"`)` INT NOT NULL, " +
			"d INT AS ((`)` + 1))" +
			")",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` VARCHAR (10) GENERATED ALWAYS AS (('(') = (')')) VIRTUAL,\n" +
			"`c` VARCHAR (10) GENERATED ALWAYS AS (concat(a, ')')) VIRTUAL,\n" +
			"`)` INT (11) NOT NULL,\n" +
			"`d` INT (11) GENERATED ALWAYS AS (`)` + 1) VIRTUAL\n" +
			");\n",
	})

	// https://github.com/shogo82148/schemalex-deploy/issues/308
	parse("EscapedBackslashInDefault", &Spec{
		Input: `CREATE TABLE t (a VARCHAR(10) DEFAULT '\\')`,
		Expect: "CREATE TABLE `t` (\n" +
			"`a` VARCHAR (10) DEFAULT '\\\\'\n" +
			");\n",
	})
	parse("EscapedBackslashInComment", &Spec{
		Input: `CREATE TABLE t (a VARCHAR(10) COMMENT 'a\\', b INT)`,
		Expect: "CREATE TABLE `t` (\n" +
			"`a` VARCHAR (10) DEFAULT NULL COMMENT 'a\\\\',\n" +
			"`b` INT (11) DEFAULT NULL\n" +
			");\n",
	})
	parse("EscapedBackslashInGeneratedColumn", &Spec{
		Input: `CREATE TABLE t (a INT, b VARCHAR(10) AS (concat(a,'\\',')')))`,
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT NULL,\n" +
			"`b` VARCHAR (10) GENERATED ALWAYS AS (concat(a,'\\\\',')')) VIRTUAL\n" +
			");\n",
	})
	parse("BackslashInQuotedIdentifier", &Spec{
		Input: "CREATE TABLE `a\\` (`b\\` INT)",
		Expect: "CREATE TABLE `a\\` (\n" +
			"`b\\` INT (11) DEFAULT NULL\n" +
			");\n",
	})

	// Non-reserved keywords are permitted as identifiers without quoting.
	// https://dev.mysql.com/doc/refman/26.10/en/keywords.html
	parse("NonReservedKeywordsAsIdentifiers", &Spec{
		Input: "create table action (" +
			"date DATE NOT NULL, " +
			"text TEXT, " +
			"comment VARCHAR(10) COMMENT 'comment', " +
			"year YEAR, " +
			"key date (date), " +
			"index text_idx (text(10))" +
			") ENGINE = MEMORY",
		Expect: "CREATE TABLE `action` (\n" +
			"`date` DATE NOT NULL,\n" +
			"`text` TEXT,\n" +
			"`comment` VARCHAR (10) DEFAULT NULL COMMENT 'comment',\n" +
			"`year` YEAR DEFAULT NULL,\n" +
			"INDEX `date` (`date`),\n" +
			"INDEX `text_idx` (`text`(10))\n" +
			") ENGINE = MEMORY;\n",
	})
	parse("NonReservedKeywordsInForeignKey", &Spec{
		Input: "create table data (id INT NOT NULL, status INT NOT NULL, " +
			"constraint fk_status foreign key status (status) references action (date) on delete no action)",
		Expect: "CREATE TABLE `data` (\n" +
			"`id` INT (11) NOT NULL,\n" +
			"`status` INT (11) NOT NULL,\n" +
			"INDEX `fk_status` (`status`),\n" +
			"CONSTRAINT `fk_status` FOREIGN KEY `status` (`status`) REFERENCES `action` (`date`) ON DELETE NO ACTION\n" +
			");\n",
	})
	parse("NonReservedKeywordAsLikeTable", &Spec{
		Input:  "create table data like action",
		Expect: "CREATE TABLE `data` LIKE `action`;\n",
	})
	parse("NonReservedKeywordAsDatabase", &Spec{
		Input: "create database data",
	})
	parse("ReservedWordAsTableName", &Spec{
		Input: "create table table (id INT)",
		Error: true,
	})
	parse("ReservedWordAsColumnName", &Spec{
		Input: "create table foo (int INT)",
		Error: true,
	})
	parse("QuotedReservedWordAsColumnName", &Spec{
		Input: "create table foo (`int` INT)",
		Expect: "CREATE TABLE `foo` (\n" +
			"`int` INT (11) DEFAULT NULL\n" +
			");\n",
	})

	// quotes in string literals are escaped.
	parse("QuoteInDefault", &Spec{
		Input: "CREATE TABLE t (a VARCHAR(10) DEFAULT 'it''s', b VARCHAR(10) DEFAULT 'it\\'s')",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` VARCHAR (10) DEFAULT 'it''s',\n" +
			"`b` VARCHAR (10) DEFAULT 'it''s'\n" +
			");\n",
	})
	parse("QuoteInDoubleQuotedDefault", &Spec{
		Input: `CREATE TABLE t (a VARCHAR(10) DEFAULT "'", b VARCHAR(10) DEFAULT "say ""hi"" \"x\"")`,
		Expect: "CREATE TABLE `t` (\n" +
			"`a` VARCHAR (10) DEFAULT '''',\n" +
			"`b` VARCHAR (10) DEFAULT 'say \"hi\" \"x\"'\n" +
			");\n",
	})
	parse("EscapedQuoteInDoubleQuotedDefault", &Spec{
		// the lexer keeps \' in a double-quoted string as it is,
		// so the value is written in double quotes.
		Input: `CREATE TABLE t (a VARCHAR(10) DEFAULT "it\'s ""x""")`,
		Expect: "CREATE TABLE `t` (\n" +
			"`a` VARCHAR (10) DEFAULT \"it\\'s \"\"x\"\"\"\n" +
			");\n",
	})
	parse("EscapeSequenceInDefault", &Spec{
		Input: `CREATE TABLE t (a VARCHAR(10) DEFAULT 'a\n''b')`,
		Expect: "CREATE TABLE `t` (\n" +
			"`a` VARCHAR (10) DEFAULT 'a\\n''b'\n" +
			");\n",
	})
	parse("QuoteInComment", &Spec{
		Input: "CREATE TABLE t (a INT COMMENT 'it''s')",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT NULL COMMENT 'it''s'\n" +
			");\n",
	})
	parse("QuoteInEnum", &Spec{
		Input: `CREATE TABLE t (a ENUM('it''s', "b'"))`,
		Expect: "CREATE TABLE `t` (\n" +
			"`a` ENUM ('it''s','b''') DEFAULT NULL\n" +
			");\n",
	})
	parse("QuoteInSet", &Spec{
		Input: "CREATE TABLE t (a SET('it''s', 'b'))",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` SET ('it''s','b') DEFAULT NULL\n" +
			");\n",
	})
	parse("EnumWithoutValues", &Spec{
		Input: "CREATE TABLE t (a ENUM)",
		Error: true,
	})
	parse("EnumWithEmptyValues", &Spec{
		Input: "CREATE TABLE t (a ENUM())",
		Error: true,
	})
	parse("SetWithoutValues", &Spec{
		Input: "CREATE TABLE t (a SET)",
		Error: true,
	})
	parse("QuotedNumberDefault", &Spec{
		Input: "CREATE TABLE t (a INT DEFAULT '-1', b DOUBLE DEFAULT '1.5e-3', c BOOL DEFAULT '1')",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT -1,\n" +
			"`b` DOUBLE DEFAULT 1.5E-3,\n" +
			"`c` TINYINT (1) DEFAULT 1\n" +
			");\n",
	})
	parse("QuotedNonNumberDefault", &Spec{
		// MySQL rejects these defaults, but keep the quotes to make valid SQL.
		Input: "CREATE TABLE t (a INT DEFAULT '', b INT DEFAULT 'abc', c BOOL DEFAULT 'true')",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT '',\n" +
			"`b` INT (11) DEFAULT 'abc',\n" +
			"`c` TINYINT (1) DEFAULT 'true'\n" +
			");\n",
	})
	parse("NulInColumnName", &Spec{
		Input: "CREATE TABLE t (a\x00 TEXT)",
		Error: true,
	})
	parse("NulInDefault", &Spec{
		Input: "CREATE TABLE t (a VARCHAR(10) DEFAULT 'a\x00b')",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` VARCHAR (10) DEFAULT 'a\x00b'\n" +
			");\n",
	})
	parse("PrimaryKeyAndUniqueColumn", &Spec{
		Input: "CREATE TABLE t (a INT PRIMARY KEY UNIQUE)",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT NULL,\n" +
			"PRIMARY KEY (`a`),\n" +
			"UNIQUE INDEX `a` (`a`)\n" +
			");\n",
	})
	parse("QuotedTableOptionIdent", &Spec{
		Input: "CREATE TABLE t (a INT) ENGINE `InnoDB`, DEFAULT CHARSET `my charset`",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT NULL\n" +
			") ENGINE = InnoDB, DEFAULT CHARACTER SET = `my charset`;\n",
	})
	parse("QuoteInTableComment", &Spec{
		Input: `CREATE TABLE t (a INT) COMMENT = "it's"`,
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT NULL\n" +
			") COMMENT = 'it''s';\n",
	})

	// https://github.com/shogo82148/schemalex-deploy/issues/312
	// The content of executable comments is parsed as SQL.
	// The version number is ignored, i.e. the latest MySQL is assumed.
	// https://dev.mysql.com/doc/refman/26.10/en/comments.html
	parse("StatementsWithoutSpaces", &Spec{
		Input: "CREATE TABLE a (x INT);CREATE TABLE b (y INT)",
		Expect: "CREATE TABLE `a` (\n" +
			"`x` INT (11) DEFAULT NULL\n" +
			");\n" +
			"CREATE TABLE `b` (\n" +
			"`y` INT (11) DEFAULT NULL\n" +
			");\n",
	})
	parse("ExecutableCommentColumnOptions", &Spec{
		Input: "CREATE TABLE t (a INT /*! NOT NULL */, b INT /*!99999 DEFAULT 1 */, c INT /*!100000 COMMENT 'c' */)",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"`b` INT (11) DEFAULT 1,\n" +
			"`c` INT (11) DEFAULT NULL COMMENT 'c'\n" +
			");\n",
	})
	parse("ExecutableCommentStatement", &Spec{
		Input: "/*!40101 CREATE TABLE t (a INT) */;",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT NULL\n" +
			");\n",
	})
	parse("ExecutableCommentTableOptions", &Spec{
		Input: "CREATE TABLE t (a INT) /*!50000 ENGINE=InnoDB */ /*!50100 DEFAULT CHARSET=utf8mb4 */",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT NULL\n" +
			") ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4;\n",
	})
	// https://github.com/shogo82148/schemalex-deploy/issues/327
	parse("MariaDBTextDefaultNull", &Spec{
		// MariaDB shows DEFAULT NULL for nullable TEXT and BLOB columns, but MySQL does not.
		Input: "CREATE TABLE `t` (\n" +
			"  `a` text DEFAULT NULL,\n" +
			"  `b` mediumblob DEFAULT NULL,\n" +
			"  `c` text NOT NULL,\n" +
			"  `d` text DEFAULT 'NULL'\n" +
			")",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` TEXT,\n" +
			"`b` MEDIUMBLOB,\n" +
			"`c` TEXT NOT NULL,\n" +
			"`d` TEXT DEFAULT 'NULL'\n" +
			");\n",
	})

	// https://github.com/shogo82148/schemalex-deploy/issues/326
	parse("MariaDBJSONColumn", &Spec{
		// MariaDB returns JSON columns as LONGTEXT with a CHECK constraint.
		Input: "CREATE TABLE `t` (\n" +
			"  `doc` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin DEFAULT NULL CHECK (json_valid(`doc`))\n" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci",
		Expect: "CREATE TABLE `t` (\n" +
			"`doc` JSON DEFAULT NULL\n" +
			") ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4, DEFAULT COLLATE = utf8mb4_general_ci;\n",
	})
	parse("MariaDBJSONColumnNotNull", &Spec{
		Input: "CREATE TABLE `t` (\n" +
			"  `doc` longtext CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL CHECK (json_valid(`doc`))\n" +
			")",
		Expect: "CREATE TABLE `t` (\n" +
			"`doc` JSON NOT NULL\n" +
			");\n",
	})
	parse("MariaDBJSONColumnHandWritten", &Spec{
		// the function name and the column name are case-insensitive.
		Input: "CREATE TABLE t (Doc LONGTEXT CHECK (JSON_VALID( doc )))",
		Expect: "CREATE TABLE `t` (\n" +
			"`Doc` JSON DEFAULT NULL\n" +
			");\n",
	})
	// other CHECK constraints on columns are not supported.
	parse("ColumnCheck", &Spec{
		Input: "CREATE TABLE t (a INT CHECK (a > 0))",
		Error: true,
	})
	parse("ColumnCheckJSONValidOtherColumn", &Spec{
		Input: "CREATE TABLE t (doc LONGTEXT, doc2 LONGTEXT CHECK (json_valid(doc)))",
		Error: true,
	})
	parse("ColumnCheckJSONValidText", &Spec{
		// MariaDB uses LONGTEXT for JSON columns.
		Input: "CREATE TABLE t (doc TEXT CHECK (json_valid(doc)))",
		Error: true,
	})
	parse("ColumnCheckJSONValidOtherCharset", &Spec{
		// MariaDB uses utf8mb4 and utf8mb4_bin for JSON columns.
		Input: "CREATE TABLE t (doc LONGTEXT CHARACTER SET latin1 CHECK (json_valid(doc)))",
		Error: true,
	})
	parse("ColumnCheckJSONValidOtherCollation", &Spec{
		Input: "CREATE TABLE t (doc LONGTEXT CHECK (json_valid(doc)) COLLATE utf8mb4_general_ci)",
		Error: true,
	})
	parse("ColumnCheckEmpty", &Spec{
		Input: "CREATE TABLE t (doc LONGTEXT CHECK ())",
		Error: true,
	})

	parse("MariaDBExecutableComment", &Spec{
		// MySQL treats MariaDB-specific executable comments as ordinary comments.
		Input: "CREATE TABLE t (a INT /*M!100100 NOT NULL */)",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT NULL\n" +
			");\n",
	})

	// Unsupported syntax in executable comments is an error, as it is outside of them.
	parse("UnsupportedStatementInExecutableComment", &Spec{
		// a part of mysqldump output for views.
		Input: "/*!50001 CREATE ALGORITHM=UNDEFINED */\n" +
			"/*!50013 DEFINER=`root`@`localhost` SQL SECURITY DEFINER */\n" +
			"/*!50001 VIEW `v` AS select 1 AS `a` */;\n" +
			"CREATE TABLE t (a INT);",
		Error: true,
	})
	parse("MysqldumpCreateDatabase", &Spec{
		// the options of CREATE DATABASE are not supported.
		Input: "CREATE DATABASE /*!32312 IF NOT EXISTS*/ `zz_db` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci */ /*!80016 DEFAULT ENCRYPTION='N' */;\n" +
			"USE `zz_db`;\n" +
			"CREATE TABLE t (a INT);",
		Error: true,
	})
	parse("IncompleteClauseInExecutableComment", &Spec{
		Input: "CREATE TABLE t (a INT /*! NOT */)",
		Error: true,
	})
	parse("DuplicatedIndexTypeInExecutableComment", &Spec{
		Input: "CREATE TABLE t (a INT, INDEX i USING BTREE (a) /*! USING HASH */)",
		Error: true,
	})
	parse("UnterminatedExecutableComment", &Spec{
		Input: "CREATE TABLE t (a INT /*! NOT NULL",
		Error: true,
	})
	parse("UnterminatedExecutableCommentAfterStatement", &Spec{
		Input: "/*! CREATE TABLE t (a INT)",
		Error: true,
	})
	parse("UnterminatedStringInExecutableComment", &Spec{
		// MySQL also rejects it, because the string literal does not end at "*/".
		Input: "/*! 'abc */; CREATE TABLE t (a INT);",
		Error: true,
	})

	// https://github.com/shogo82148/schemalex-deploy/issues/320
	// https://dev.mysql.com/doc/refman/26.10/en/invisible-indexes.html
	// https://dev.mysql.com/doc/refman/26.10/en/invisible-columns.html
	parse("InvisibleIndex", &Spec{
		Input: "CREATE TABLE t (a INT, b INT, c TEXT, d INT, " +
			"KEY idx_a (a) INVISIBLE, " +
			"UNIQUE KEY uniq_b (b) USING BTREE INVISIBLE, " +
			"FULLTEXT KEY ft (c) INVISIBLE WITH PARSER ngram, " +
			"KEY idx_d (d) VISIBLE)",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT NULL,\n" +
			"`b` INT (11) DEFAULT NULL,\n" +
			"`c` TEXT,\n" +
			"`d` INT (11) DEFAULT NULL,\n" +
			"INDEX `idx_a` (`a`) INVISIBLE,\n" +
			"UNIQUE INDEX `uniq_b` USING BTREE (`b`) INVISIBLE,\n" +
			"FULLTEXT INDEX `ft` (`c`) WITH PARSER `ngram` INVISIBLE,\n" +
			"INDEX `idx_d` (`d`)\n" +
			");\n",
	})
	parse("InvisibleIndexLastOneWins", &Spec{
		Input: "CREATE TABLE t (a INT, b INT, KEY idx_a (a) VISIBLE INVISIBLE, KEY idx_b (b) INVISIBLE VISIBLE)",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) DEFAULT NULL,\n" +
			"`b` INT (11) DEFAULT NULL,\n" +
			"INDEX `idx_a` (`a`) INVISIBLE,\n" +
			"INDEX `idx_b` (`b`)\n" +
			");\n",
	})
	parse("InvisiblePrimaryKey", &Spec{
		// MySQL rejects it with "A primary key index cannot be invisible".
		Input: "CREATE TABLE t (a INT, PRIMARY KEY (a) INVISIBLE)",
		Error: true,
	})
	parse("VisiblePrimaryKey", &Spec{
		Input: "CREATE TABLE t (a INT NOT NULL, PRIMARY KEY (a) VISIBLE)",
		Expect: "CREATE TABLE `t` (\n" +
			"`a` INT (11) NOT NULL,\n" +
			"PRIMARY KEY (`a`)\n" +
			");\n",
	})
	parse("InvisibleForeignKey", &Spec{
		// foreign keys do not have the visibility.
		Input: "CREATE TABLE t (a INT, FOREIGN KEY (a) REFERENCES u (a) INVISIBLE)",
		Error: true,
	})
	parse("InvisibleColumn", &Spec{
		Input: "CREATE TABLE t (id INT NOT NULL AUTO_INCREMENT INVISIBLE PRIMARY KEY, a INT VISIBLE, b INT INVISIBLE COMMENT 'b', c INT AS (a + 1) STORED INVISIBLE)",
		Expect: "CREATE TABLE `t` (\n" +
			"`id` INT (11) NOT NULL AUTO_INCREMENT INVISIBLE,\n" +
			"`a` INT (11) DEFAULT NULL,\n" +
			"`b` INT (11) DEFAULT NULL INVISIBLE COMMENT 'b',\n" +
			"`c` INT (11) GENERATED ALWAYS AS (a + 1) STORED INVISIBLE,\n" +
			"PRIMARY KEY (`id`)\n" +
			");\n",
	})
	parse("InvisibleInShowCreateTable", &Spec{
		// the output of SHOW CREATE TABLE in MySQL 26.10.
		Input: "CREATE TABLE `t` (\n" +
			"  `id` int NOT NULL AUTO_INCREMENT /*!80023 INVISIBLE */,\n" +
			"  `a` int DEFAULT NULL,\n" +
			"  `body` text,\n" +
			"  PRIMARY KEY (`id`),\n" +
			"  KEY `idx_a` (`a`) /*!80000 INVISIBLE */,\n" +
			"  FULLTEXT KEY `ft` (`body`) /*!80000 INVISIBLE */ /*!50100 WITH PARSER `ngram` */ \n" +
			") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci",
		Expect: "CREATE TABLE `t` (\n" +
			"`id` INT (11) NOT NULL AUTO_INCREMENT INVISIBLE,\n" +
			"`a` INT (11) DEFAULT NULL,\n" +
			"`body` TEXT,\n" +
			"PRIMARY KEY (`id`),\n" +
			"INDEX `idx_a` (`a`) INVISIBLE,\n" +
			"FULLTEXT INDEX `ft` (`body`) WITH PARSER `ngram` INVISIBLE\n" +
			") ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4, DEFAULT COLLATE = utf8mb4_0900_ai_ci;\n",
	})
}

func TestFormatRoundTrip(t *testing.T) {
	tests := []string{
		"CREATE TABLE t (a INT, b INT INVISIBLE, KEY (a) INVISIBLE, KEY (b) VISIBLE)",
		"CREATE TABLE t (a TEXT, FULLTEXT KEY (a) WITH PARSER ngram INVISIBLE)",
		"CREATE TABLE t (a VARCHAR(10) DEFAULT 'it''s')",
		"CREATE TABLE t (a VARCHAR(10) DEFAULT 'it\\'s')",
		`CREATE TABLE t (a VARCHAR(10) DEFAULT "'")`,
		`CREATE TABLE t (a VARCHAR(10) DEFAULT "it\'s")`,
		`CREATE TABLE t (a VARCHAR(10) DEFAULT "say ""hi"" \"x\"")`,
		`CREATE TABLE t (a VARCHAR(10) DEFAULT 'a\n''b')`,
		"CREATE TABLE t (a INT COMMENT 'it''s')",
		`CREATE TABLE t (a ENUM('it''s', "b'") DEFAULT 'it''s')`,
		"CREATE TABLE t (a SET('it''s', 'b'))",
		`CREATE TABLE t (a INT) COMMENT = "it's"`,
		"CREATE TABLE t (a INT) COMMENT = 'it''s'",
		"CREATE TABLE t (a INT DEFAULT '')",
		"CREATE TABLE t (a INT DEFAULT 'abc')",
		"CREATE TABLE t (a INT DEFAULT '1e5')",
		"CREATE TABLE t (a INT DEFAULT '+1')",
		"CREATE TABLE t (a BOOL DEFAULT A)",
		"CREATE TABLE t (a BOOL DEFAULT 'true')",
		"CREATE TABLE t (a BOOL DEFAULT '1')",
		"CREATE TABLE t (a BOOL DEFAULT TRUE)",
		"CREATE TABLE t (a TINYINT(1) DEFAULT TRUE)",
		"CREATE TABLE t (a VARCHAR(10) DEFAULT 'a\x00b')",
		"CREATE TABLE `a\x00b` (`c\x00d` INT)",
		"CREATE TABLE t (a INT PRIMARY KEY UNIQUE)",
		"CREATE TABLE t (a INT UNIQUE PRIMARY KEY)",
		"CREATE TABLE t (a INT) ENGINE ``",
		"CREATE TABLE t (a INT) ENGINE `TABLE`",
		"CREATE TABLE t (a INT) ENGINE `default`",
		"CREATE TABLE t (a INT) ENGINE `my engine`",
		"CREATE TABLE t (a INT) ENGINE `a``b`",
		"CREATE TABLE t (a INT) DEFAULT CHARACTER SET `utf8mb4` COLLATE `utf8mb4 bin`",
		"CREATE TEMPORARY TABLE t (a INT)",
		"CREATE TEMPORARY TABLE IF NOT EXISTS t LIKE u",
		"CREATE TABLE t (id INT) ENGINE InnoDB PARTITION BY HASH (id) PARTITIONS 2",
		"CREATE TABLE t (id INT) PARTITION BY HASH (id * 2) PARTITIONS 2",
		"CREATE TABLE t (id INT) PARTITION BY HASH (id % 3) PARTITIONS 2",
		"CREATE TABLE t (doc LONGTEXT CHECK (json_valid(doc)))",
		"CREATE TABLE t (a INT, b INT AS (a # comment\n + 1))",
		"CREATE TABLE t (a INT, b INT AS (a /* comment */))",
		"CREATE TABLE t (id INT) PARTITION BY RANGE (id) (PARTITION p0 VALUES LESS THAN (10), PARTITION p1 VALUES LESS THAN MAXVALUE)",
	}
	for _, ddl0 := range tests {
		t.Run(ddl0, func(t *testing.T) {
			p := schemalex.New()
			stmts0, err := p.ParseString(ddl0)
			if err != nil {
				t.Fatal(err)
			}

			var buf strings.Builder
			if err := SQL(&buf, stmts0); err != nil {
				t.Fatal(err)
			}
			ddl1 := buf.String()

			stmts1, err := p.ParseString(ddl1)
			if err != nil {
				t.Fatalf("failed to parse %q: %v", ddl1, err)
			}
			if diff := cmp.Diff(stmts0, stmts1); diff != "" {
				t.Errorf("(-want/+got):\n%s", diff)
			}
		})
	}
}

func TestFormatEnumWithoutValues(t *testing.T) {
	tests := []*model.TableColumn{
		{Name: "a", Type: model.ColumnTypeEnum},
		{Name: "a", Type: model.ColumnTypeSet},
	}
	for _, col := range tests {
		t.Run(col.Type.String(), func(t *testing.T) {
			table := model.NewTable("t")
			table.Columns = append(table.Columns, col)
			var buf strings.Builder
			if err := SQL(&buf, table); err == nil {
				t.Errorf("want error, got %q", buf.String())
			}
		})
	}
}
