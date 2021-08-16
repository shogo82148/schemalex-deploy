package schemalex

var _ Stmt = (*Database)(nil)

type Database struct {
	Name        string
	IfNotExists bool
}

func (d *Database) ID() string {
	return "database#" + d.Name
}
