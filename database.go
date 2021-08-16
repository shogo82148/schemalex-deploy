package schemalex

type Database struct {
	Name        string
	IfNotExists bool
}

func (d *Database) ID() string {
	return "database#" + d.Name
}
