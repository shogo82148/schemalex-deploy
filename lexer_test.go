package schemalex

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLexToken(t *testing.T) {
	type Spec struct {
		input string
		token Token
	}

	specs := []Spec{
		// number
		{
			input: "123",
			token: Token{Value: "123", Type: NUMBER},
		},
		{
			input: ".2",
			token: Token{Value: ".2", Type: NUMBER},
		},
		{
			input: "3.4",
			token: Token{Value: "3.4", Type: NUMBER},
		},
		{
			input: "-5",
			token: Token{Value: "-5", Type: NUMBER},
		},
		{
			input: "-6.78",
			token: Token{Value: "-6.78", Type: NUMBER},
		},
		{
			input: "+9.10",
			token: Token{Value: "+9.10", Type: NUMBER},
		},
		{
			input: "1.2E3",
			token: Token{Value: "1.2E3", Type: NUMBER},
		},
		{
			input: "1.2E-3",
			token: Token{Value: "1.2E-3", Type: NUMBER},
		},
		{
			input: "-1.2E3",
			token: Token{Value: "-1.2E3", Type: NUMBER},
		},
		{
			input: "-1.2E-3",
			token: Token{Value: "-1.2E-3", Type: NUMBER},
		},
		// SINGLE_QUOTE_IDENT
		{
			input: `'hoge'`,
			token: Token{Value: `hoge`, Type: SINGLE_QUOTE_IDENT},
		},
		{
			input: `'ho''ge'`,
			token: Token{Value: `ho'ge`, Type: SINGLE_QUOTE_IDENT},
		},
		// DOUBLE_QUOTE_IDENT
		{
			input: `"hoge"`,
			token: Token{Value: `hoge`, Type: DOUBLE_QUOTE_IDENT},
		},
		{
			input: `"ho""ge"`,
			token: Token{Value: `ho"ge`, Type: DOUBLE_QUOTE_IDENT},
		},
		// BACKTICK_IDENT
		{
			input: "`hoge`",
			token: Token{Value: "hoge", Type: BACKTICK_IDENT},
		},
		{
			input: "`ho``ge`",
			token: Token{Value: "ho`ge", Type: BACKTICK_IDENT},
		},
		{
			input: "``",
			token: Token{Value: "", Type: BACKTICK_IDENT},
		},
		{
			input: "````",
			token: Token{Value: "`", Type: BACKTICK_IDENT},
		},
		// ESCAPED STRING BY BACKSLASH
		{
			input: `'ho\'ge'`,
			token: Token{Value: `ho'ge`, Type: SINGLE_QUOTE_IDENT},
		},
		{
			input: `"ho\"ge"`,
			token: Token{Value: `ho"ge`, Type: DOUBLE_QUOTE_IDENT},
		},
		// a backslash escapes any character, including another backslash.
		// escape sequences other than the quote are kept as they are.
		{
			input: `'\\'`,
			token: Token{Value: `\\`, Type: SINGLE_QUOTE_IDENT},
		},
		{
			input: `'a\\', b`,
			token: Token{Value: `a\\`, Type: SINGLE_QUOTE_IDENT},
		},
		{
			input: `'\\\''`,
			token: Token{Value: `\\'`, Type: SINGLE_QUOTE_IDENT},
		},
		{
			input: `'a\nb'`,
			token: Token{Value: `a\nb`, Type: SINGLE_QUOTE_IDENT},
		},
		{
			input: `"\\"`,
			token: Token{Value: `\\`, Type: DOUBLE_QUOTE_IDENT},
		},
		// a backslash is not an escape character in quoted identifiers.
		{
			input: "`a\\`",
			token: Token{Value: `a\`, Type: BACKTICK_IDENT},
		},
		// a NUL character is not the end of the input.
		{
			input: "'a\x00b'",
			token: Token{Value: "a\x00b", Type: SINGLE_QUOTE_IDENT},
		},
		{
			input: "`a\x00b`",
			token: Token{Value: "a\x00b", Type: BACKTICK_IDENT},
		},
		{
			input: "\x00",
			token: Token{Value: "\x00", Type: ILLEGAL},
		},
	}

	for _, spec := range specs {
		t.Run(spec.input, func(t *testing.T) {
			tok := lex([]byte(spec.input))
			spec.token.Line = 1
			spec.token.Col = 1
			if diff := cmp.Diff(&spec.token, tok[0]); diff != "" {
				t.Errorf("tok mismatch: (-want/+got):\n%s", diff)
			}
		})
	}
}
