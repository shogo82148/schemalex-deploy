// Package diff contains functions to generate SQL statements to
// migrate an old schema to the new schema
package diff

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sort"

	mapset "github.com/deckarep/golang-set"
	"github.com/shogo82148/schemalex-deploy"
	"github.com/shogo82148/schemalex-deploy/format"
	"github.com/shogo82148/schemalex-deploy/model"
)

type diffCtx struct {
	fromSet mapset.Set
	toSet   mapset.Set
	from    model.Stmts
	to      model.Stmts
	result  Stmts
}

func newDiffCtx(from, to model.Stmts) *diffCtx {
	fromSet := mapset.NewSet()
	for _, stmt := range from {
		if cs, ok := stmt.(*model.Table); ok {
			fromSet.Add(cs.ID())
		}
	}
	toSet := mapset.NewSet()
	for _, stmt := range to {
		if cs, ok := stmt.(*model.Table); ok {
			toSet.Add(cs.ID())
		}
	}

	return &diffCtx{
		fromSet: fromSet,
		toSet:   toSet,
		from:    from,
		to:      to,
	}
}

func (ctx *diffCtx) append(stmt string) {
	ctx.result.Append(Stmt(stmt))
}

// Diff compares two model.Stmts, and generates a series of
// statements as `diff.Stmts` so the consumer can, for example,
// analyze or use these statements standalone by themselves.
func Diff(from, to model.Stmts, options ...Option) (Stmts, error) {
	var txn bool
	for _, o := range options {
		switch o.Name() {
		case optkeyTransaction:
			txn = o.Value().(bool)
		}
	}

	ctx := newDiffCtx(from, to)

	if txn {
		ctx.append(`BEGIN`)
		ctx.append(`SET FOREIGN_KEY_CHECKS = 0`)
	}

	procs := []func() error{
		ctx.dropTables,
		ctx.createTables,
		ctx.alterTables,
	}
	for _, p := range procs {
		if err := p(); err != nil {
			return nil, fmt.Errorf("failed to produce diff: %w", err)
		}
	}

	if txn {
		ctx.append(`SET FOREIGN_KEY_CHECKS = 1`)
		ctx.append(`COMMIT`)
	}

	return ctx.result, nil
}

// Statements compares two model.Stmts and generates a series
// of statements to migrate from the old one to the new one,
// writing the result to `dst`
func Statements(dst io.Writer, from, to model.Stmts, options ...Option) error {
	stmts, err := Diff(from, to, options...)
	if err != nil {
		return err
	}

	if _, err := stmts.WriteTo(dst); err != nil {
		return fmt.Errorf("failed to write diff: %w", err)
	}
	return nil
}

// Strings compares two strings and generates a series
// of statements to migrate from the old one to the new one,
// writing the result to `dst`
func Strings(dst io.Writer, from, to string, options ...Option) error {
	var p *schemalex.Parser
	for _, o := range options {
		switch o.Name() {
		case optkeyParser:
			p = o.Value().(*schemalex.Parser)
		}
	}
	if p == nil {
		p = schemalex.New()
	}

	stmts1, err := p.ParseString(from)
	if err != nil {
		return fmt.Errorf(`failed to parse "from" %s: %w`, from, err)
	}

	stmts2, err := p.ParseString(to)
	if err != nil {
		return fmt.Errorf(`failed to parse "to" %s: %w`, to, err)
	}

	return Statements(dst, stmts1, stmts2, options...)
}

func (ctx *diffCtx) dropTables() error {
	ids := ctx.fromSet.Difference(ctx.toSet)
	for _, id := range ids.ToSlice() {
		stmt, ok := ctx.from.Lookup(id.(string))
		if !ok {
			return fmt.Errorf("failed to lookup table: %q", id)
		}

		table, ok := stmt.(*model.Table)
		if !ok {
			return fmt.Errorf(`lookup failed: %q is not a model.Table`, id)
		}
		ctx.append("DROP TABLE " + table.Name.Quoted())
	}
	return nil
}

