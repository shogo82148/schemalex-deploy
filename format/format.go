package format

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/shogo82148/schemalex-deploy/internal/util"
	"github.com/shogo82148/schemalex-deploy/model"
)

type fmtCtx struct {
	curIndent string
	dst       io.Writer
	indent    string
}

func newFmtCtx(dst io.Writer) *fmtCtx {
	return &fmtCtx{
		dst: dst,
	}
}

func (ctx *fmtCtx) clone() *fmtCtx {
	return &fmtCtx{
		curIndent: ctx.curIndent,
		dst:       ctx.dst,
		indent:    ctx.indent,
	}
}

// SQL takes an arbitrary `model.*` object and formats it as SQL,
// writing its result to `dst`
func SQL(dst io.Writer, v interface{}, options ...Option) error {
	ctx := newFmtCtx(dst)
	for _, o := range options {
		switch o.Name() {
		case optkeyIndent:
			ctx.indent = o.Value().(string)
		}
	}

	return format(ctx, v)
}

func format(ctx *fmtCtx, v interface{}) error {
	switch v := v.(type) {
	case model.ColumnType:
		return formatColumnType(ctx, v)
	case *model.Database:
		return formatDatabase(ctx, v)
	case model.Stmts:
		for _, s := range v {
			if err := format(ctx, s); err != nil {
				return err
			}
		}
		return nil
	case *model.Table:
		return formatTable(ctx, v)
	case *model.TableColumn:
		return formatTableColumn(ctx, v)
	case *model.TableOption:
		return formatTableOption(ctx, v)
	case model.Index:
		return formatIndex(ctx, v)
	case *model.Reference:
		return formatReference(ctx, v)
	default:
		return fmt.Errorf("unsupported model type: %T", v)
	}
}

func formatDatabase(ctx *fmtCtx, d *model.Database) error {
	var buf bytes.Buffer
	buf.WriteString("CREATE DATABASE")
	if d.IfNotExists {
		buf.WriteString(" IF NOT EXISTS")
	}
	buf.WriteByte(' ')
	buf.WriteString(util.Backquote(d.Name))
	buf.WriteByte(';')

	if _, err := buf.WriteTo(ctx.dst); err != nil {
		return err
	}
	return nil
}

func formatTableOption(ctx *fmtCtx, option *model.TableOption) error {
	var buf bytes.Buffer
	buf.WriteString(option.Key)
	buf.WriteString(" = ")
	if option.NeedQuotes {
		buf.WriteByte('\'')
		buf.WriteString(option.Value)
		buf.WriteByte('\'')
	} else {
		buf.WriteString(option.Value)
	}

	if _, err := buf.WriteTo(ctx.dst); err != nil {
		return err
	}
	return nil
}

func formatTable(ctx *fmtCtx, table *model.Table) error {
	var buf bytes.Buffer

	buf.WriteString("CREATE")
	if table.Temporary {
		buf.WriteString(" TEMPORARY")
	}

	buf.WriteString(" TABLE")
	if table.IfNotExists {
		buf.WriteString(" IF NOT EXISTS")
	}

	buf.WriteByte(' ')
	buf.WriteString(util.Backquote(table.Name))

	if table.LikeTable.Valid {
		buf.WriteString(" LIKE ")
		buf.WriteString(util.Backquote(table.LikeTable.Value))
	} else {

		newctx := ctx.clone()
		newctx.curIndent = newctx.indent + newctx.curIndent
		newctx.dst = &buf

		buf.WriteString(" (")

		for i, col := range table.Columns {
			buf.WriteByte('\n')
			if err := formatTableColumn(newctx, col); err != nil {
				return err
			}
			if i < len(table.Columns)-1 || len(table.Indexes) > 0 {
				buf.WriteByte(',')
			}
			i++
		}

		for i, idx := range table.Indexes {
			buf.WriteByte('\n')
			if err := formatIndex(newctx, idx); err != nil {
				return err
			}
			if i < len(table.Indexes)-1 {
				buf.WriteByte(',')
			}
			i++
		}

		buf.WriteString("\n)")

		if l := len(table.Options); l > 0 {
			buf.WriteByte(' ')
			for i, option := range table.Options {
				if err := formatTableOption(newctx, option); err != nil {
					return err
				}

				if i < l-1 {
					buf.WriteString(", ")
				}
				i++
			}
		}
	}

	if _, err := buf.WriteTo(ctx.dst); err != nil {
		return err
	}
	return nil
}

