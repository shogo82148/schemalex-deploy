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
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("from %q to %q", tc.beforeStr, tc.afterStr), func(t *testing.T) {
			norm, _ := tc.before.Normalize()
			if diff := cmp.Diff(tc.after, norm); diff != "" {
				t.Errorf("mismatch (-want/+got)\n%s", diff)
			}
		})
	}
}
