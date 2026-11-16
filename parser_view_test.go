package schemalex_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy"
	"github.com/shogo82148/schemalex-deploy/model"
)

func TestParseView(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want *model.View
	}{
		{
			name: "simple",
			src:  "CREATE VIEW v AS SELECT * FROM t WHERE a > 1;",
			want: &model.View{
				Name:   "v",
				Select: "SELECT * FROM t WHERE a > 1",
			},
		},
		{
			name: "all options",
			src: "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = 'root'@'localhost' SQL SECURITY INVOKER " +
				"VIEW `v` (`x`, y) AS SELECT a, b FROM t WITH LOCAL CHECK OPTION",
			want: &model.View{
				Name:        "v",
				OrReplace:   true,
				Algorithm:   model.ViewAlgorithmMerge,
				Definer:     model.MaybeString{Valid: true, Value: "'root'@'localhost'"},
				SQLSecurity: model.ViewSQLSecurityInvoker,
				Columns:     []model.Ident{"x", "y"},
				Select:      "SELECT a, b FROM t",
				CheckOption: model.ViewCheckOptionLocal,
			},
		},
		{
			name: "SHOW CREATE VIEW",
			src:  "CREATE ALGORITHM=UNDEFINED DEFINER=`root`@`localhost` SQL SECURITY DEFINER VIEW `v` AS select `t`.`a` AS `a` from `t` WITH CASCADED CHECK OPTION",
			want: &model.View{
				Name:        "v",
				Algorithm:   model.ViewAlgorithmUndefined,
				Definer:     model.MaybeString{Valid: true, Value: "`root`@`localhost`"},
				SQLSecurity: model.ViewSQLSecurityDefiner,
				Select:      "select `t`.`a` AS `a` from `t`",
				CheckOption: model.ViewCheckOptionCascaded,
			},
		},
		{
			name: "mysqldump",
			src:  "/*!50001 CREATE ALGORITHM=UNDEFINED */\n/*!50013 DEFINER=`root`@`localhost` SQL SECURITY DEFINER */\n/*!50001 VIEW `v` AS select 1 AS `a` */;",
			want: &model.View{
				Name:        "v",
				Algorithm:   model.ViewAlgorithmUndefined,
				Definer:     model.MaybeString{Valid: true, Value: "`root`@`localhost`"},
				SQLSecurity: model.ViewSQLSecurityDefiner,
				Select:      "select 1 AS `a`",
			},
		},
		{
			name: "WITH CHECK OPTION is CASCADED",
			src:  "CREATE VIEW v AS SELECT a FROM t WITH CHECK OPTION",
			want: &model.View{
				Name:        "v",
				Select:      "SELECT a FROM t",
				CheckOption: model.ViewCheckOptionCascaded,
			},
		},
		{
			name: "WITH ROLLUP",
			src:  "CREATE VIEW v AS SELECT a, COUNT(*) FROM t GROUP BY a WITH ROLLUP",
			want: &model.View{
				Name:   "v",
				Select: "SELECT a, COUNT(*) FROM t GROUP BY a WITH ROLLUP",
			},
		},
		{
			name: "common table expression",
			src:  "CREATE VIEW v AS WITH cascaded AS (SELECT 1 AS a) SELECT a FROM cascaded",
			want: &model.View{
				Name:   "v",
				Select: "WITH cascaded AS (SELECT 1 AS a) SELECT a FROM cascaded",
			},
		},
		{
			name: "comments and semicolons in strings",
			src:  "CREATE VIEW v AS SELECT ';' /* comment */ AS a -- trailing comment\n;",
			want: &model.View{
				Name:   "v",
				Select: "SELECT ';' /* comment */ AS a",
			},
		},
		{
			name: "IF NOT EXISTS and CURRENT_USER",
			src:  "CREATE DEFINER = CURRENT_USER() VIEW IF NOT EXISTS v AS SELECT 1",
			want: &model.View{
				Name:        "v",
				IfNotExists: true,
				Definer:     model.MaybeString{Valid: true, Value: "CURRENT_USER()"},
				Select:      "SELECT 1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := schemalex.New().ParseString(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(model.Stmts{tt.want}, got); diff != "" {
				t.Errorf("(-want/+got):\n%s", diff)
			}
		})
	}
}

func TestParseView_Multiple(t *testing.T) {
	src := "CREATE TABLE t (a INT);\nCREATE VIEW v AS SELECT a FROM t;\nCREATE TABLE u (b INT);"
	got, err := schemalex.New().ParseString(src)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, stmt := range got {
		ids = append(ids, stmt.ID())
	}
	if diff := cmp.Diff([]string{"table#t", "view#v", "table#u"}, ids); diff != "" {
		t.Errorf("(-want/+got):\n%s", diff)
	}
}

func TestParseView_Error(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{"CREATE OR VIEW v AS SELECT 1", "expected REPLACE"},
		{"CREATE ALGORITHM = FAST VIEW v AS SELECT 1", "expected UNDEFINED, MERGE or TEMPTABLE"},
		{"CREATE ALGORITHM MERGE VIEW v AS SELECT 1", "expected EQUAL"},
		{"CREATE DEFINER = VIEW v AS SELECT 1", "expected the user of DEFINER"},
		{"CREATE SQL SECURITY NOBODY VIEW v AS SELECT 1", "expected DEFINER or INVOKER"},
		{"CREATE OR REPLACE TABLE t (a INT)", "expected VIEW"},
		{"CREATE OR REPLACE VIEW IF NOT EXISTS v AS SELECT 1", "OR REPLACE and IF NOT EXISTS cannot be used together"},
		{"CREATE VIEW v (a, ) AS SELECT 1", "expected IDENT or BACKTICK_IDENT"},
		{"CREATE VIEW v SELECT 1", "expected AS"},
		{"CREATE VIEW v AS ;", "expected the select statement of the view"},
		{"CREATE VIEW v AS SELECT (1", "expected RPAREN in the select statement of the view"},
		{"CREATE VIEW v AS SELECT 1)", "unexpected RPAREN in the select statement of the view"},
		{"CREATE VIEW v AS SELECT 'abc", "unterminated quoted string"},
		{"CREATE VIEW v AS SELECT 1 WITH CHECK", "expected OPTION"},
		{"CREATE VIEW v AS SELECT 1 WITH CHECK OPTION SELECT", "expected SEMICOLON or EOF"},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			_, err := schemalex.New().ParseString(tt.src)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("want error contains %q, got %v", tt.want, err)
			}
		})
	}
}
