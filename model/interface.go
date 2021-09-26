package model

// Stmt is the interface to define a statement
type Stmt interface {
	ID() string
}

// Stmts describes a list of statements
type Stmts []Stmt

type MaybeString struct {
	Valid bool
	Value string
}
