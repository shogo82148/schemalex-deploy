package diff

import (
	"strings"

	"github.com/shogo82148/schemalex-deploy"
)

type myOptions struct {
	parser        *schemalex.Parser
	transaction   bool
	currentSchema string
	indent        string
}

type Option interface {
	apply(opts *myOptions)
}

type withParser struct {
	p *schemalex.Parser
}

func (opt withParser) apply(opts *myOptions) {
	opts.parser = opt.p
}

// WithParser specifies the parser instance to use when parsing
// the statements given to the diffing functions. If unspecified,
// a default parser will be used
func WithParser(p *schemalex.Parser) Option {
	return &withParser{p}
}

type withTransaction bool

func (opt withTransaction) apply(opts *myOptions) {
	opts.transaction = bool(opt)
}

// WithTransaction specifies if statements to control transactions
// should be included in the diff.
func WithTransaction(b bool) Option {
	return withTransaction(b)
}

type withCurrentSchema string

func (opt withCurrentSchema) apply(opts *myOptions) {
	opts.currentSchema = string(opt)
}

// WithCurrentSchema specifies the current schema deployed in MySQL.
func WithCurrentSchema(schema string) Option {
	return withCurrentSchema(schema)
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
