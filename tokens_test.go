package schemalex

import (
	"strings"
	"testing"

	"github.com/shogo82148/schemalex-deploy/internal/keywords"
)

func TestTokenTypeKeyword(t *testing.T) {
	testCases := []struct {
		typ      TokenType
		keyword  bool
		reserved bool
	}{
		// not keywords
		{typ: ILLEGAL},
		{typ: EOF},
		{typ: SPACE},
		{typ: IDENT},
		{typ: BACKTICK_IDENT},
		{typ: LPAREN},
		{typ: COMMENT_IDENT},

		// non-reserved keywords
		{typ: ACTION, keyword: true},
		{typ: DATE, keyword: true},
		{typ: TEXT, keyword: true},
		{typ: YEAR, keyword: true},

		// reserved keywords
		{typ: ASC, keyword: true, reserved: true},
		{typ: TABLE, keyword: true, reserved: true},
		{typ: ZEROFILL, keyword: true, reserved: true},
	}
	for _, tc := range testCases {
		t.Run(tc.typ.String(), func(t *testing.T) {
			if got := tc.typ.IsKeyword(); got != tc.keyword {
				t.Errorf("%s.IsKeyword() = %t, want %t", tc.typ, got, tc.keyword)
			}
			if got := tc.typ.IsReservedKeyword(); got != tc.reserved {
				t.Errorf("%s.IsReservedKeyword() = %t, want %t", tc.typ, got, tc.reserved)
			}
		})
	}
}

// keywords.IsReserved must agree with the lexer.
func TestKeywordsIsReserved(t *testing.T) {
	for name, typ := range keywordIdentMap {
		if got, want := keywords.IsReserved(name), typ.IsReservedKeyword(); got != want {
			t.Errorf("keywords.IsReserved(%q) = %v, want %v", name, got, want)
		}
		if got, want := keywords.IsReserved(strings.ToLower(name)), typ.IsReservedKeyword(); got != want {
			t.Errorf("keywords.IsReserved(%q) = %v, want %v", strings.ToLower(name), got, want)
		}
	}
	if keywords.IsReserved("InnoDB") {
		t.Error("InnoDB must not be reserved")
	}
}
