package diff

import (
	"github.com/shogo82148/schemalex-deploy"
)

type myOptions struct {
	parser        *schemalex.Parser
	transaction   bool
	currentSchema string
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
