//go:generate stringer -type=IndexType -output=index_type_string_gen.go
//go:generate stringer -type=IndexKind -output=index_kind_string_gen.go

package model

// Stmt is the interface to define a statement
type Stmt interface {
	ID() string
}

// Stmts describes a list of statements
type Stmts []Stmt

type MaybeString struct {
	Valid bool
	Value string
}

// ColumnContainer is the interface for objects that can contain
// column names
type ColumnContainer interface {
	AddColumns(...*IndexColumn)
	Columns() chan *IndexColumn
}

type IndexColumnSortDirection int

const (
	SortDirectionNone IndexColumnSortDirection = iota
	SortDirectionAscending
	SortDirectionDescending
)

// Index describes an index on a table.
type Index interface {
	Stmt
	ColumnContainer

	HasType() bool
	HasName() bool
	HasSymbol() bool
	Name() string
	Reference() *Reference
	SetReference(*Reference) Index
	SetSymbol(string) Index
	SetType(IndexType) Index
	SetName(string) Index
	Symbol() string
	IsBtree() bool
	IsHash() bool
	IsPrimaryKey() bool
	IsNormal() bool
	IsUnique() bool
	IsFullText() bool
	IsSpatial() bool
	IsForeignKey() bool
	AddOption(*IndexOption) Index
	Options() chan *IndexOption

	// Normalize returns normalized index. If a normalization was performed
	// and the index is modified, returns a new instance of the Table object
	// along with a true value as the second return value.
	// Otherwise, Normalize() returns the receiver unchanged, with a false
	// as the second return value.
	Normalize() (Index, bool)

	// Clone returns the clone index
	Clone() Index
}

// IndexKind describes the kind (purpose) of an index
type IndexKind int

// List of possible IndexKind.
const (
	IndexKindInvalid IndexKind = iota
	IndexKindPrimaryKey
	IndexKindNormal
	IndexKindUnique
	IndexKindFullText
	IndexKindSpatial
	IndexKindForeignKey
)

// IndexType describes the type (algorithm) used by the index.
type IndexType int

// List of possible index types
const (
	IndexTypeNone IndexType = iota
	IndexTypeBtree
	IndexTypeHash
)

type index struct {
	symbol    MaybeString
	kind      IndexKind
	name      MaybeString
	typ       IndexType
	table     string
	columns   []*IndexColumn
	reference *Reference
	options   []*IndexOption
}
