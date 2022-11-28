package format

import "strings"

type myOptions struct {
	indent string
}

type Option interface {
	apply(opts *myOptions)
}

type withIndent string

func (opt withIndent) apply(opts *myOptions) {
	opts.indent = string(opt)
}

// WithIndent specifies the indent string to use, and the length.
// For example, if you specify WithIndent(" " /* single space */, 2), the
// indent will be 2 spaces per level.
//
// Please note that no check on the string will be performed, so anything
// you specify will be used as-is.
func WithIndent(s string, n int) Option {
	if n <= 0 {
		n = 1
	}
	return withIndent(strings.Repeat(s, n))
}
