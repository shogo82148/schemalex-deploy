package model

import "strings"

// The algorithms of views, see View.Algorithm.
const (
	ViewAlgorithmUndefined = "UNDEFINED"
	ViewAlgorithmMerge     = "MERGE"
	ViewAlgorithmTemptable = "TEMPTABLE"
)

// The SQL security characteristics of views, see View.SQLSecurity.
const (
	ViewSQLSecurityDefiner = "DEFINER"
	ViewSQLSecurityInvoker = "INVOKER"
)

// The check options of views, see View.CheckOption.
const (
	ViewCheckOptionCascaded = "CASCADED"
	ViewCheckOptionLocal    = "LOCAL"
)

// View describes a view model.
// https://dev.mysql.com/doc/refman/26.10/en/create-view.html
type View struct {
	Name Ident

	// OrReplace is true if the view is defined by `CREATE OR REPLACE VIEW`.
	OrReplace bool

	// IfNotExists is true if the view is defined by `CREATE VIEW IF NOT EXISTS`.
	// It is supported by MariaDB.
	IfNotExists bool

	// Algorithm is the ALGORITHM clause, i.e. ViewAlgorithmUndefined, ViewAlgorithmMerge or ViewAlgorithmTemptable.
	// It is empty if it is not specified, which means ViewAlgorithmUndefined.
	Algorithm string

	// Definer is the DEFINER clause, e.g. `'root'@'localhost'` and `CURRENT_USER`.
	// It is kept as it is written, except for the markers of executable comments.
	Definer MaybeString

	// SQLSecurity is the SQL SECURITY clause, i.e. ViewSQLSecurityDefiner or ViewSQLSecurityInvoker.
	// It is empty if it is not specified, which means ViewSQLSecurityDefiner.
	SQLSecurity string

	// Columns are the names of the columns of the view, e.g. `CREATE VIEW v (a, b) AS ...`.
	// It is nil if they are not specified.
	Columns []Ident

	// Select is the select statement of the view, e.g. `SELECT a, b FROM t`.
	// It is kept as it is written, except for the markers of executable comments.
	Select string

	// CheckOption is the WITH CHECK OPTION clause, i.e. ViewCheckOptionCascaded or ViewCheckOptionLocal.
	// It is empty if the view does not have the clause.
	// `WITH CHECK OPTION` is the same as `WITH CASCADED CHECK OPTION`.
	CheckOption string
}

// NewView creates a new view with the given name.
func NewView(name Ident) *View {
	return &View{
		Name: name,
	}
}

func (v *View) ID() string {
	return "view#" + strings.ToLower(string(v.Name))
}