func (ctx *diffCtx) createTables() error {
	var buf bytes.Buffer

	ids := ctx.toSet.Difference(ctx.fromSet)
	for _, id := range ids.ToSlice() {
		// Lookup the corresponding statement, and add its SQL
		stmt, ok := ctx.to.Lookup(id.(string))
		if !ok {
			return fmt.Errorf("failed to lookup table: %q", id)
		}

		buf.Reset()
		if err := format.SQL(&buf, stmt); err != nil {
			return fmt.Errorf("failed to format a statement: %w", err)
		}
		ctx.append(buf.String())
	}
	return nil
}

type alterCtx struct {
	fromColumns mapset.Set
	toColumns   mapset.Set
	fromIndexes mapset.Set
	toIndexes   mapset.Set
	from        *model.Table
	to          *model.Table
	result      Stmts
}

func newAlterCtx(ctx *diffCtx, from, to *model.Table) *alterCtx {
	fromColumns := mapset.NewSet()
	for _, col := range from.Columns {
		fromColumns.Add(col.ID())
	}

	toColumns := mapset.NewSet()
	for _, col := range to.Columns {
		toColumns.Add(col.ID())
	}

	fromIndexes := mapset.NewSet()
	for _, idx := range from.Indexes {
		fromIndexes.Add(idx.ID())
	}

	toIndexes := mapset.NewSet()
	for _, idx := range to.Indexes {
		toIndexes.Add(idx.ID())
	}

	return &alterCtx{
		fromColumns: fromColumns,
		toColumns:   toColumns,
		fromIndexes: fromIndexes,
		toIndexes:   toIndexes,
		from:        from,
		to:          to,
		result:      ctx.result,
	}
}

func (ctx *alterCtx) append(stmt string) {
	ctx.result.Append(Stmt(stmt))
}

func (ctx *diffCtx) alterTables() error {
	procs := []func(*alterCtx) error{
		(*alterCtx).dropTableIndexes,
		(*alterCtx).dropTableColumns,
		(*alterCtx).addTableColumns,
		(*alterCtx).alterTableColumns,
		(*alterCtx).addTableIndexes,
	}

	ids := ctx.toSet.Intersect(ctx.fromSet)
	for _, id := range ids.ToSlice() {
		var stmt model.Stmt
		var ok bool

		stmt, ok = ctx.from.Lookup(id.(string))
		if !ok {
			return fmt.Errorf("table not found in old schema (alter table): %q", id)
		}
		beforeStmt := stmt.(*model.Table)

		stmt, ok = ctx.to.Lookup(id.(string))
		if !ok {
			return fmt.Errorf("table not found in new schema (alter table): %q", id)
		}
		afterStmt := stmt.(*model.Table)

		alterCtx := newAlterCtx(ctx, beforeStmt, afterStmt)
		for _, p := range procs {
			if err := p(alterCtx); err != nil {
				return fmt.Errorf("failed to generate alter table %q: %w", id, err)
			}
		}
		ctx.result = alterCtx.result
	}

	return nil
}

func (ctx *alterCtx) dropTableColumns() error {
	columnNames := ctx.fromColumns.Difference(ctx.toColumns)

	var buf bytes.Buffer
	for _, columnName := range columnNames.ToSlice() {
		buf.Reset()
		buf.WriteString("ALTER TABLE ")
		buf.WriteString(ctx.from.Name.Quoted())
		buf.WriteString(" DROP COLUMN ")
		col, ok := ctx.from.LookupColumn(columnName.(string))
		if !ok {
			return fmt.Errorf("failed to lookup column %q", columnName)
		}
		buf.WriteString(col.Name.Quoted())
		ctx.append(buf.String())
	}
	return nil
}

