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
	var removeQuotes bool
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

	if t.Default.Valid {
		switch t.Type {
		case ColumnTypeTinyInt, ColumnTypeSmallInt,
			ColumnTypeMediumInt, ColumnTypeInt,
			ColumnTypeInteger, ColumnTypeBigInt,
			ColumnTypeFloat, ColumnTypeDouble,
			ColumnTypeDecimal, ColumnTypeNumeric, ColumnTypeReal:
			// If numeric type then trim quote
			if t.Default.Quoted {
				removeQuotes = true
			}
		case ColumnTypeBool, ColumnTypeBoolean:
			switch t.Default.Value {
			case "TRUE":
				t.Default.Valid = true
				t.Default.Value = "1"
				t.Default.Quoted = false
			case "FALSE":
				t.Default.Valid = true
				t.Default.Value = "0"
				t.Default.Quoted = false
			}
		}
	} else {
		switch t.Type {
		case ColumnTypeTinyText, ColumnTypeTinyBlob,
			ColumnTypeBlob, ColumnTypeText,
			ColumnTypeMediumBlob, ColumnTypeMediumText,
			ColumnTypeLongBlob, ColumnTypeLongText:
		default:
			// if nullable then set default null.
			if nullState != NullStateNotNull {
				setDefaultNull = true
			}
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

	if removeQuotes {
		col.Default.Valid = true
		col.Default.Value = t.Default.Value
		col.Default.Quoted = false
	}

	if setDefaultNull {
		col.Default.Valid = true
		col.Default.Value = "NULL"
		col.Default.Quoted = false
	}
	return &col
}
