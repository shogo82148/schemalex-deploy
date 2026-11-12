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
	indent  string

	tableNormalizer TableNormalizer
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
	var opts myOptions
	for _, opt := range options {
		opt.apply(&opts)
	}
	p := opts.parser
	txn := opts.transaction
	current := opts.currentSchema

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
	ctx.indent = opts.indent
	ctx.tableNormalizer = opts.tableNormalizer

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
	var opts myOptions
	for _, opt := range options {
		opt.apply(&opts)
	}
	p := opts.parser

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
		if err := format.SQL(&buf, stmt, format.WithIndent(ctx.indent, 1)); err != nil {
			return fmt.Errorf("failed to format a statement: %w", err)
		}
		ctx.append(buf.String())
	}
	return nil
}

type alterCtx struct {
	fromColumns set
	toColumns   set

	// recreateColumns are the columns that exist in both from and to,
	// but cannot be changed by CHANGE COLUMN.
	// They are dropped and added again.
	recreateColumns set

	// fromIndexes and toIndexes are the keys of the indexes, see indexKeys.
	fromIndexes      set
	toIndexes        set
	fromIndexesByKey map[string]*model.Index
	toIndexesByKey   map[string]*model.Index
	indexKeyOrder    map[string]indexKey
	from             *model.Table
	to               *model.Table
	buf              strings.Builder

	// cur is the current model deployed to MySQL actually.
	// it may be nil.
	cur *model.Table

	tableNormalizer TableNormalizer

	// normalizedFrom and normalizedTo are from and to normalized by tableNormalizer.
	// they are computed lazily, and nil if they are not computed yet or cannot be normalized.
	normalized                   bool
	normalizedFrom, normalizedTo *model.Table
}