func (ctx *alterCtx) addTableColumns() error {
	beforeToNext := make(map[string]string) // lookup next column
	nextToBefore := make(map[string]string) // lookup before column

	// In order to do this correctly, we need to create a graph so that
	// we always start adding with a column that has a either no before
	// columns, or one that already exists in the database
	var firstColumn *model.TableColumn
	for _, v := range ctx.toColumns.Difference(ctx.fromColumns).ToSlice() {
		columnName := v.(string)
		// find the before-column for each.
		col, ok := ctx.to.LookupColumn(columnName)
		if !ok {
			return fmt.Errorf("failed to lookup column %q", columnName)
		}

		beforeCol, hasBeforeCol := ctx.to.LookupColumnBefore(col.ID())
		if !hasBeforeCol {
			// if there is no before-column, then this is a special "FIRST"
			// column
			firstColumn = col
			continue
		}

		// otherwise, keep a reverse-lookup map of before -> next columns
		beforeToNext[beforeCol.ID()] = columnName
		nextToBefore[columnName] = beforeCol.ID()
	}

	// First column is always safe to add
	if firstColumn != nil {
		ctx.writeAddColumn(firstColumn.ID())
	}

	var columnNames []string
	// Find columns that have before columns which existed in both
	// from and to tables
	for _, v := range ctx.toColumns.Intersect(ctx.fromColumns).ToSlice() {
		columnName := v.(string)
		if nextColumnName, ok := beforeToNext[columnName]; ok {
			delete(beforeToNext, columnName)
			delete(nextToBefore, nextColumnName)
			columnNames = append(columnNames, nextColumnName)
		}
	}

	if len(columnNames) > 0 {
		sort.Strings(columnNames)
		ctx.writeAddColumn(columnNames...)
	}

	// Finally, we process the remaining columns.
	// All remaining columns are new, and they will depend on a
	// newly created column. This means we have to make sure to
	// create them in the order that they are dependent on.
	columnNames = columnNames[:0]
	for _, nextCol := range beforeToNext {
		columnNames = append(columnNames, nextCol)
	}
	// if there's one left, that can be appended
	if len(columnNames) > 0 {
		sort.Slice(columnNames, func(i, j int) bool {
			icol, _ := ctx.to.LookupColumnOrder(columnNames[i])
			jcol, _ := ctx.to.LookupColumnOrder(columnNames[j])
			return icol < jcol
		})
		ctx.writeAddColumn(columnNames...)
	}
	return nil
}

func (ctx *alterCtx) writeAddColumn(columnNames ...string) error {
	var buf bytes.Buffer
	for _, columnName := range columnNames {
		stmt, ok := ctx.to.LookupColumn(columnName)
		if !ok {
			return fmt.Errorf("failed to lookup column %q", columnName)
		}

		beforeCol, hasBeforeCol := ctx.to.LookupColumnBefore(stmt.ID())
		buf.Reset()
		buf.WriteString("ALTER TABLE ")
		buf.WriteString(ctx.from.Name.Quoted())
		buf.WriteString(" ADD COLUMN ")
		if err := format.SQL(&buf, stmt); err != nil {
			return err
		}
		if hasBeforeCol {
			buf.WriteString(" AFTER ")
			buf.WriteString(beforeCol.Name.Quoted())
			buf.WriteString("")
		} else {
			buf.WriteString(" FIRST")
		}
		ctx.append(buf.String())
	}
	return nil
}

func (ctx *alterCtx) alterTableColumns() error {
	var buf bytes.Buffer
	columnNames := ctx.toColumns.Intersect(ctx.fromColumns)
	for _, columnName := range columnNames.ToSlice() {
		beforeColumnStmt, ok := ctx.from.LookupColumn(columnName.(string))
		if !ok {
			return fmt.Errorf("column not found in old schema: %q", columnName)
		}

		afterColumnStmt, ok := ctx.to.LookupColumn(columnName.(string))
		if !ok {
			return fmt.Errorf("column not found in new schema: %q", columnName)
		}

		if reflect.DeepEqual(beforeColumnStmt, afterColumnStmt) {
			continue
		}

		buf.Reset()
		buf.WriteString("ALTER TABLE ")
		buf.WriteString(ctx.from.Name.Quoted())
		buf.WriteString(" CHANGE COLUMN ")
		buf.WriteString(afterColumnStmt.Name.Quoted())
		buf.WriteString(" ")
		if err := format.SQL(&buf, afterColumnStmt); err != nil {
			return err
		}
		ctx.append(buf.String())
	}
	return nil
}