func formatColumnType(ctx *fmtCtx, col model.ColumnType) error {
	if col <= model.ColumnTypeInvalid || col >= model.ColumnTypeMax {
		return fmt.Errorf("known column type: %d", int(col))
	}

	if _, err := io.WriteString(ctx.dst, col.String()); err != nil {
		return err
	}

	return nil
}

func formatTableColumn(ctx *fmtCtx, col *model.TableColumn) error {
	var buf bytes.Buffer

	buf.WriteString(ctx.curIndent)
	buf.WriteString(util.Backquote(col.Name))
	buf.WriteByte(' ')

	newctx := ctx.clone()
	newctx.curIndent = ""
	newctx.dst = &buf
	if err := formatColumnType(newctx, col.Type); err != nil {
		return err
	}

	switch col.Type {
	case model.ColumnTypeEnum:
		buf.WriteString(" (")
		for _, enumValue := range col.EnumValues {
			buf.WriteByte('\'')
			buf.WriteString(enumValue)
			buf.WriteByte('\'')
			buf.WriteByte(',')
		}
		buf.Truncate(buf.Len() - 1)
		buf.WriteByte(')')
	case model.ColumnTypeSet:
		buf.WriteString(" (")
		for _, setValue := range col.SetValues {
			buf.WriteByte('\'')
			buf.WriteString(setValue)
			buf.WriteByte('\'')
			buf.WriteByte(',')
		}
		buf.Truncate(buf.Len() - 1)
		buf.WriteByte(')')
	default:
		if col.Length != nil {
			l := col.Length
			buf.WriteString(" (")
			buf.WriteString(l.Length)
			if l.Decimals.Valid {
				buf.WriteByte(',')
				buf.WriteString(l.Decimals.Value)
			}
			buf.WriteByte(')')
		}
	}

	if col.Unsigned {
		buf.WriteString(" UNSIGNED")
	}

	if col.ZeroFill {
		buf.WriteString(" ZEROFILL")
	}

	if col.Binary {
		buf.WriteString(" BINARY")
	}

	if col.CharacterSet.Valid {
		buf.WriteString(" CHARACTER SET ")
		buf.WriteString(util.Backquote(col.CharacterSet.Value))
	}

	if col.Collation.Valid {
		buf.WriteString(" COLLATE ")
		buf.WriteString(util.Backquote(col.Collation.Value))
	}

	if col.AutoUpdate.Valid {
		buf.WriteString(" ON UPDATE ")
		buf.WriteString(col.AutoUpdate.Value)
	}

	if n := col.NullState; n != model.NullStateNone {
		buf.WriteByte(' ')
		switch n {
		case model.NullStateNull:
			buf.WriteString("NULL")
		case model.NullStateNotNull:
			buf.WriteString("NOT NULL")
		}
	}

	if col.Default.Valid {
		buf.WriteString(" DEFAULT ")
		if col.Default.Quoted {
			buf.WriteByte('\'')
			buf.WriteString(col.Default.Value)
			buf.WriteByte('\'')
		} else {
			buf.WriteString(col.Default.Value)
		}
	}

	if col.AutoIncrement {
		buf.WriteString(" AUTO_INCREMENT")
	}

	if col.Unique {
		buf.WriteString(" UNIQUE KEY")
	}

	if col.Primary {
		buf.WriteString(" PRIMARY KEY")
	} else if col.Key {
		buf.WriteString(" KEY")
	}

	if col.Comment.Valid {
		buf.WriteString(" COMMENT '")
		buf.WriteString(col.Comment.Value)
		buf.WriteByte('\'')
	}

	if _, err := buf.WriteTo(ctx.dst); err != nil {
		return err
	}
	return nil
}