func (ctx *diffCtx) alterTables() error {
	procs := []func(*alterCtx) error{
		(*alterCtx).dropTableIndexes,
		(*alterCtx).dropTableColumns,
		(*alterCtx).addTableColumns,
		(*alterCtx).alterTableColumns,
		(*alterCtx).addTableIndexes,
		(*alterCtx).alterTableIndexes,
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

		alterCtx, err := newAlterCtx(ctx, beforeStmt, afterStmt, curStmt)
		if err != nil {
			return fmt.Errorf("failed to generate alter table %q: %w", id, err)
		}
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

func newAlterCtx(ctx *diffCtx, from, to, cur *model.Table) (*alterCtx, error) {
	fromColumns := newSet()
	for _, col := range from.Columns {
		fromColumns.Add(col.ID())
	}

	toColumns := newSet()
	for _, col := range to.Columns {
		toColumns.Add(col.ID())
	}

	recreateColumns := newSet()
	for _, columnName := range toColumns.Intersect(fromColumns).ToSlice() {
		fromCol, ok := from.LookupColumn(columnName)
		if !ok {
			return nil, fmt.Errorf("column not found in old schema: %q", columnName)
		}
		toCol, ok := to.LookupColumn(columnName)
		if !ok {
			return nil, fmt.Errorf("column not found in new schema: %q", columnName)
		}
		recreate, err := needsRecreateColumn(fromCol, toCol)
		if err != nil {
			return nil, err
		}
		if recreate {
			recreateColumns.Add(columnName)
		}
	}

	indexKeyOrder := make(map[string]indexKey, len(from.Indexes)+len(to.Indexes))
	fromIndexes, fromIndexesByKey := indexKeys(from.Indexes, indexKeyOrder)
	toIndexes, toIndexesByKey := indexKeys(to.Indexes, indexKeyOrder)

	return &alterCtx{
		fromColumns:      fromColumns,
		toColumns:        toColumns,
		recreateColumns:  recreateColumns,
		fromIndexes:      fromIndexes,
		toIndexes:        toIndexes,
		fromIndexesByKey: fromIndexesByKey,
		toIndexesByKey:   toIndexesByKey,
		indexKeyOrder:    indexKeyOrder,
		from:             from,
		to:               to,
		cur:              cur,
		tableNormalizer:  ctx.tableNormalizer,
	}, nil
}

// indexKeys returns the keys that identify the indexes, and the indexes by their keys.
// Usually the key is the ID of the index.
// However, some indexes have the same ID, e.g. anonymous indexes with the same definition such as `INDEX (a), INDEX (a)`.
// They are distinguished by their order: the first one is keyed by its ID, and the n-th one by "<ID>#<n>".
// The ID and the order of each key are recorded in order, see sortIndexKeys.
func indexKeys(indexes []*model.Index, order map[string]indexKey) (set, map[string]*model.Index) {
	keys := newSet()
	byKey := make(map[string]*model.Index, len(indexes))
	count := make(map[string]int, len(indexes))
	for _, idx := range indexes {
		id := idx.ID()
		count[id]++
		n := count[id]
		key := id
		if n > 1 {
			key = fmt.Sprintf("%s#%d", id, n)
		}
		keys.Add(key)
		byKey[key] = idx
		order[key] = indexKey{id: id, n: n}
	}
	return keys, byKey
}

// indexKey is the n-th index with the ID in a table.
type indexKey struct {
	id string
	n  int
}

// sortIndexKeys returns the keys of the indexes, sorted by their IDs and then by their order.
// It does not use set.ToSlice, because it sorts "<ID>#10" before "<ID>#2".
// The order matters when adding the indexes with the same ID,
// because MySQL names them in the order they are added, e.g. `a_2`, `a_3`, ...
func (ctx *alterCtx) sortIndexKeys(s set) []string {
	keys := make([]string, 0, len(s))
	for key := range s {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := ctx.indexKeyOrder[keys[i]], ctx.indexKeyOrder[keys[j]]
		if a.id != b.id {
			return a.id < b.id
		}
		return a.n < b.n
	})
	return keys
}

// needsRecreateColumn reports whether the column must be dropped and added again
// to change it from `from` to `to`.
//
// MySQL does not support changing a column into or out of a VIRTUAL generated column by CHANGE COLUMN.
// e.g. VIRTUAL <-> STORED, and non-generated -> VIRTUAL.
// It fails with "ERROR 3106: 'Changing the STORED status' is not supported for generated columns."
// https://dev.mysql.com/doc/refman/8.4/en/alter-table-generated-columns.html
//
// Dropping and adding the column again works for them, because the values of generated columns are computed.
// However, it loses the values when changing a VIRTUAL generated column into a non-generated column,
// so it is reported as an error.
func needsRecreateColumn(from, to *model.TableColumn) (bool, error) {
	fromVirtual := from.Generated.Valid && !from.Generated.Stored
	toVirtual := to.Generated.Valid && !to.Generated.Stored
	if fromVirtual == toVirtual {
		return false, nil
	}
	if !to.Generated.Valid {
		return false, fmt.Errorf("cannot change the virtual generated column %q into a non-generated column: it is not supported by MySQL, and dropping and adding the column again loses its values", from.Name)
	}
	return true, nil
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
	columnNames := ctx.fromColumns.Difference(ctx.toColumns).Union(ctx.recreateColumns)

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
	for _, columnName := range ctx.toColumns.Difference(ctx.fromColumns).Union(ctx.recreateColumns).ToSlice() {
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
	for _, columnName := range ctx.toColumns.Intersect(ctx.fromColumns).Difference(ctx.recreateColumns).ToSlice() {
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
	columnNames := ctx.toColumns.Intersect(ctx.fromColumns).Difference(ctx.recreateColumns)
	for _, columnName := range columnNames.ToSlice() {
		beforeColumnStmt, ok := ctx.from.LookupColumn(columnName)
		if !ok {
			return fmt.Errorf("column not found in old schema: %q", columnName)
		}

		afterColumnStmt, ok := ctx.to.LookupColumn(columnName)
		if !ok {
			return fmt.Errorf("column not found in new schema: %q", columnName)
		}

		if beforeColumnStmt.Generated.Valid && afterColumnStmt.Generated.Valid &&
			beforeColumnStmt.Generated.Expr != afterColumnStmt.Generated.Expr {
			equal, err := ctx.equalGeneratedExpr(columnName)
			if err != nil {
				return err
			}
			if equal {
				// the expressions are equivalent. compare the other attributes.
				col := *beforeColumnStmt
				col.Generated.Expr = afterColumnStmt.Generated.Expr
				beforeColumnStmt = &col
			}
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

// equalGeneratedExpr reports whether the expressions of the generated column are equivalent
// after they are normalized by tableNormalizer.
// It returns false if tableNormalizer is not specified or the tables cannot be normalized.
func (ctx *alterCtx) equalGeneratedExpr(columnName string) (bool, error) {
	if ctx.tableNormalizer == nil {
		return false, nil
	}
	if !ctx.normalized {
		ctx.normalized = true
		from, err := ctx.tableNormalizer(ctx.from)
		if err != nil {
			return false, fmt.Errorf("failed to normalize the old table %q: %w", ctx.from.Name, err)
		}
		to, err := ctx.tableNormalizer(ctx.to)
		if err != nil {
			return false, fmt.Errorf("failed to normalize the new table %q: %w", ctx.to.Name, err)
		}
		ctx.normalizedFrom, ctx.normalizedTo = from, to
	}
	if ctx.normalizedFrom == nil || ctx.normalizedTo == nil {
		return false, nil
	}

	from, ok := ctx.normalizedFrom.LookupColumn(columnName)
	if !ok || !from.Generated.Valid {
		return false, nil
	}
	to, ok := ctx.normalizedTo.LookupColumn(columnName)
	if !ok || !to.Generated.Valid {
		return false, nil
	}
	return from.Generated.Expr == to.Generated.Expr, nil
}

func (ctx *alterCtx) dropTableIndexes() error {
	indexes := ctx.fromIndexes.Difference(ctx.toIndexes)
	// drop index after drop constraint.
	// because cannot drop index if needed in a foreign key constraint
	lazy := make([]model.Ident, 0, indexes.Cardinality())
	for _, index := range ctx.sortIndexKeys(indexes) {
		indexStmt, ok := ctx.fromIndexesByKey[index]
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
			name, err := ctx.guessTableIndexName(indexStmt)
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
	for _, index := range ctx.sortIndexKeys(indexes) {
		indexStmt, ok := ctx.toIndexesByKey[index]
		if !ok {
			return fmt.Errorf("index not found in new schema: %q", index)
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

// alterTableIndexes changes the visibility of the indexes that exist in both from and to.
// The visibility can be changed without rebuilding the index.
// https://dev.mysql.com/doc/refman/26.10/en/invisible-indexes.html
func (ctx *alterCtx) alterTableIndexes() error {
	indexes := ctx.toIndexes.Intersect(ctx.fromIndexes)
	for _, index := range ctx.sortIndexKeys(indexes) {
		beforeIndexStmt, ok := ctx.fromIndexesByKey[index]
		if !ok {
			return fmt.Errorf("index not found in old schema: %q", index)
		}
		afterIndexStmt, ok := ctx.toIndexesByKey[index]
		if !ok {
			return fmt.Errorf("index not found in new schema: %q", index)
		}
		if beforeIndexStmt.Invisible == afterIndexStmt.Invisible {
			continue
		}

		indexName := getIndexName(beforeIndexStmt)
		if !indexName.Valid {
			name, err := ctx.guessTableIndexName(beforeIndexStmt)
			if err != nil {
				return err
			}
			indexName.Valid = true
			indexName.Ident = name
		}

		ctx.begin()
		ctx.writeString("ALTER INDEX ")
		ctx.writeIdent(indexName.Ident)
		if afterIndexStmt.Invisible {
			ctx.writeString(" INVISIBLE")
		} else {
			ctx.writeString(" VISIBLE")
		}
	}
	return nil
}

// guessTableIndexName guesses the name of the index that does not have its name, from the current schema.
func (ctx *alterCtx) guessTableIndexName(indexStmt *model.Index) (name model.Ident, err error) {
	cur := ctx.cur
	if cur == nil {
		return "", fmt.Errorf("can not find the name of the index: %q", indexStmt.ID())
	}

	// MySQL names anonymous indexes with the same definition in order, e.g. `a`, `a_2`, ...
	// so the n-th one in the "from" corresponds to the n-th candidate in the current schema.
	nth := 0
	for _, idx := range ctx.from.Indexes {
		if idx == indexStmt {
			break
		}
		if !getIndexName(idx).Valid && equalIndex(idx, indexStmt) {
			nth++
		}
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

		if nth > 0 {
			nth--
			continue
		}
		return name.Ident, nil // found
	}
	return "", fmt.Errorf("can not find the name of the index: %q", indexStmt.ID())
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