func (ctx *alterCtx) dropTableIndexes() error {
	var buf bytes.Buffer
	indexes := ctx.fromIndexes.Difference(ctx.toIndexes)
	// drop index after drop constraint.
	// because cannot drop index if needed in a foreign key constraint
	lazy := make([]*model.Index, 0, indexes.Cardinality())
	for _, index := range indexes.ToSlice() {
		indexStmt, ok := ctx.from.LookupIndex(index.(string))
		if !ok {
			return fmt.Errorf("index not found in old schema: %q", index)
		}

		if indexStmt.Kind == model.IndexKindPrimaryKey {
			buf.Reset()
			buf.WriteString("ALTER TABLE ")
			buf.WriteString(ctx.from.Name.Quoted())
			buf.WriteString(" DROP PRIMARY KEY")
			ctx.append(buf.String())
			continue
		}

		if !indexStmt.Name.Valid && !indexStmt.Symbol.Valid {
			return fmt.Errorf("can not drop index without name: %q", indexStmt.ID())
		}
		if indexStmt.Kind != model.IndexKindForeignKey {
			lazy = append(lazy, indexStmt)
			continue
		}

		buf.Reset()
		buf.WriteString("ALTER TABLE ")
		buf.WriteString(ctx.from.Name.Quoted())
		buf.WriteString(" DROP FOREIGN KEY ")
		if indexStmt.Symbol.Valid {
			buf.WriteString(indexStmt.Symbol.Quoted())
		} else {
			buf.WriteString(indexStmt.Name.Quoted())
		}
		ctx.append(buf.String())
	}

	// drop index after drop CONSTRAINT
	for _, indexStmt := range lazy {
		buf.Reset()
		buf.WriteString("ALTER TABLE ")
		buf.WriteString(ctx.from.Name.Quoted())
		buf.WriteString(" DROP INDEX ")
		if !indexStmt.Name.Valid {
			buf.WriteString(indexStmt.Symbol.Quoted())
		} else {
			buf.WriteString(indexStmt.Name.Quoted())
		}
		ctx.append(buf.String())
	}

	return nil
}

func (ctx *alterCtx) addTableIndexes() error {
	var buf bytes.Buffer
	indexes := ctx.toIndexes.Difference(ctx.fromIndexes)
	// add index before add foreign key.
	// because cannot add index if create implicitly index by foreign key.
	lazy := make([]*model.Index, 0, indexes.Cardinality())
	for _, index := range indexes.ToSlice() {
		indexStmt, ok := ctx.to.LookupIndex(index.(string))
		if !ok {
			return fmt.Errorf("index not found in old schema: %q", index)
		}
		if indexStmt.Kind == model.IndexKindForeignKey {
			lazy = append(lazy, indexStmt)
			continue
		}

		buf.Reset()
		buf.WriteString("ALTER TABLE ")
		buf.WriteString(ctx.from.Name.Quoted())
		buf.WriteString(" ADD ")
		if err := format.SQL(&buf, indexStmt); err != nil {
			return err
		}
		ctx.append(buf.String())
	}

	for _, indexStmt := range lazy {
		buf.Reset()
		buf.WriteString("ALTER TABLE ")
		buf.WriteString(ctx.from.Name.Quoted())
		buf.WriteString(" ADD ")
		if err := format.SQL(&buf, indexStmt); err != nil {
			return err
		}
		ctx.append(buf.String())
	}

	return nil
}
