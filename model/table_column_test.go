package model_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/shogo82148/schemalex-deploy/format"
	"github.com/shogo82148/schemalex-deploy/model"
	"github.com/stretchr/testify/assert"
)

func TestTableColumnNormalize(t *testing.T) {
	type testCase struct {
		before, after *model.TableColumn
	}

	for _, tc := range []testCase{
		{
			// foo VARCHAR (255) NOT NULL
			before: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeVarChar,
				Length:    model.NewLength("255"),
				NullState: model.NullStateNotNull,
			},
			// foo VARCHAR (255) NOT NULL
			after: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeVarChar,
				Length:    model.NewLength("255"),
				NullState: model.NullStateNotNull,
			},
		},
		{
			// foo VARCHAR NULL
			before: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeVarChar,
				NullState: model.NullStateNull,
			},
			// foo VARCHAR DEFAULT NULL
			after: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeVarChar,
				NullState: model.NullStateNone,
				Default: model.DefaultValue{
					Valid:  true,
					Value:  "NULL",
					Quoted: false,
				},
			},
		},
		{
			// foo INTEGER NOT NULL,
			before: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeInteger,
				NullState: model.NullStateNotNull,
			},
			// foo INT (11) NOT NULL,
			after: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeInt,
				Length:    model.NewLength("11"),
				NullState: model.NullStateNotNull,
			},
		},
		{
			// foo INTEGER UNSIGNED NULL DEFAULT 0,
			before: &model.TableColumn{
				Name:      "foo",
				Unsigned:  true,
				Type:      model.ColumnTypeInteger,
				NullState: model.NullStateNull,
				Default: model.DefaultValue{
					Valid:  true,
					Value:  "0",
					Quoted: false,
				},
			},
			// foo INT (10) UNSIGNED DEFAULT 0,
			after: &model.TableColumn{
				Name:      "foo",
				Unsigned:  true,
				Type:      model.ColumnTypeInt,
				Length:    model.NewLength("10"),
				NullState: model.NullStateNone,
				Default: model.DefaultValue{
					Valid:  true,
					Value:  "0",
					Quoted: false,
				},
			},
		},
		{
			// foo bigint null default null,
			before: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeBigInt,
				NullState: model.NullStateNull,
				Default: model.DefaultValue{
					Valid:  true,
					Value:  "NULL",
					Quoted: false,
				},
			},
			// foo BIGINT (20) DEFAULT NULL,
			after: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeBigInt,
				Length:    model.NewLength("20"),
				NullState: model.NullStateNone,
				Default: model.DefaultValue{
					Valid:  true,
					Value:  "NULL",
					Quoted: false,
				},
			},
		},
		{
			// foo NUMERIC,
			before: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeNumeric,
				NullState: model.NullStateNone,
			},
			// foo DECIMAL (10,0) DEFAULT NULL,
			after: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeDecimal,
				Length:    model.NewLength("10").SetDecimal("0"),
				NullState: model.NullStateNone,
				Default: model.DefaultValue{
					Valid:  true,
					Value:  "NULL",
					Quoted: false,
				},
			},
		},
		{
			// foo TEXT,
			before: &model.TableColumn{
				Name: "foo",
				Type: model.ColumnTypeText,
			},
			// foo TEXT,
			after: &model.TableColumn{
				Name: "foo",
				Type: model.ColumnTypeText,
			},
		},
		{
			// foo BOOL,
			before: &model.TableColumn{
				Name: "foo",
				Type: model.ColumnTypeBool,
			},
			// foo TINYINT(1) DEFAULT NULL,
			after: &model.TableColumn{
				Name:      "foo",
				Type:      model.ColumnTypeTinyInt,
				Length:    model.NewLength("1"),
				NullState: model.NullStateNone,
				Default: model.DefaultValue{
					Valid:  true,
					Value:  "NULL",
					Quoted: false,
				},
			},
		},
	} {
		var buf bytes.Buffer
		format.SQL(&buf, tc.before)
		beforeStr := buf.String()
		buf.Reset()
		format.SQL(&buf, tc.after)
		afterStr := buf.String()
		t.Run(fmt.Sprintf("from %s to %s", beforeStr, afterStr), func(t *testing.T) {
			norm, _ := tc.before.Normalize()
			if !assert.Equal(t, norm, tc.after, "Unexpected return value.") {
				buf.Reset()
				format.SQL(&buf, norm)
				normStr := buf.String()
				t.Logf("before: %s normalized: %s", beforeStr, normStr)
				t.Logf("after: %s", afterStr)
			}
		})
	}
}
