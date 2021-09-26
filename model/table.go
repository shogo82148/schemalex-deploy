package model

import "strings"

// Table describes a table model
type Table struct {
	Name        string
	Temporary   bool
	IfNotExists bool
	LikeTable   maybeString
	Columns     []TableColumn
	Indexes     []Index
	Options     []TableOption
}

// NewTable create a new table with the given name
func NewTable(name string) *Table {
	return &Table{
		Name: name,
	}
}

func (t *Table) ID() string {
	return "table#" + strings.ToLower(t.Name)
}

func (t *Table) LookupColumn(id string) (TableColumn, bool) {
	for _, col := range t.Columns {
		if col.ID() == id {
			return col, true
		}
	}
	return nil, false
}

func (t *Table) LookupColumnOrder(id string) (int, bool) {
	for i, col := range t.Columns {
		if col.ID() == id {
			return i, true
		}
	}
	return 0, false
}

func (t *Table) LookupColumnBefore(id string) (TableColumn, bool) {
	for i, col := range t.Columns {
		if col.ID() == id {
			if i > 0 {
				return t.Columns[i-1], true
			}
			return nil, false
		}
	}
	return nil, false
}

func (t *Table) LookupIndex(id string) (Index, bool) {
	for _, idx := range t.Indexes {
		if idx.ID() == id {
			return idx, true
		}
	}
	return nil, false
}

func (t *Table) Normalize() (*Table, bool) {
	var clone bool
	var additionalIndexes []Index
	var columns []TableColumn
	for _, col := range t.Columns {
		ncol, modified := col.Normalize()
		if modified {
			clone = true
		}

		// column_definition [UNIQUE [KEY] | [PRIMARY] KEY]
		// they mean same as INDEX or CONSTRAINT
		switch {
		case ncol.IsPrimary():
			// we have to move off the index declaration from the
			// primary key column to an index associated with the table
			index := NewIndex(IndexKindPrimaryKey, t.ID())
			index.SetType(IndexTypeNone)
			idxCol := NewIndexColumn(ncol.Name())
			index.AddColumns(idxCol)
			additionalIndexes = append(additionalIndexes, index)
			if !modified {
				clone = true
			}
			ncol = ncol.Clone()
			ncol.SetPrimary(false)
		case ncol.IsUnique():
			index := NewIndex(IndexKindUnique, t.ID())
			// if you do not assign a name, the index is assigned the same name as the first indexed column
			index.SetName(ncol.Name())
			index.SetType(IndexTypeNone)
			idxCol := NewIndexColumn(ncol.Name())
			index.AddColumns(idxCol)
			additionalIndexes = append(additionalIndexes, index)
			if !modified {
				clone = true
			}
			ncol = ncol.Clone()
			ncol.SetUnique(false)
		}

		columns = append(columns, ncol)
	}

	var indexes []Index
	var seen = make(map[string]struct{})
	for _, idx := range t.Indexes {
		nidx, modified := idx.Normalize()
		if modified {
			clone = true
		}

		// if Not defined CONSTRAINT symbol, then resolve
		// implicitly created INDEX too difficult.
		// (lestrrat) this comment is confusing. Please add
		// actual examples somewhere
		if nidx.IsForeignKey() && nidx.HasSymbol() {
			// There's a chance the user has already explicitly declared the
			// index for this constraint. Only add this implicit index if we
			// haven't seen it before
			if _, ok := seen[nidx.Symbol()]; !ok {
				clone = true
				// add implicitly created INDEX
				index := NewIndex(IndexKindNormal, t.ID())
				index.SetName(nidx.Symbol())
				if nidx.IsBtree() {
					index.SetType(IndexTypeBtree)
				} else if nidx.IsHash() {
					index.SetType(IndexTypeHash)
				}
				columns := []IndexColumn{}
				for c := range nidx.Columns() {
					columns = append(columns, c)
				}
				index.AddColumns(columns...)
				indexes = append(indexes, index)
			}
		}
		indexes = append(indexes, nidx)
		seen[nidx.Name()] = struct{}{}
	}

	if !clone {
		return t, false
	}

	tbl := NewTable(t.Name)
	tbl.IfNotExists = t.IfNotExists
	tbl.Temporary = t.Temporary
	tbl.Indexes = append(additionalIndexes, indexes...)
	tbl.Columns = columns
	tbl.Options = make([]TableOption, len(t.Options))
	copy(tbl.Options, t.Options)

	return tbl, true
}

// NewTableOption creates a new table option with the given name, value, and a flag indicating if quoting is necessary
func NewTableOption(k, v string, q bool) TableOption {
	return &tableopt{
		key:        k,
		value:      v,
		needQuotes: q,
	}
}

func (t *tableopt) ID() string       { return "tableopt#" + t.key }
func (t *tableopt) Key() string      { return t.key }
func (t *tableopt) Value() string    { return t.value }
func (t *tableopt) NeedQuotes() bool { return t.needQuotes }
