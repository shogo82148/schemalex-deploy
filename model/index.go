//go:generate go tool stringer -type=IndexType -output=index_type_string_gen.go
//go:generate go tool stringer -type=IndexKind -output=index_kind_string_gen.go

package model

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

type IndexColumnSortDirection int

const (
	SortDirectionNone IndexColumnSortDirection = iota
	SortDirectionAscending
	SortDirectionDescending
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

// IndexType describes the type (algorithm) used by the index.
type IndexType int

// List of possible index types
const (
	IndexTypeNone IndexType = iota
	IndexTypeBtree
	IndexTypeHash
)

// Index describes an index on a table.
type Index struct {
	Table          string
	Type           IndexType
	ConstraintName MaybeIdent
	Kind           IndexKind
	Name           MaybeIdent
	Columns        []*IndexColumn
	Reference      *Reference
	Options        []*IndexOption
}

// NewIndex creates a new index with the given index kind.
func NewIndex(kind IndexKind, table string) *Index {
	return &Index{
		Kind:  kind,
		Table: table,
	}
}

func (idx *Index) ID() string {
	// This is tricky. and index may or may not have a name. It would
	// have been so much easier if we did, but we don't, so we'll fake
	// something.
	//
	// In case we don't have a name, we need to know the table, the kind,
	// the type, the column(s), and the reference(s).
	name := "index"
	if idx.Name.Valid {
		name = name + "#" + string(idx.Name.Ident)
	}
	h := sha256.New()

	sym := "none"
	if idx.ConstraintName.Valid {
		sym = string(idx.ConstraintName.Ident)
	}

	fmt.Fprintf(h,
		"%s.%s.%s.%s",
		idx.Table,
		sym,
		idx.Kind,
		idx.Type,
	)
	for _, col := range idx.Columns {
		fmt.Fprintf(h, ".")
		fmt.Fprintf(h, "%s", col.ID())
	}
	if idx.Reference != nil {
		fmt.Fprintf(h, ".")
		fmt.Fprintf(h, "%s", idx.Reference.ID())
	}
	return fmt.Sprintf("%s#%x", name, h.Sum(nil))
}

func (stmt *Index) Normalize() *Index {
	newindex := *stmt
	return &newindex
}

// IndexColumn is a column name/length specification used in indexes
type IndexColumn struct {
	Name          Ident
	Length        MaybeString
	SortDirection IndexColumnSortDirection
}

func NewIndexColumn(name Ident) *IndexColumn {
	return &IndexColumn{
		Name: name,
	}
}

func (col *IndexColumn) ID() string {
	name := strings.ToLower(string(col.Name))
	if col.Length.Valid {
		return "index_column#" + name + "-" + col.Length.Value
	}
	return "index_column#" + name
}

// IndexOption describes a possible index option, such as `WITH PARSER ngram`
type IndexOption struct {
	Key        string
	Value      string
	NeedQuotes bool
}

func NewIndexOption(k, v string, q bool) *IndexOption {
	return &IndexOption{
		Key:        k,
		Value:      v,
		NeedQuotes: q,
	}
}

func (opt *IndexOption) ID() string { return "indexopt#" + strings.ToLower(opt.Key) }
