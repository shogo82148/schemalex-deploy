package model

// Stmt is the interface to define a statement
type Stmt interface {
	ID() string
}

// Stmts describes a list of statements
type Stmts []Stmt

// Lookup looks for a statement with the given ID
func (s Stmts) Lookup(id string) (Stmt, bool) {
	for _, stmt := range s {
		if stmt.ID() == id {
			return stmt, true
		}
	}
	return nil, false
}
