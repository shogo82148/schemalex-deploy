package diff

import (
	"bytes"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/shogo82148/schemalex-deploy/format"
	"github.com/shogo82148/schemalex-deploy/model"
)

// viewsByID returns the IDs of the views in stmts, and the views by their IDs.
// If a view is defined more than once, the last one is used, as executing the statements in order does.
// e.g. mysqldump defines a placeholder view first, and replaces it with the actual definition later.
// The IDs are also returned in the order of their definitions.
func viewsByID(stmts model.Stmts) (set, map[string]*model.View, []string) {
	ids := newSet()
	views := make(map[string]*model.View)
	var order []string
	for _, stmt := range stmts {
		if view, ok := stmt.(*model.View); ok {
			id := view.ID()
			if _, ok := ids[id]; ok {
				order = slices.DeleteFunc(order, func(s string) bool { return s == id })
			}
			ids.Add(id)
			views[id] = view
			order = append(order, id)
		}
	}
	return ids, views, order
}

// dropViews drops the views that do not exist in the new schema.
// They are dropped before the tables are changed, because a new table may have the same name as a dropped view.
func (ctx *diffCtx) dropViews() error {
	ids := ctx.fromViewSet.Difference(ctx.toViewSet)
	for _, id := range ids.ToSlice() {
		view, ok := ctx.fromViews[id]
		if !ok {
			return fmt.Errorf("failed to lookup view: %q", id)
		}
		ctx.append("DROP VIEW " + view.Name.Quoted())
		ctx.markChanged(view.Name)
	}
	return nil
}

// createViews creates the new views, and replaces the changed views by CREATE OR REPLACE VIEW.
// They are created after the tables are changed, because views refer to tables,
// and a new view may have the same name as a dropped table.
// They are created in the order of their dependencies, because a view can refer to other views.
//
// The views that refer to the created, dropped or changed tables and views are also replaced,
// even if their definitions are not changed,
// because MySQL expands `SELECT *` into the columns when the view is created.
// e.g. a view `SELECT * FROM t` does not have a column added to `t` later,
// and it becomes invalid if a column is dropped from `t`.
// The replaced views are also treated as changed, so the views that refer to them are replaced too.
// The references are guessed from the select statements, see viewRefs,
// so a view may be replaced unnecessarily. It does not touch any data.
func (ctx *diffCtx) createViews() error {
	// find the views to create or replace.
	// it is repeated until no more views are found,
	// because a view may refer to a view that is found later, e.g. in a cycle of the guessed references.
	targets := make(map[string]bool, len(ctx.toViews))
	for id, view := range ctx.toViews {
		from, exists := ctx.fromViews[id]
		if !exists || !equalView(from, view) {
			targets[id] = true
			ctx.markChanged(view.Name)
		}
	}
	for updated := true; updated; {
		updated = false
		for id, view := range ctx.toViews {
			if !targets[id] && ctx.refersToChanged(view) {
				targets[id] = true
				ctx.markChanged(view.Name)
				updated = true
			}
		}
	}

	var buf bytes.Buffer
	for _, view := range sortViews(ctx.toViews, ctx.toViewOrder) {
		if !targets[view.ID()] {
			continue
		}
		v := *view
		if _, exists := ctx.fromViews[view.ID()]; exists {
			v.OrReplace = true
			v.IfNotExists = false
		}
		buf.Reset()
		if err := format.SQL(&buf, &v); err != nil {
			return fmt.Errorf("failed to format a statement: %w", err)
		}
		ctx.append(buf.String())
	}
	return nil
}

// refersToChanged reports whether the view refers to a table or a view that is created, dropped or changed.
func (ctx *diffCtx) refersToChanged(view *model.View) bool {
	for _, ref := range viewRefs(view) {
		if _, ok := ctx.changed[ref]; ok {
			return true
		}
	}
	return false
}

// viewRefs returns the lower-cased names of the tables and views that the view may refer to,
// excluding the view itself.
func viewRefs(view *model.View) []string {
	self := strings.ToLower(string(view.Name))
	seen := newSet()
	var refs []string
	for _, name := range tableRefs(view.Select) {
		ref := strings.ToLower(name)
		if ref == self {
			continue
		}
		if _, ok := seen[ref]; ok {
			continue
		}
		seen.Add(ref)
		refs = append(refs, ref)
	}
	return refs
}

