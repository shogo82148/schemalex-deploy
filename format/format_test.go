package format

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy"
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
			"INDEX `created_at` (`created_at` DESC),\n" +
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
			") ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8;\n",
	})
	parse("WhiteSpacesBetweenTableOptionsAndSemicolon", &Spec{
		Input: "CREATE TABLE foo (id INT(10) NOT NULL) ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4 \n/**/ ;",
		Expect: "CREATE TABLE `foo` (\n" +
			"`id` INT (10) NOT NULL\n" +
			") ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4;\n",
	})
}
