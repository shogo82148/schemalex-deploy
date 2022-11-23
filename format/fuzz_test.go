//go:build go1.19
// +build go1.19

package format

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy"
)

func FuzzFormat(f *testing.F) {
	f.Add("create table hoge_table ( id integer unsigned not null)")
	f.Add("create table hoge ( /* id integer unsigned not null */ c varchar not null )")
	f.Add("create table hoge ( -- id integer unsigned not null;\n" +
		" c varchar not null )")
	f.Add(`create table hoge (
		a varchar(20) default "hoge",
		b varchar(20) default 'hoge',
		c int not null default 10
		);
		`)
	f.Add(`create table hoge (
			id bigint unsigned not null auto_increment,
			c varchar(20) not null default "hoge",
			primary key (id, c)
			);
			`)
	f.Add("create table hoge (" +
		"id bigint unsigned not null auto_increment" +
		") ENGINE=InnoDB AUTO_INCREMENT 10 DEFAULT CHARACTER SET = utf8 COMMENT = 'hoge comment';")
	f.Add("create table hoge (" +
		"id bigint unsigned not null auto_increment" +
		") ENGINE=InnoDB AUTO_INCREMENT 10 DEFAULT CHARSET = utf8 COMMENT = 'hoge comment';")
	f.Add("create table hoge (" +
		"`id` bigint unsigned not null auto_increment,\n" +
		"`c` varchar(20) not null,\n" +
		"KEY (`id`), INDEX (`c`)\n" +
		")")
	f.Add("create table hoge (" +
		"`id` bigint unsigned not null auto_increment,\n" +
		"`c` varchar(20) not null,\n" +
		"UNIQUE INDEX `uniq_id` (`id`, `c`),\n" +
		"PRIMARY KEY (`id`)\n" +
		")")
	f.Add("create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
		"`c` varchar(20) not null,\n" +
		"FOREIGN KEY `fk_c` (`c`) )")
	f.Add("create table hoge (txt TEXT, fulltext ft_idx(txt))")
	f.Add("create table hoge (txt TEXT, fulltext index ft_idx(txt))")
	f.Add("create table hoge (txt TEXT, fulltext key ft_idx(txt))")
	f.Add("create table hoge (txt TEXT, fulltext ft_idx(txt) with parser ngram)")
	f.Add("create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
		"`c` varchar(20) not null,\n" +
		"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) )")
	f.Add("create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
		"`c` varchar(20) not null,\n" +
		"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) MATCH SIMPLE )")
	f.Add("create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
		"`c` varchar(20) not null,\n" +
		"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) ON DELETE SET NULL)")
	f.Add("create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
		"`c` varchar(20) not null,\n" +
		"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) MATCH PARTIAL ON DELETE NO ACTION)")
	f.Add("create table hoge ( `id` bigint unsigned not null auto_increment,\n" +
		"`c` varchar(20) not null,\n" +
		"FOREIGN KEY `fk_c` (`c`) REFERENCES `fuga` (`id`) ON DELETE NO ACTION ON UPDATE CASCADE)")
	f.Add("create table hoge (`foo` DECIMAL(32,30))")
	f.Add("CREATE TABLE `fuga` ( `id` INTEGER NOT NULL AUTO_INCREMENT, CONSTRAINT `symbol` UNIQUE KEY `uniq_id` USING BTREE (`id`) )")
	f.Add("DROP TABLE IF EXISTS `konboi_bug`; CREATE TABLE foo(`id` INT)")
	f.Add("CREATE TABLE `foo` (col TEXT CHARACTER SET latin1)")
	f.Add("CREATE TABLE `foo` (col DATETIME ON UPDATE CURRENT_TIMESTAMP)")
	f.Add("CREATE TABLE `foo` (col TEXT, KEY col_idx (col(196)))")
	f.Add("CREATE TABLE foo LIKE bar")
	f.Add("CREATE TABLE foo (id INTEGER PRIMARY KEY AUTO_INCREMENT)")
	f.Add("CREATE TABLE `test` (\n" +
		"`id` int(11) PRIMARY KEY COMMENT 'aaa' NOT NULL,\n" +
		"hoge int default 1 not null COMMENT 'bbb' UNIQUE\n" +
		");")
	f.Add("CREATE TABLE `test` (\n" +
		"`id` int(11) COMMENT 'aaa' PRIMARY KEY NOT NULL,\n" +
		"hoge int default 1 UNIQUE not null COMMENT 'bbb'\n" +
		");")
	f.Add("CREATE TABLE `test` (\n" +
		"`status` ENUM('on', 'off') NOT NULL DEFAULT 'off'\n" +
		");")
	f.Add("CREATE TABLE `test` (\n" +
		"`status` SET('foo', 'bar', 'baz') NOT NULL DEFAULT 'foo,baz'\n" +
		");")
	f.Add("CREATE TABLE `test` (\n" +
		"`valid` BOOLEAN not null default true\n" +
		");")
	f.Add("CREATE TABLE `test` (\n`valid` BOOLEAN not null default false\n);")
	f.Add("CREATE TABLE `test` (\n`valid` BOOL not null default true\n);")
	f.Add("CREATE TABLE `test` (\n`valid` BOOL not null default false\n);")
	f.Add("CREATE TABLE `test` (\n`valid` JSON not null\n);")
	f.Add("CREATE TABLE `test` (\n`valid` GEOMETRY not null\n);")
	f.Add("CREATE TABLE IF NOT EXISTS `test` (\n`id` INT (10) NOT NULL\n);")
	f.Add("CREATE TABLE foo (id INT(10) NOT NULL) ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4")
	f.Add("DROP TABLE IF EXISTS `socialaccount_socialtoken`;\n" +
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
		") ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci ROW_FORMAT=DYNAMIC;")
	f.Add("/* hello, world*/;\n" +
		"CREATE TABLE foo (\n" +
		"a int);\n" +
		"/* hello, world again! */;\n" +
		"CREATE TABLE bar (\n" +
		"b int);")
	f.Add("DROP TABLE IF EXISTS `some_table`;\r\n" +
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
		") ENGINE=InnoDB AUTO_INCREMENT=19 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;")
	f.Add("create table `test_log` (`created_at` DATETIME default NOW())")
	f.Add("CREATE TABLE `test_tb` (" +
		"  `t_id` char(17) NOT NULL," +
		"  `t_type` smallint(6) NOT NULL," +
		"  `cur_date` datetime NOT NULL" +
		") ENGINE=InnoDB DEFAULT CHARSET=utf8" +
		"/*!50100 PARTITION BY LIST (`t_type`)" +
		"(PARTITION p_1 VALUES IN (1) ENGINE = InnoDB," +
		" PARTITION p_100 VALUES IN (100) ENGINE = InnoDB) */;" +
		"/*!40101 SET character_set_client = @saved_cs_client */;")
	f.Add("CREATE TABLE foo (id INT(10) NOT NULL) ENGINE = InnoDB, DEFAULT CHARACTER SET = utf8mb4 \n/**/ ;")

	f.Fuzz(func(t *testing.T, ddl0 string) {
		p := schemalex.New()
		stmts0, err := p.ParseString(ddl0)
		if err != nil {
			return
		}

		var buf strings.Builder
		if err := SQL(&buf, stmts0); err != nil {
			t.Errorf("format.SQL returns unexpected error: %v", err)
			return
		}
		ddl1 := buf.String()

		stmts1, err := p.ParseString(ddl1)
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(stmts0, stmts1); diff != "" {
			t.Errorf("%s", diff)
		}
	})
}
