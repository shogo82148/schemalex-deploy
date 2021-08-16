package schemalex

//go:generate stringer -type=IndexType -output=index_type_string_gen.go
//go:generate stringer -type=IndexKind -output=index_kind_string_gen.go

// IndexType describes the type (algorithm) used by the index.
type IndexType int

// List of possible index types
const (
	IndexTypeNone IndexType = iota
	IndexTypeBtree
	IndexTypeHash
)

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
