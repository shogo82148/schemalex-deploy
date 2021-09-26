package model

import "strings"

// Database represents a database definition
type Database struct {
	Name        Ident
	IfNotExists bool
}

// NewDatabase creates a new database mode with th given name
func NewDatabase(name Ident) *Database {
	return &Database{
		Name: name,
	}
}

func (d *Database) ID() string {
	return "database#" + strings.ToLower(string(d.Name))
}
