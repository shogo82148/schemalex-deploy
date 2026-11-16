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
// The references are guessed from the identifiers in the select statements, see viewRefs,
// so a view may be replaced unnecessarily. It does not touch any data.
// The replaced views are also treated as changed, so the views that refer to them are replaced too.
func (ctx *diffCtx) createViews() error {
	var buf bytes.Buffer
	for _, view := range sortViews(ctx.toViews, ctx.toViewOrder) {
		from, exists := ctx.fromViews[view.ID()]
		if exists && equalView(from, view) && !ctx.refersToChanged(view) {
			continue
		}
		ctx.markChanged(view.Name)

		v := *view
		if exists {
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

// viewRefs returns the lower-cased names that the view may refer to, excluding the view itself.
// They are the identifiers in the select statement, so they include the names of columns, aliases and functions.
func viewRefs(view *model.View) []string {
	self := strings.ToLower(string(view.Name))
	seen := newSet()
	var refs []string
	for _, ident := range exprIdents(view.Select) {
		ref := strings.ToLower(ident)
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
// The references are guessed from the identifiers in the select statements,
// so a column or an alias that has the same name as a view is also treated as a reference,
// and it may make a cycle of the references.
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
