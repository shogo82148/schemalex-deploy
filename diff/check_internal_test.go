package diff

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestExprIdents(t *testing.T) {
	tests := []struct {
		expr string
		want []string
	}{
		{"a > 0", []string{"a", "0"}},
		{"`a` < `b`", []string{"a", "b"}},
		{"`a``b` <> `c d`", []string{"a`b", "c d"}},
		{"concat(`a`, b, 'c') = _utf8mb4'x'", []string{"concat", "a", "b", "_utf8mb4"}},
		// the identifiers in string literals are skipped.
		{"a IN ('b', \"c\", 'it''s d', 'e\\'f')", []string{"a", "IN"}},
		{"$a + b_1 + é", []string{"$a", "b_1", "é"}},
		{"t.a > 0", []string{"t", "a", "0"}},
		// the quotes in comments are not the start of string literals.
		{"a > 0 /* ' */ AND b > 0", []string{"a", "0", "AND", "b", "0"}},
		{"a > 0 # it's a\nAND b > 0", []string{"a", "0", "AND", "b", "0"}},
		{"a > 0 -- it's a\nAND b > 0", []string{"a", "0", "AND", "b", "0"}},
		{"a > 0 --\tit's a\nAND b > 0", []string{"a", "0", "AND", "b", "0"}},
		// `--` without a space is not a comment, e.g. `a - (-b)`.
		{"a--b", []string{"a", "b"}},
		{"a /* b", []string{"a"}},
		{"a # b", []string{"a"}},
		// unterminated quotes.
		{"a = 'b", []string{"a"}},
		{"`a", []string{"a"}},
	}
	for _, tt := range tests {
		got := exprIdents(tt.expr)
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("exprIdents(%q) (-want/+got)\n%s", tt.expr, diff)
		}
	}
}

func TestTableRefs(t *testing.T) {
	tests := []struct {
		stmt string
		want []string
	}{
		{"SELECT * FROM t", []string{"t"}},
		{"SELECT a, b FROM `t` AS a WHERE b > 0", []string{"t"}},
		{"SELECT * FROM t a, u b", []string{"t", "u"}},
		{"SELECT * FROM db.t JOIN `db`.`u` USING (id), v", []string{"t", "u", "v"}},
		{"SELECT * FROM t LEFT JOIN u ON t.id = u.id STRAIGHT_JOIN v", []string{"t", "u", "v"}},
		{"SELECT x FROM (SELECT a AS x FROM t) AS d, u", []string{"t", "u"}},
		// parenthesized table references.
		{"SELECT * FROM (t)", []string{"t"}},
		{"SELECT * FROM (t JOIN u ON t.id = u.id), v", []string{"t", "u", "v"}},
		{"SELECT * FROM ((t, u)) JOIN (v)", []string{"t", "u", "v"}},
		{"SELECT * FROM t JOIN LATERAL (SELECT a FROM u) AS d", []string{"t", "u"}},
		{"SELECT f(a, b) FROM t", []string{"t"}},
		{"SELECT (SELECT max(a) FROM t) AS m, b FROM u", []string{"t", "u"}},
		{"SELECT a FROM t UNION SELECT a FROM u", []string{"t", "u"}},
		{"SELECT * FROM t WHERE a IN (SELECT a FROM u)", []string{"t", "u"}},
		// common table expressions.
		{"WITH c AS (SELECT * FROM t) SELECT * FROM c", []string{"t"}},
		{"WITH RECURSIVE c (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM c) SELECT * FROM c, t", []string{"t"}},
		{"WITH c AS (SELECT 1), d AS (SELECT * FROM t) SELECT * FROM c JOIN d", []string{"t"}},
		{"SELECT a FROM t GROUP BY a WITH ROLLUP", []string{"t"}},
		// string literals and comments.
		{"SELECT 'FROM x' FROM /* FROM y */ t -- FROM z\n", []string{"t"}},
		{"SELECT 1", nil},
	}
	for _, tt := range tests {
		got := tableRefs(tt.stmt)
		if len(got) == 0 {
			got = nil
		}
		if diff := cmp.Diff(tt.want, got); diff != "" {
			t.Errorf("tableRefs(%q) (-want/+got)\n%s", tt.stmt, diff)
		}
	}
}