func formatIndex(ctx *fmtCtx, index model.Index) error {
	var buf bytes.Buffer

	buf.WriteString(ctx.curIndent)
	if index.HasSymbol() {
		buf.WriteString("CONSTRAINT ")
		buf.WriteString(util.Backquote(index.Symbol()))
		buf.WriteByte(' ')
	}

	switch {
	case index.IsPrimaryKey():
		buf.WriteString("PRIMARY KEY")
	case index.IsNormal():
		buf.WriteString("INDEX")
	case index.IsUnique():
		buf.WriteString("UNIQUE INDEX")
	case index.IsFullText():
		buf.WriteString("FULLTEXT INDEX")
	case index.IsSpatial():
		buf.WriteString("SPATIAL INDEX")
	case index.IsForeignKey():
		buf.WriteString("FOREIGN KEY")
	}

	if index.HasName() {
		buf.WriteByte(' ')
		buf.WriteString(util.Backquote(index.Name()))
	}

	switch {
	case index.IsBtree():
		buf.WriteString(" USING BTREE")
	case index.IsHash():
		buf.WriteString(" USING HASH")
	}

	buf.WriteString(" (")
	ch := index.Columns()
	lch := len(ch)
	if lch == 0 {
		return errors.New(`no columns in index`)
	}

	var i int
	for col := range ch {
		buf.WriteString(util.Backquote(col.Name()))
		if col.HasLength() {
			buf.WriteByte('(')
			buf.WriteString(col.Length())
			buf.WriteByte(')')
		}
		if col.HasSortDirection() {
			if col.IsAscending() {
				buf.WriteString(" ASC")
			} else {
				buf.WriteString(" DESC")
			}
		}

		if i < lch-1 {
			buf.WriteString(", ")
		}
		i++
	}
	buf.WriteByte(')')

	switch {
	case index.IsFullText():
		for opt := range index.Options() {
			if opt.Key != "WITH PARSER" {
				continue
			}
			buf.WriteByte(' ')
			buf.WriteString("WITH PARSER")
			buf.WriteByte(' ')
			if opt.NeedQuotes {
				buf.WriteString(util.Backquote(opt.Value))
			} else {
				buf.WriteString(opt.Value)
			}
		}
	}

	if ref := index.Reference(); ref != nil {
		newctx := ctx.clone()
		newctx.dst = &buf

		buf.WriteByte(' ')
		if err := formatReference(newctx, ref); err != nil {
			return err
		}
	}

	if _, err := buf.WriteTo(ctx.dst); err != nil {
		return err
	}
	return nil
}

func formatReference(ctx *fmtCtx, r *model.Reference) error {
	var buf bytes.Buffer

	buf.WriteString(ctx.curIndent)
	buf.WriteString("REFERENCES ")
	buf.WriteString(util.Backquote(r.TableName))
	buf.WriteString(" (")

	ch := r.Columns
	lch := len(ch)
	for i, col := range ch {
		buf.WriteString(util.Backquote(col.Name()))
		if col.HasLength() {
			buf.WriteByte('(')
			buf.WriteString(col.Length())
			buf.WriteByte(')')
		}
		if i < lch-1 {
			buf.WriteString(", ")
		}
		i++
	}
	buf.WriteByte(')')

	switch r.Match {
	case model.ReferenceMatchFull:
		buf.WriteString(" MATCH FULL")
	case model.ReferenceMatchPartial:
		buf.WriteString(" MATCH PARTIAL")
	case model.ReferenceMatchSimple:
		buf.WriteString(" MATCH SIMPLE")
	}

	// we don't need to check the errors, because strings.Builder doesn't return any error.
	writeReferenceOption(&buf, "ON DELETE", r.OnDelete)
	writeReferenceOption(&buf, "ON UPDATE", r.OnUpdate)

	if _, err := buf.WriteTo(ctx.dst); err != nil {
		return err
	}
	return nil
}

func writeReferenceOption(buf *bytes.Buffer, prefix string, opt model.ReferenceOption) {
	if opt == model.ReferenceOptionNone {
		return
	}
	buf.WriteByte(' ')
	buf.WriteString(prefix)
	switch opt {
	case model.ReferenceOptionRestrict:
		buf.WriteString(" RESTRICT")
	case model.ReferenceOptionCascade:
		buf.WriteString(" CASCADE")
	case model.ReferenceOptionSetNull:
		buf.WriteString(" SET NULL")
	case model.ReferenceOptionNoAction:
		buf.WriteString(" NO ACTION")
	default:
		panic(fmt.Errorf("unknown reference option: %d", int(opt)))
	}
}
