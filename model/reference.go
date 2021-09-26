package model

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/shogo82148/schemalex-deploy/internal/util"
)

//go:generate stringer -type=ReferenceMatch -output=reference_match_string_gen.go
//go:generate stringer -type=ReferenceOption -output=reference_option_string_gen.go

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

// Reference describes a possible reference from one table to another
type Reference struct {
	TableName string
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

func (r *Reference) String() string {
	var buf strings.Builder

	buf.WriteString("REFERENCES ")
	buf.WriteString(util.Backquote(r.TableName))
	buf.WriteString(" (")

	ch := r.Columns
	lch := len(ch)
	for i, col := range ch {
		buf.WriteString(col.Name.Quoted())
		if i < lch-1 {
			buf.WriteString(", ")
		}
		i++
	}
	buf.WriteByte(')')

	switch r.Match {
	case ReferenceMatchFull:
		buf.WriteString(" MATCH FULL")
	case ReferenceMatchPartial:
		buf.WriteString(" MATCH PARTIAL")
	case ReferenceMatchSimple:
		buf.WriteString(" MATCH SIMPLE")
	}

	// we don't need to check the errors, because strings.Builder doesn't return any error.
	writeReferenceOption(&buf, "ON DELETE", r.OnDelete)
	writeReferenceOption(&buf, "ON UPDATE", r.OnUpdate)

	return buf.String()
}

func writeReferenceOption(buf *strings.Builder, prefix string, opt ReferenceOption) {
	if opt == ReferenceOptionNone {
		return
	}
	buf.WriteByte(' ')
	buf.WriteString(prefix)
	switch opt {
	case ReferenceOptionRestrict:
		buf.WriteString(" RESTRICT")
	case ReferenceOptionCascade:
		buf.WriteString(" CASCADE")
	case ReferenceOptionSetNull:
		buf.WriteString(" SET NULL")
	case ReferenceOptionNoAction:
		buf.WriteString(" NO ACTION")
	default:
		panic(fmt.Errorf("unknown reference option: %d", int(opt)))
	}
}
