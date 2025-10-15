package model

import (
	"crypto/sha256"
	"fmt"
)

//go:generate go tool stringer -type=ReferenceMatch -output=reference_match_string_gen.go
//go:generate go tool stringer -type=ReferenceOption -output=reference_option_string_gen.go

// ReferenceMatch describes the matching method of a reference
type ReferenceMatch int

// List of possible ReferenceMatch values
const (
	ReferenceMatchNone ReferenceMatch = iota
	ReferenceMatchFull
	ReferenceMatchPartial
	ReferenceMatchSimple
)

// ReferenceOption describes the actions that could be taken when
// a table/column referred by the reference has been deleted
type ReferenceOption int

// List of possible ReferenceOption values
const (
	ReferenceOptionNone ReferenceOption = iota
	ReferenceOptionRestrict
	ReferenceOptionCascade
	ReferenceOptionSetNull
	ReferenceOptionNoAction
)

// Reference describes a possible reference from one table to another
type Reference struct {
	TableName Ident
	Columns   []*IndexColumn
	Match     ReferenceMatch
	OnDelete  ReferenceOption
	OnUpdate  ReferenceOption
}

// NewReference creates a reference constraint
func NewReference() *Reference {
	return &Reference{}
}

func (r *Reference) ID() string {
	h := sha256.New()
	fmt.Fprintf(h,
		"%s.%s.%s.%s",
		r.TableName,
		r.Match,
		r.OnDelete,
		r.OnUpdate,
	)
	for _, col := range r.Columns {
		fmt.Fprintf(h, "%s", col.ID())
		fmt.Fprintf(h, ".")
	}
	return fmt.Sprintf("reference#%x", h.Sum(nil))
}
