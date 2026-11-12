package model

import (
	"strconv"
	"strings"
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

// GeneratedColumn describes the definition of a generated column.
// https://dev.mysql.com/doc/refman/8.0/en/create-table-generated-columns.html
type GeneratedColumn struct {
	Valid bool

	// Expr is the expression that computes the column value.
	// It does not include the outermost parentheses,
	// and redundant parentheses around the whole expression are removed by the parser.
	Expr string

	// Stored is true if the column is a STORED generated column.
	// Otherwise, the column is a VIRTUAL generated column.
	Stored bool
}

type Length struct {
	Decimals MaybeString
	Length   string
}

// NewLength creates a new Length which describes the
// length of a column
func NewLength(v string) *Length {
	return &Length{
		Length: v,
	}
}

// TableColumn describes a model object that describes a column
// definition of a table
type TableColumn struct {
	TableID       string
	Name          Ident
	Type          ColumnType
	Length        *Length
	NullState     NullState
	CharacterSet  MaybeIdent
	Collation     MaybeIdent
	Default       DefaultValue
	Comment       MaybeString
	AutoUpdate    MaybeString
	EnumValues    []string
	SetValues     []string
	AutoIncrement bool
	Binary        bool
	Key           bool
	Primary       bool
	Unique        bool
	Unsigned      bool
	ZeroFill      bool
	SRID          MaybeInteger
	Generated     GeneratedColumn

	// Invisible is true if the column is hidden from `SELECT *`.
	// https://dev.mysql.com/doc/refman/26.10/en/invisible-columns.html
	Invisible bool
}

// NewTableColumn creates a new TableColumn with the given name
func NewTableColumn(name string) *TableColumn {
	return &TableColumn{
		Name: Ident(name),
	}
}

func (t *TableColumn) ID() string {
	return "tablecol#" + strings.ToLower(string(t.Name))
}

func (t *TableColumn) NativeLength() *Length {
	// I referred to perl: SQL::Translator::Parser::MySQL#normalize_field https://metacpan.org/source/SQL::Translator::Parser::MySQL#L1072
	unsigned := 0
	if t.Unsigned {
		unsigned++
	}
	var size int
	switch t.Type {
	case ColumnTypeBool, ColumnTypeBoolean:
		// bool and boolean is tinyint(1)
		size = 1
	case ColumnTypeTinyInt:
		size = 4 - unsigned
	case ColumnTypeSmallInt:
		size = 6 - unsigned
	case ColumnTypeMediumInt:
		size = 9 - unsigned
	case ColumnTypeInt, ColumnTypeInteger:
		size = 11 - unsigned
	case ColumnTypeBigInt:
		size = 20
	case ColumnTypeDecimal, ColumnTypeNumeric:
		// DECIMAL(M) means DECIMAL(M,0)
		// The default value of M is 10.
		// https://dev.mysql.com/doc/refman/5.6/en/fixed-point-types.html
		l := &Length{
			Length: "10",
			Decimals: MaybeString{
				Valid: true,
				Value: "0",
			},
		}
		return l
	default:
		return nil
	}

	return NewLength(strconv.Itoa(size))
}

func (t *TableColumn) Normalize() *TableColumn {
	var length *Length
	var synonym ColumnType
	var setDefaultNull bool

	if t.Length == nil {
		if l := t.NativeLength(); l != nil {
			length = l
		}
	}

	if t.Type.SynonymType() != t.Type {
		synonym = t.Type.SynonymType()
	}

	nullState := t.NullState
	// remove null state if not `NOT NULL`
	// If none is specified, the column is treated as if NULL was specified.
	if nullState == NullStateNull {
		nullState = NullStateNone
	}

	defaultValue := t.Default
	if defaultValue.Valid {
		switch t.Type {
		case ColumnTypeBool, ColumnTypeBoolean:
			switch defaultValue.Value {
			case "TRUE":
				defaultValue.Value = "1"
				defaultValue.Quoted = false
			case "FALSE":
				defaultValue.Value = "0"
				defaultValue.Quoted = false
			}
		}

		switch t.Type {
		case ColumnTypeTinyInt, ColumnTypeSmallInt,
			ColumnTypeMediumInt, ColumnTypeInt,
			ColumnTypeInteger, ColumnTypeBigInt,
			ColumnTypeFloat, ColumnTypeDouble,
			ColumnTypeDecimal, ColumnTypeNumeric, ColumnTypeReal,
			ColumnTypeBool, ColumnTypeBoolean:
			// If numeric type then trim quote.
			// BOOL and BOOLEAN are synonyms for TINYINT(1), so they are treated in the same way.
			// Keep the quotes if the value is not a number, e.g. DEFAULT '',
			// because writing it without the quotes makes invalid SQL.
			if defaultValue.Quoted && isNumberLiteral(defaultValue.Value) {
				defaultValue.Value = strings.ToUpper(defaultValue.Value)
				defaultValue.Quoted = false
			}
		}

		// MariaDB shows DEFAULT NULL for nullable TEXT and BLOB columns, but MySQL does not.
		// NULL is the default value of nullable columns even if it is not specified,
		// so remove it to compare them with the columns without DEFAULT NULL.
		// https://github.com/shogo82148/schemalex-deploy/issues/327
		if isTextOrBlob(t.Type) && nullState != NullStateNotNull &&
			!defaultValue.Quoted && defaultValue.Value == "NULL" {
			defaultValue = DefaultValue{}
		}
	} else if !isTextOrBlob(t.Type) {
		// if nullable then set default null.
		// generated columns cannot have a default value.
		if nullState != NullStateNotNull && !t.Generated.Valid {
			setDefaultNull = true
		}
	}

	col := *t
	if length != nil {
		col.Length = length
	}
	if synonym != ColumnTypeInvalid {
		col.Type = synonym
	}

	col.NullState = nullState
	col.Default = defaultValue

	if setDefaultNull {
		col.Default.Valid = true
		col.Default.Value = "NULL"
		col.Default.Quoted = false
	}

	return &col
}

// isTextOrBlob reports whether t is one of the TEXT and BLOB types.
func isTextOrBlob(t ColumnType) bool {
	switch t {
	case ColumnTypeTinyText, ColumnTypeTinyBlob,
		ColumnTypeBlob, ColumnTypeText,
		ColumnTypeMediumBlob, ColumnTypeMediumText,
		ColumnTypeLongBlob, ColumnTypeLongText:
		return true
	}
	return false
}

// isNumberLiteral reports whether s is a number literal that the lexer reads as a single NUMBER token.
// e.g. 1, -1, +1, 1.5, .5, 1e10, 1e-10
func isNumberLiteral(s string) bool {
	isDigit := func(b byte) bool { return '0' <= b && b <= '9' }
	digits := func(i int) int {
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		return i
	}

	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		// a sign must be followed by a digit.
		i++
		if i >= len(s) || !isDigit(s[i]) {
			return false
		}
	}

	start := i
	i = digits(i)
	if i < len(s) && s[i] == '.' {
		i = digits(i + 1)
	}
	if i == start || s[start:i] == "." {
		return false
	}

	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && s[i] == '-' {
			i++
		}
		e := i
		i = digits(i)
		if i == e {
			return false
		}
	}
	return i == len(s)
}
