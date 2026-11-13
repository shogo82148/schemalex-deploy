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
