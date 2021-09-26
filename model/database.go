package model

import "strings"

// Database represents a database definition
type Database struct {
	Name        Ident
	IfNotExists bool
}

// NewDatabase creates a new database mode with th given name
func NewDatabase(name string) *Database {
	return &Database{
		Name: Ident(name),
	}
}

func (d *Database) ID() string {
	return "database#" + strings.ToLower(string(d.Name))
}
