package model

import "strings"

// MaybeString is a string that may not be set.
type MaybeString struct {
	Valid bool
	Value string
}

// Ident is a string for use as identifiers such as table and column names.
// It should be quoted to avoid confusion with reserved words.
type Ident string

func (ident Ident) Quoted() string {
	var buf strings.Builder
	// Strictly speaking, we need to count the number of backquotes in s.
	// However, in many cases, s doesn't include backquotes.
	buf.Grow(len(ident) + len("``"))

	buf.WriteByte('`')
	for _, r := range ident {
		if r == '`' {
			buf.WriteByte('`')
		}
		buf.WriteRune(r)
	}
	buf.WriteByte('`')
	return buf.String()
}
