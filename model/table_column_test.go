package model

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTableColumnNormalize(t *testing.T) {
	type testCase struct {
		beforeStr, afterStr string
		before, after       *TableColumn
	}

	testCases := []testCase{
		{
			beforeStr: "foo VARCHAR (255) NOT NULL",
			before: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeVarChar,
				Length:    NewLength("255"),
				NullState: NullStateNotNull,
			},
			afterStr: "foo VARCHAR (255) NOT NULL",
			after: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeVarChar,
				Length:    NewLength("255"),
				NullState: NullStateNotNull,
			},
		},
		{
			beforeStr: "foo VARCHAR NULL",
			before: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeVarChar,
				NullState: NullStateNull,
			},
			afterStr: "foo VARCHAR DEFAULT NULL",
			after: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeVarChar,
				NullState: NullStateNone,
				Default: DefaultValue{
					Valid:  true,
					Value:  "NULL",
					Quoted: false,
				},
			},
		},
		{
			beforeStr: "foo INTEGER NOT NULL",
			before: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeInteger,
				NullState: NullStateNotNull,
			},
			afterStr: "foo INT (11) NOT NULL",
			after: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeInt,
				Length:    NewLength("11"),
				NullState: NullStateNotNull,
			},
		},
		{
			beforeStr: "foo INTEGER UNSIGNED NULL DEFAULT 0",
			before: &TableColumn{
				Name:      "foo",
				Unsigned:  true,
				Type:      ColumnTypeInteger,
				NullState: NullStateNull,
				Default: DefaultValue{
					Valid:  true,
					Value:  "0",
					Quoted: false,
				},
			},
			afterStr: "foo INT (10) UNSIGNED DEFAULT 0",
			after: &TableColumn{
				Name:      "foo",
				Unsigned:  true,
				Type:      ColumnTypeInt,
				Length:    NewLength("10"),
				NullState: NullStateNone,
				Default: DefaultValue{
					Valid:  true,
					Value:  "0",
					Quoted: false,
				},
			},
		},
		{
			beforeStr: "foo bigint null default null",
			before: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeBigInt,
				NullState: NullStateNull,
				Default: DefaultValue{
					Valid:  true,
					Value:  "NULL",
					Quoted: false,
				},
			},
			afterStr: "foo BIGINT (20) DEFAULT NULL",
			after: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeBigInt,
				Length:    NewLength("20"),
				NullState: NullStateNone,
				Default: DefaultValue{
					Valid:  true,
					Value:  "NULL",
					Quoted: false,
				},
			},
		},
		{
			beforeStr: "foo NUMERIC",
			before: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeNumeric,
				NullState: NullStateNone,
			},
			afterStr: "foo DECIMAL (10,0) DEFAULT NULL",
			after: &TableColumn{
				Name: "foo",
				Type: ColumnTypeDecimal,
				Length: &Length{
					Length: "10",
					Decimals: MaybeString{
						Valid: true,
						Value: "0",
					},
				},
				NullState: NullStateNone,
				Default: DefaultValue{
					Valid:  true,
					Value:  "NULL",
					Quoted: false,
				},
			},
		},
		{
			beforeStr: "foo TEXT",
			before: &TableColumn{
				Name: "foo",
				Type: ColumnTypeText,
			},
			afterStr: "foo TEXT",
			after: &TableColumn{
				Name: "foo",
				Type: ColumnTypeText,
			},
		},
		{
			beforeStr: "foo BOOL",
			before: &TableColumn{
				Name: "foo",
				Type: ColumnTypeBool,
			},
			afterStr: "foo TINYINT(1) DEFAULT NULL",
			after: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeTinyInt,
				Length:    NewLength("1"),
				NullState: NullStateNone,
				Default: DefaultValue{
					Valid:  true,
					Value:  "NULL",
					Quoted: false,
				},
			},
		},
		{
			beforeStr: "intud int unsigned default 0",
			before: &TableColumn{
				Name:      "intud",
				Unsigned:  true,
				Type:      ColumnTypeInt,
				NullState: NullStateNone,
				Default: DefaultValue{
					Valid:  true,
					Value:  "0",
					Quoted: true,
				},
			},
			afterStr: "intud INT (10) UNSIGNED DEFAULT '0'",
			after: &TableColumn{
				Name:      "intud",
				Unsigned:  true,
				Type:      ColumnTypeInt,
				Length:    NewLength("10"),
				NullState: NullStateNone,
				Default: DefaultValue{
					Valid:  true,
					Value:  "0",
					Quoted: false,
				},
			},
		},
		{
			// generated columns never have DEFAULT NULL.
			beforeStr: "foo INTEGER GENERATED ALWAYS AS (bar + 1) VIRTUAL",
			before: &TableColumn{
				Name: "foo",
				Type: ColumnTypeInteger,
				Generated: GeneratedColumn{
					Valid: true,
					Expr:  "bar + 1",
				},
			},
			afterStr: "foo INT (11) GENERATED ALWAYS AS (bar + 1) VIRTUAL",
			after: &TableColumn{
				Name:   "foo",
				Type:   ColumnTypeInt,
				Length: NewLength("11"),
				Generated: GeneratedColumn{
					Valid: true,
					Expr:  "bar + 1",
				},
			},
		},
		{
			beforeStr: "foo INT DEFAULT '1e5'",
			before: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeInt,
				Default: DefaultValue{Valid: true, Value: "1e5", Quoted: true},
			},
			afterStr: "foo INT (11) DEFAULT 1E5",
			after: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeInt,
				Length:  NewLength("11"),
				Default: DefaultValue{Valid: true, Value: "1E5", Quoted: false},
			},
		},
		{
			// the quotes are kept, because DEFAULT without a value is invalid.
			beforeStr: "foo INT DEFAULT ''",
			before: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeInt,
				Default: DefaultValue{Valid: true, Value: "", Quoted: true},
			},
			afterStr: "foo INT (11) DEFAULT ''",
			after: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeInt,
				Length:  NewLength("11"),
				Default: DefaultValue{Valid: true, Value: "", Quoted: true},
			},
		},
		{
			// BOOL is TINYINT(1), so the quotes are removed in the same way.
			beforeStr: "foo BOOL DEFAULT '1'",
			before: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeBool,
				Default: DefaultValue{Valid: true, Value: "1", Quoted: true},
			},
			afterStr: "foo TINYINT (1) DEFAULT 1",
			after: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeTinyInt,
				Length:  NewLength("1"),
				Default: DefaultValue{Valid: true, Value: "1", Quoted: false},
			},
		},
		{
			beforeStr: "foo BOOL DEFAULT 'a'",
			before: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeBool,
				Default: DefaultValue{Valid: true, Value: "a", Quoted: true},
			},
			afterStr: "foo TINYINT (1) DEFAULT 'a'",
			after: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeTinyInt,
				Length:  NewLength("1"),
				Default: DefaultValue{Valid: true, Value: "a", Quoted: true},
			},
		},

		// https://github.com/shogo82148/schemalex-deploy/issues/327
		{
			// MariaDB shows DEFAULT NULL for nullable TEXT columns.
			beforeStr: "foo TEXT DEFAULT NULL",
			before: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeText,
				Default: DefaultValue{Valid: true, Value: "NULL"},
			},
			afterStr: "foo TEXT",
			after: &TableColumn{
				Name: "foo",
				Type: ColumnTypeText,
			},
		},
		{
			beforeStr: "foo LONGBLOB NULL DEFAULT NULL",
			before: &TableColumn{
				Name:      "foo",
				Type:      ColumnTypeLongBlob,
				NullState: NullStateNull,
				Default:   DefaultValue{Valid: true, Value: "NULL"},
			},
			afterStr: "foo LONGBLOB",
			after: &TableColumn{
				Name: "foo",
				Type: ColumnTypeLongBlob,
			},
		},
		{
			// the string 'NULL' is not NULL.
			beforeStr: "foo TEXT DEFAULT 'NULL'",
			before: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeText,
				Default: DefaultValue{Valid: true, Value: "NULL", Quoted: true},
			},
			afterStr: "foo TEXT DEFAULT 'NULL'",
			after: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeText,
				Default: DefaultValue{Valid: true, Value: "NULL", Quoted: true},
			},
		},
		{
			// DEFAULT NULL is not removed from other types.
			beforeStr: "foo VARCHAR (10) DEFAULT NULL",
			before: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeVarChar,
				Length:  NewLength("10"),
				Default: DefaultValue{Valid: true, Value: "NULL"},
			},
			afterStr: "foo VARCHAR (10) DEFAULT NULL",
			after: &TableColumn{
				Name:    "foo",
				Type:    ColumnTypeVarChar,
				Length:  NewLength("10"),
				Default: DefaultValue{Valid: true, Value: "NULL"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("from %q to %q", tc.beforeStr, tc.afterStr), func(t *testing.T) {
			norm := tc.before.Normalize()
			if diff := cmp.Diff(tc.after, norm); diff != "" {
				t.Errorf("mismatch (-want/+got)\n%s", diff)
			}
		})
	}
}

func TestIsNumberLiteral(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"0", true},
		{"123", true},
		{"-1", true},
		{"+1", true},
		{"1.5", true},
		{"1.", true},
		{".5", true},
		{"1e5", true},
		{"1E-5", true},
		{"-1.5e10", true},
		{"", false},
		{".", false},
		{"-", false},
		{"+", false},
		{"-.5", false},
		{"+.5", false},
		{"1e", false},
		{"1e+5", false},
		{"1.2.3", false},
		{"abc", false},
		{"1a", false},
		{" 1", false},
		{"TRUE", false},
	}
	for _, tt := range tests {
		if got := isNumberLiteral(tt.input); got != tt.want {
			t.Errorf("isNumberLiteral(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
