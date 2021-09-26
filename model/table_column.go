package model

import (
	"strconv"
	"strings"
)

// NewLength creates a new Length which describes the
// length of a column
func NewLength(v string) Length {
	return &length{
		length: v,
	}
}

func (l *length) Decimal() string {
	return l.decimals.Value
}

func (l *length) HasDecimal() bool {
	return l.decimals.Valid
}

func (l *length) SetDecimal(v string) Length {
	l.decimals.Valid = true
	l.decimals.Value = v
	return l
}

func (l *length) Length() string {
	return l.length
}

// TableColumn describes a model object that describes a column
// definition of a table
type TableColumn struct {
	TableID       string
	Name          string
	Type          ColumnType
	Length        Length
	NullState     NullState
	CharacterSet  maybeString
	Collation     maybeString
	Default       DefaultValue
	Comment       maybeString
	AutoUpdate    maybeString
	EnumValues    []string
	SetValues     []string
	AutoIncrement bool
	Binary        bool
	Key           bool
	Primary       bool
	Unique        bool
	Unsigned      bool
	ZeroFill      bool
}

// NewTableColumn creates a new TableColumn with the given name
func NewTableColumn(name string) *TableColumn {
	return &TableColumn{
		Name: name,
	}
}

func (t *TableColumn) ID() string {
	return "tablecol#" + strings.ToLower(t.Name)
}

func (t *TableColumn) NativeLength() Length {
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
		l := NewLength("10")
		l.SetDecimal("0")
		return l
	default:
		return nil
	}

	return NewLength(strconv.Itoa(size))
}

func (t *TableColumn) Normalize() (*TableColumn, bool) {
	var clone bool
	var length Length
	var synonym ColumnType
	var removeQuotes bool
	var setDefaultNull bool

	if t.Length == nil {
		if l := t.NativeLength(); l != nil {
			clone = true
			length = l
		}
	}

	if t.Type.SynonymType() != t.Type {
		clone = true
		synonym = t.Type.SynonymType()
	}

	nullState := t.NullState
	// remove null state if not `NOT NULL`
	// If none is specified, the column is treated as if NULL was specified.
	if nullState == NullStateNull {
		clone = true
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
				clone = true
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
				clone = true
				setDefaultNull = true
			}
		}
	}

	// avoid cloning if we don't have to
	if !clone {
		return t, false
	}

	col := t.Clone()
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
	return col, true
}

func (t *TableColumn) Clone() *TableColumn {
	col := *t
	return &col
}
