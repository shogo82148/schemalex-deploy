package diff

import (
	"strings"

	"github.com/shogo82148/schemalex-deploy"
	"github.com/shogo82148/schemalex-deploy/model"
)

type myOptions struct {
	parser          *schemalex.Parser
	transaction     bool
	currentSchema   string
	indent          string
	tableNormalizer TableNormalizer
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

// TableNormalizer returns the definition of table normalized by the database server,
// e.g. the result of SHOW CREATE TABLE after creating the table.
// It returns nil if the table cannot be normalized.
//
// MySQL rewrites the expressions of generated columns into its own form,
// e.g. `a + 1` into `(`a` + 1)` and `concat(a, 'x')` into `concat(`a`,_utf8mb4'x')`.
// The diff compares the normalized expressions if the expressions differ as written,
// so that equivalent expressions do not produce CHANGE COLUMN.
//
// The expressions of CHECK constraints are also compared after they are normalized.
// The normalized table must have the CHECK constraints in the same order as table,
// otherwise they are compared as written.
// The current schema, see WithCurrentSchema, has the normalized expressions,
// so they are also used to find the names of the CHECK constraints that do not have their names.
type TableNormalizer func(table *model.Table) (*model.Table, error)

type withTableNormalizer TableNormalizer

func (opt withTableNormalizer) apply(opts *myOptions) {
	opts.tableNormalizer = TableNormalizer(opt)
}

// WithTableNormalizer specifies the function to normalize tables.
// See TableNormalizer for details.
func WithTableNormalizer(f TableNormalizer) Option {
	return withTableNormalizer(f)
}
