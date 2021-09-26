package format_test

import (
	"bytes"
	"testing"

	"github.com/shogo82148/schemalex-deploy/format"
	"github.com/shogo82148/schemalex-deploy/model"
	"github.com/stretchr/testify/assert"
)

// XXX This test needs serious loving.
func TestFormat(t *testing.T) {
	var dst bytes.Buffer

	table := model.NewTable("hoge")

	col := model.NewTableColumn("fuga")
	col.SetType(model.ColumnTypeInt)
	table.Columns = append(table.Columns, col)

	opt := model.NewTableOption("ENGINE", "InnoDB", false)
	table.Options = append(table.Options, opt)

	index := model.NewIndex(model.IndexKindPrimaryKey, table.ID())
	index.SetName("hoge_pk")
	index.AddColumns(model.NewIndexColumn("fuga"))
	table.Indexes = append(table.Indexes, index)

	if !assert.NoError(t, format.SQL(&dst, table), "format.SQL should succeed") {
		return
	}

	t.Logf("%s", dst.String())
}
