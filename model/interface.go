//go:generate stringer -type=IndexType -output=index_type_string_gen.go
//go:generate stringer -type=IndexKind -output=index_kind_string_gen.go
//go:generate stringer -type=ReferenceMatch -output=reference_match_string_gen.go
//go:generate stringer -type=ReferenceOption -output=reference_option_string_gen.go

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
	AddColumns(...IndexColumn)
	Columns() chan IndexColumn
}

type IndexColumnSortDirection int

const (
	SortDirectionNone IndexColumnSortDirection = iota
	SortDirectionAscending
	SortDirectionDescending
)

// IndexColumn is a column name/length specification used in indexes
type IndexColumn interface {
	ID() string
	Name() string
	SetLength(string) IndexColumn
	HasLength() bool
	Length() string
	SetSortDirection(IndexColumnSortDirection)
	HasSortDirection() bool
	IsAscending() bool
	IsDescending() bool
}

// Index describes an index on a table.
type Index interface {
	Stmt
	ColumnContainer

	HasType() bool
	HasName() bool
	HasSymbol() bool
	Name() string
	Reference() Reference
	SetReference(Reference) Index
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
	AddOption(IndexOption) Index
	Options() chan IndexOption

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

// and index column specification may be
// name or name(length)
type indexColumn struct {
	name          string
	length        MaybeString
	sortDirection IndexColumnSortDirection
}

// IndexOption describes a possible index option, such as `WITH PARSER ngram`
type IndexOption interface {
	Stmt
	Key() string
	Value() string
	NeedQuotes() bool
}

type index struct {
	symbol    MaybeString
	kind      IndexKind
	name      MaybeString
	typ       IndexType
	table     string
	columns   []IndexColumn
	reference Reference
	options   []IndexOption
}

type indexopt struct {
	key        string
	value      string
	needQuotes bool
}

// Reference describes a possible reference from one table to another
type Reference interface {
	ColumnContainer

	ID() string
	String() string
	TableName() string
	OnDelete() ReferenceOption
	OnUpdate() ReferenceOption
	SetTableName(string) Reference
	SetMatch(ReferenceMatch) Reference
	SetOnDelete(ReferenceOption) Reference
	SetOnUpdate(ReferenceOption) Reference
	MatchFull() bool
	MatchPartial() bool
	MatchSimple() bool
}

type reference struct {
	tableName string
	columns   []IndexColumn
	match     ReferenceMatch
	onDelete  ReferenceOption
	onUpdate  ReferenceOption
}

// ReferenceMatch describes the mathing method of a reference
type ReferenceMatch int

// List of possible ReferenceMatch values
const (
	ReferenceMatchNone ReferenceMatch = iota
	ReferenceMatchFull
	ReferenceMatchPartial
	ReferenceMatchSimple
)

// ReferenceOption describes the actions that could be taken when
// a table/column referered by the reference has been deleted
type ReferenceOption int

// List of possible ReferenceOption values
const (
	ReferenceOptionNone ReferenceOption = iota
	ReferenceOptionRestrict
	ReferenceOptionCascade
	ReferenceOptionSetNull
	ReferenceOptionNoAction
)

// NullState describes the possible NULL constraint of a column
type NullState int

// List of possible NullStates. NullStateNone specifies that there is
// no NULL constraint. NullStateNull explicitly specifies that the column
// may be NULL. NullStateNotNull specifies that the column may not be NULL
const (
	NullStateNone NullState = iota
	NullStateNull
	NullStateNotNull
)

type DefaultValue struct {
	Valid  bool
	Value  string
	Quoted bool
}
