// Package diff contains functions to generate SQL statements to
// migrate an old schema to the new schema
package diff

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"

	"github.com/shogo82148/schemalex-deploy"
	"github.com/shogo82148/schemalex-deploy/format"
	"github.com/shogo82148/schemalex-deploy/model"
)

type diffCtx struct {
	fromSet set
	toSet   set
	from    model.Stmts
	to      model.Stmts
	cur     model.Stmts
	result  Stmts
}

func newDiffCtx(from, to, cur model.Stmts) *diffCtx {
	fromSet := newSet()
	for _, stmt := range from {
		if cs, ok := stmt.(*model.Table); ok {
			fromSet.Add(cs.ID())
		}
	}
	toSet := newSet()
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
		cur:     cur,
	}
}

func (ctx *diffCtx) append(stmt string) {
	ctx.result.Append(Stmt(stmt))
}

// Diff compares two model.Stmts, and generates a series of
// statements as `diff.Stmts` so the consumer can, for example,
// analyze or use these statements standalone by themselves.
func Diff(from, to model.Stmts, options ...Option) (Stmts, error) {
	var p *schemalex.Parser
	var txn bool
	var current string
	for _, o := range options {
		switch o.Name() {
		case optkeyParser:
			p = o.Value().(*schemalex.Parser)
		case optkeyTransaction:
			txn = o.Value().(bool)
		case optkeyCurrent:
			current = o.Value().(string)
		}
	}

	if p == nil {
		p = schemalex.New()
	}
	var cur model.Stmts
	if current != "" {
		var err error
		cur, err = p.ParseString(current)
		if err != nil {
			return nil, err
		}
	}
	ctx := newDiffCtx(from, to, cur)

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
		stmt, ok := ctx.from.Lookup(id)
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
		stmt, ok := ctx.to.Lookup(id)
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
	fromColumns set
	toColumns   set
	fromIndexes set
	toIndexes   set
	from        *model.Table
	to          *model.Table
	buf         strings.Builder

	// cur is the current model deployed to MySQL actually.
	// it may be nil.
	cur *model.Table
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

		// before statement
		stmt, ok = ctx.from.Lookup(id)
		if !ok {
			return fmt.Errorf("table not found in old schema (alter table): %q", id)
		}
		beforeStmt := stmt.(*model.Table)

		// after statement
		stmt, ok = ctx.to.Lookup(id)
		if !ok {
			return fmt.Errorf("table not found in new schema (alter table): %q", id)
		}
		afterStmt := stmt.(*model.Table)

		// current statement
		var curStmt *model.Table
		if ctx.cur != nil {
			stmt, ok = ctx.cur.Lookup(id)
			if ok {
				curStmt = stmt.(*model.Table)
			}
		}

		alterCtx := newAlterCtx(ctx, beforeStmt, afterStmt, curStmt)
		for _, p := range procs {
			if err := p(alterCtx); err != nil {
				return fmt.Errorf("failed to generate alter table %q: %w", id, err)
			}
		}
		if alterCtx.buf.Len() > 0 {
			ctx.result = append(ctx.result, Stmt(alterCtx.buf.String()))
		}
	}

	return nil
}

func newAlterCtx(ctx *diffCtx, from, to, cur *model.Table) *alterCtx {
	fromColumns := newSet()
	for _, col := range from.Columns {
		fromColumns.Add(col.ID())
	}

	toColumns := newSet()
	for _, col := range to.Columns {
		toColumns.Add(col.ID())
	}

	fromIndexes := newSet()
	for _, idx := range from.Indexes {
		fromIndexes.Add(idx.ID())
	}

	toIndexes := newSet()
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
		cur:         cur,
	}
}

// begin begins a new alter specification.
func (ctx *alterCtx) begin() {
	if ctx.buf.Len() == 0 {
		ctx.writeString("ALTER TABLE ")
		ctx.writeIdent(ctx.from.Name)
		ctx.writeString(" ")
	} else {
		ctx.writeString(", ")
	}
}

func (ctx *alterCtx) writeString(s string) {
	ctx.buf.WriteString(s)
}

func (ctx *alterCtx) writeIdent(ident model.Ident) {
	ctx.buf.WriteString(ident.Quoted())
}

