package diff

import (
	"bytes"
	"fmt"
	"slices"
	"sort"

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
	}
	return nil
}

// createViews creates the new views, and replaces the changed views by CREATE OR REPLACE VIEW.
// They are created after the tables are changed, because views refer to tables,
// and a new view may have the same name as a dropped table.
// They are created in the order of their dependencies, because a view can refer to other views.
func (ctx *diffCtx) createViews() error {
	var buf bytes.Buffer
	for _, view := range sortViews(ctx.toViews, ctx.toViewOrder) {
		from, exists := ctx.fromViews[view.ID()]
		if exists && equalView(from, view) {
			continue
		}

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
		seen := newSet()
		for _, ident := range exprIdents(views[id].Select) {
			ref := model.NewView(model.Ident(ident)).ID()
			if ref == id {
				continue
			}
			if _, ok := views[ref]; !ok {
				continue
			}
			if _, ok := seen[ref]; ok {
				continue
			}
			seen.Add(ref)
			refs[id] = append(refs[id], ref)
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
