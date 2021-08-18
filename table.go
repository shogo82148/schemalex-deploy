package schemalex

var _ Stmt = (*Table)(nil)

type MaybeString struct {
	Valid bool
	Value string
}

type Table struct {
	Name        string
	Temporary   bool
	IfNotExists bool
	LikeTable   MaybeString
	// Columns           []TableColumn
	// ColumnNameToIndex map[string]int
	// Indexes           []Index
	// Options           []TableOption
}

func (t *Table) ID() string {
	return "table#" + t.Name
}
