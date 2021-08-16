package schemalex

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParser(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		output []Stmt
	}{
		{
			name:  "CREATE DATABASE foo",
			input: "CREATE DATABASE foo",
			output: []Stmt{
				&Database{
					Name: "foo",
				},
			},
		},
	}

	for _, tt := range tests {
		name := tt.name
		if name == "" {
			name = tt.input
		}
		t.Run(name, func(t *testing.T) {
			p := New()
			stmts, err := p.ParseString(tt.input)
			if err != nil {
				t.Error(err)
				return
			}
			if diff := cmp.Diff(tt.output, stmts); diff != "" {
				t.Errorf("statements mismatch: (-want/+got):\n%s", diff)
			}
		})
	}
}