// tableRefs guesses the names of the tables and views that the select statement refers to.
// They are the identifiers that follow FROM and JOIN, and the commas in the FROM clauses.
// The schema names are removed, e.g. `t` for `db.t`.
// The names of common table expressions are excluded, e.g. `cte` for `WITH cte AS (...) SELECT * FROM cte`.
// It does not parse the statement, so the result may be inaccurate.
func tableRefs(stmt string) []string {
	tokens := sqlTokens(stmt)
	keyword := func(i int, words ...string) bool {
		if i < 0 || i >= len(tokens) || tokens[i].kind != sqlTokenWord {
			return false
		}
		for _, w := range words {
			if strings.EqualFold(tokens[i].text, w) {
				return true
			}
		}
		return false
	}
	symbol := func(i int, sym string) bool {
		return i >= 0 && i < len(tokens) && tokens[i].kind == sqlTokenSymbol && tokens[i].text == sym
	}
	ident := func(i int) bool {
		return i >= 0 && i < len(tokens) && (tokens[i].kind == sqlTokenWord || tokens[i].kind == sqlTokenQuotedIdent)
	}

	ctes := newSet()
	var refs []string
	inFrom := []bool{false} // whether it is in a FROM clause, for each depth of the parentheses
	expectTable := false
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]

		// the names of common table expressions:
		// `WITH [RECURSIVE] name [(columns)] AS (...)` and `..., name [(columns)] AS (...)`.
		if ident(i) && (keyword(i-1, "WITH") || keyword(i-1, "RECURSIVE") && keyword(i-2, "WITH") ||
			symbol(i-1, ",") && symbol(i-2, ")") && keyword(i+1, "AS") && symbol(i+2, "(")) {
			if !keyword(i, "RECURSIVE", "ROLLUP") {
				ctes.Add(strings.ToLower(t.text))
				expectTable = false
				continue
			}
		}

		switch {
		case keyword(i, "FROM", "JOIN", "STRAIGHT_JOIN"):
			inFrom[len(inFrom)-1] = true
			expectTable = true
		case keyword(i, "SELECT", "WHERE", "GROUP", "HAVING", "WINDOW", "ORDER", "LIMIT",
			"UNION", "INTERSECT", "EXCEPT", "FOR", "INTO"):
			// ON and USING do not end the FROM clause, e.g. `FROM t JOIN u USING (id), v`.
			inFrom[len(inFrom)-1] = false
			expectTable = false
		case keyword(i, "LATERAL"):
			// `JOIN LATERAL (...)`
		case symbol(i, "("):
			// a parenthesized table reference, e.g. `FROM (t JOIN u)`, is in the FROM clause.
			// a derived table, e.g. `FROM (SELECT ...)`, starts a new clause at SELECT.
			inFrom = append(inFrom, expectTable)
		case symbol(i, ")"):
			if len(inFrom) > 1 {
				inFrom = inFrom[:len(inFrom)-1]
			}
			expectTable = false
		case symbol(i, ","):
			expectTable = inFrom[len(inFrom)-1]
		case ident(i) && expectTable:
			name := t.text
			if symbol(i+1, ".") && ident(i+2) {
				// schema_name.table_name
				name = tokens[i+2].text
				i += 2
			}
			refs = append(refs, name)
			expectTable = false
		default:
			expectTable = false
		}
	}

	ret := refs[:0]
	for _, ref := range refs {
		if _, ok := ctes[strings.ToLower(ref)]; !ok {
			ret = append(ret, ref)
		}
	}
	return ret
}

type sqlTokenKind int

const (
	sqlTokenWord        sqlTokenKind = iota // an unquoted identifier, a keyword or a number
	sqlTokenQuotedIdent                     // a quoted identifier, e.g. `a`
	sqlTokenString                          // a string literal
	sqlTokenSymbol                          // the other characters
)

type sqlToken struct {
	kind sqlTokenKind
	text string // the unquoted identifier, or the symbol
}

