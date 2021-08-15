package util

import "strings"

// Backquote surrounds the given string in backquotes
func Backquote(s string) string {
	var buf strings.Builder
	// Strictly speaking, we need to count the number of backquotes in s.
	// However, in many cases, s doesn't include backquotes.
	buf.Grow(len(s) + len("``"))

	buf.WriteByte('`')
	for _, r := range s {
		if r == '`' {
			buf.WriteByte('`')
		}
		buf.WriteRune(r)
	}
	buf.WriteByte('`')
	return buf.String()
}
