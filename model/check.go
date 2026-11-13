package model

// CheckConstraint describes a CHECK constraint of a table, e.g. `CONSTRAINT name CHECK (a > 0) NOT ENFORCED`.
// The constraints in column definitions, e.g. `a INT CHECK (a > 0)`, are also kept in the table,
// because MySQL treats them as table constraints.
// https://dev.mysql.com/doc/refman/26.10/en/create-table-check-constraints.html
type CheckConstraint struct {
	// Name is the name of the constraint.
	// It is not set if the name is not specified, and the server generates it, e.g. `t_chk_1`.
	Name MaybeIdent

	// Expr is the expression of the constraint.
	// It is kept as it is written, except for redundant outermost parentheses
	// and the markers of executable comments.
	Expr string

	// NotEnforced is true if the constraint is created but not enforced.
	NotEnforced bool
}

// NewCheckConstraint creates a new CHECK constraint with the given expression.
func NewCheckConstraint(expr string) *CheckConstraint {
	return &CheckConstraint{
		Expr: expr,
	}
}