func (ctx *alterCtx) dropTableColumns() error {
	columnNames := ctx.fromColumns.Difference(ctx.toColumns)

	for _, columnName := range columnNames.ToSlice() {
		ctx.begin()
		ctx.writeString("DROP COLUMN ")
		col, ok := ctx.from.LookupColumn(columnName)
		if !ok {
			return fmt.Errorf("failed to lookup column %q", columnName)
		}
		ctx.writeIdent(col.Name)
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
	for _, columnName := range ctx.toColumns.Difference(ctx.fromColumns).ToSlice() {
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
	for _, columnName := range ctx.toColumns.Intersect(ctx.fromColumns).ToSlice() {
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
	for _, columnName := range columnNames {
		stmt, ok := ctx.to.LookupColumn(columnName)
		if !ok {
			return fmt.Errorf("failed to lookup column %q", columnName)
		}

		beforeCol, hasBeforeCol := ctx.to.LookupColumnBefore(stmt.ID())
		ctx.begin()
		ctx.writeString("ADD COLUMN ")
		if err := format.SQL(&ctx.buf, stmt); err != nil {
			return err
		}

		if hasBeforeCol {
			ctx.writeString(" AFTER ")
			ctx.writeIdent(beforeCol.Name)
		} else {
			ctx.writeString(" FIRST")
		}
	}
	return nil
}

func (ctx *alterCtx) alterTableColumns() error {
	columnNames := ctx.toColumns.Intersect(ctx.fromColumns)
	for _, columnName := range columnNames.ToSlice() {
		beforeColumnStmt, ok := ctx.from.LookupColumn(columnName)
		if !ok {
			return fmt.Errorf("column not found in old schema: %q", columnName)
		}

		afterColumnStmt, ok := ctx.to.LookupColumn(columnName)
		if !ok {
			return fmt.Errorf("column not found in new schema: %q", columnName)
		}

		if reflect.DeepEqual(beforeColumnStmt, afterColumnStmt) {
			continue
		}

		ctx.begin()
		ctx.writeString("CHANGE COLUMN ")
		ctx.writeIdent(afterColumnStmt.Name)
		ctx.writeString(" ")
		if err := format.SQL(&ctx.buf, afterColumnStmt); err != nil {
			return err
		}
	}
	return nil
}

func (ctx *alterCtx) dropTableIndexes() error {
	indexes := ctx.fromIndexes.Difference(ctx.toIndexes)
	// drop index after drop constraint.
	// because cannot drop index if needed in a foreign key constraint
	lazy := make([]model.Ident, 0, indexes.Cardinality())
	for _, index := range indexes.ToSlice() {
		indexStmt, ok := ctx.from.LookupIndex(index)
		if !ok {
			return fmt.Errorf("index not found in old schema: %q", index)
		}

		if indexStmt.Kind == model.IndexKindPrimaryKey {
			ctx.begin()
			ctx.writeString("DROP PRIMARY KEY")
			continue
		}

		indexName := getIndexName(indexStmt)
		if !indexName.Valid {
			name, err := ctx.guessDropTableIndexName(indexStmt)
			if err != nil {
				return err
			}
			indexName.Valid = true
			indexName.Ident = name
		}
		if indexStmt.Kind != model.IndexKindForeignKey {
			lazy = append(lazy, indexName.Ident)
			continue
		}

		ctx.begin()
		ctx.writeString("DROP FOREIGN KEY ")
		ctx.writeIdent(indexName.Ident)
	}

	// drop index after drop CONSTRAINT
	for _, indexName := range lazy {
		ctx.begin()
		ctx.writeString("DROP INDEX ")
		ctx.writeIdent(indexName)
	}

	return nil
}

func (ctx *alterCtx) addTableIndexes() error {
	indexes := ctx.toIndexes.Difference(ctx.fromIndexes)
	// add index before add foreign key.
	// because cannot add index if create implicitly index by foreign key.
	lazy := make([]*model.Index, 0, indexes.Cardinality())
	for _, index := range indexes.ToSlice() {
		indexStmt, ok := ctx.to.LookupIndex(index)
		if !ok {
			return fmt.Errorf("index not found in old schema: %q", index)
		}
		if indexStmt.Kind == model.IndexKindForeignKey {
			lazy = append(lazy, indexStmt)
			continue
		}

		ctx.begin()
		ctx.writeString("ADD ")
		if err := format.SQL(&ctx.buf, indexStmt); err != nil {
			return err
		}
	}

	for _, indexStmt := range lazy {
		ctx.begin()
		ctx.writeString("ADD ")
		if err := format.SQL(&ctx.buf, indexStmt); err != nil {
			return err
		}
	}

	return nil
}

func (ctx *alterCtx) guessDropTableIndexName(indexStmt *model.Index) (name model.Ident, err error) {
	cur := ctx.cur
	if cur == nil {
		return "", fmt.Errorf("can not drop index without name: %q", indexStmt.ID())
	}

	// Guess the name from the current schema
LOOP:
	for _, idx := range cur.Indexes {
		// find the index that has same definition with indexStmt.
		if !equalIndex(idx, indexStmt) {
			continue
		}

		name := getIndexName(idx)
		if !name.Valid {
			continue
		}

		// this name should not be used in the "from".
		for _, idx2 := range ctx.from.Indexes {
			name2 := getIndexName(idx2)
			if name2.Valid && name2.Ident == name.Ident {
				continue LOOP
			}
		}

		return name.Ident, nil // found
	}
	return "", fmt.Errorf("can not drop index without name: %q", indexStmt.ID())
}

func getIndexName(idx *model.Index) model.MaybeIdent {
	if idx.Name.Valid {
		return idx.Name
	}
	if idx.ConstraintName.Valid {
		return idx.ConstraintName
	}
	return model.MaybeIdent{}
}

// equalIndex returns whether index a and b have same definition, excluding their names.
func equalIndex(a, b *model.Index) bool {
	if a.Table != b.Table {
		return false
	}
	if a.Type != b.Type {
		return false
	}
	if a.Kind != b.Kind {
		return false
	}
	if len(a.Columns) != len(b.Columns) {
		return false
	}
	for i := range a.Columns {
		if a.Columns[i].ID() != b.Columns[i].ID() {
			return false
		}
	}
	if (a.Reference != nil) != (b.Reference != nil) {
		return false
	}
	if a.Reference != nil {
		if a.Reference.ID() != b.Reference.ID() {
			return false
		}
	}
	return true
}
