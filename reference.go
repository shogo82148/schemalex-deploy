package schemalex

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