// sqlTokens splits stmt into the tokens, skipping the white spaces and the comments.
// The comments are recognized as exprIdents does.
func sqlTokens(stmt string) []sqlToken {
	var tokens []sqlToken
	for i := 0; i < len(stmt); {
		switch c := stmt[i]; {
		case strings.HasPrefix(stmt[i:], "/*"):
			// a block comment. the markers of executable comments are already removed from stmt.
			end := strings.Index(stmt[i+2:], "*/")
			if end < 0 {
				return tokens
			}
			i += 2 + end + 2
		case c == '#' || strings.HasPrefix(stmt[i:], "-- ") || strings.HasPrefix(stmt[i:], "--\t") ||
			strings.HasPrefix(stmt[i:], "--\n") || strings.HasPrefix(stmt[i:], "--\r"):
			// a line comment.
			end := strings.IndexByte(stmt[i:], '\n')
			if end < 0 {
				return tokens
			}
			i += end + 1
		case c == '\'' || c == '"':
			tokens = append(tokens, sqlToken{kind: sqlTokenString})
			i = skipQuoted(stmt, i)
		case c == '`':
			end := skipQuoted(stmt, i)
			bodyEnd := end
			if end-1 > i && stmt[end-1] == '`' {
				bodyEnd = end - 1 // the closing backquote
			}
			tokens = append(tokens, sqlToken{kind: sqlTokenQuotedIdent, text: strings.ReplaceAll(stmt[i+1:bodyEnd], "``", "`")})
			i = end
		case isIdentChar(c):
			start := i
			for i < len(stmt) && isIdentChar(stmt[i]) {
				i++
			}
			tokens = append(tokens, sqlToken{kind: sqlTokenWord, text: stmt[start:i]})
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		default:
			tokens = append(tokens, sqlToken{kind: sqlTokenSymbol, text: stmt[i : i+1]})
			i++
		}
	}
	return tokens
}

// equalView reports whether the views a and b have the same definition.
// The omitted clauses are the same as their default values, except for DEFINER.
// DEFINER is compared only if both views have it, because its default value is the user who deploys the schema.
// The select statements are compared as written.
func equalView(a, b *model.View) bool {
	if viewAlgorithm(a) != viewAlgorithm(b) {
		return false
	}
	if a.Definer.Valid && b.Definer.Valid && a.Definer.Value != b.Definer.Value {
		return false
	}
	if viewSQLSecurity(a) != viewSQLSecurity(b) {
		return false
	}
	if !slices.Equal(a.Columns, b.Columns) {
		return false
	}
	if a.Select != b.Select {
		return false
	}
	return a.CheckOption == b.CheckOption
}

func viewAlgorithm(v *model.View) string {
	if v.Algorithm == "" {
		return model.ViewAlgorithmUndefined
	}
	return v.Algorithm
}

func viewSQLSecurity(v *model.View) string {
	if v.SQLSecurity == "" {
		return model.ViewSQLSecurityDefiner
	}
	return v.SQLSecurity
}

// sortViews sorts the views so that each view comes after the views it refers to.
// order is the IDs of the views in the order of their definitions.
//
// The references are guessed from the select statements, see viewRefs,
// so they may make a cycle, e.g. a common table expression has the same name as a view.
// The views in a cycle are sorted in the order of their definitions,
// because the schema is usually written in an order that it can be executed.
func sortViews(views map[string]*model.View, order []string) []*model.View {
	refs := make(map[string][]string, len(views))
	for _, id := range order {
		for _, name := range viewRefs(views[id]) {
			ref := model.NewView(model.Ident(name)).ID()
			if _, ok := views[ref]; ok {
				refs[id] = append(refs[id], ref)
			}
		}
	}

	// find the strongly connected components by Tarjan's algorithm.
	// a component is found after all the components it refers to,
	// so the components are found in the order that they can be created.
	position := make(map[string]int, len(order))
	for i, id := range order {
		position[id] = i
	}
	index := make(map[string]int, len(order))
	lowlink := make(map[string]int, len(order))
	onStack := make(map[string]bool, len(order))
	var stack []string
	result := make([]*model.View, 0, len(order))
	var connect func(id string)
	connect = func(id string) {
		index[id] = len(index)
		lowlink[id] = index[id]
		stack = append(stack, id)
		onStack[id] = true

		for _, ref := range refs[id] {
			if _, visited := index[ref]; !visited {
				connect(ref)
				lowlink[id] = min(lowlink[id], lowlink[ref])
			} else if onStack[ref] {
				lowlink[id] = min(lowlink[id], index[ref])
			}
		}

		if lowlink[id] == index[id] {
			// id is the root of a component.
			i := len(stack) - 1
			for stack[i] != id {
				i--
			}
			component := slices.Clone(stack[i:])
			stack = stack[:i]
			for _, member := range component {
				onStack[member] = false
			}
			sort.Slice(component, func(i, j int) bool {
				return position[component[i]] < position[component[j]]
			})
			for _, member := range component {
				result = append(result, views[member])
			}
		}
	}
	for _, id := range order {
		if _, visited := index[id]; !visited {
			connect(id)
		}
	}
	return result
}
