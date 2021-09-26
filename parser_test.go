package schemalex_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/shogo82148/schemalex-deploy"
)

func TestParseError1(t *testing.T) {
	const src = "CREATE TABLE foo (id int PRIMARY KEY);\nCREATE TABLE bar"
	p := schemalex.New()
	_, err := p.ParseString(src)
	if err == nil {
		t.Fatal("parse should fail")
	}

	expected := "parse error: expected LPAREN at line 2 column 16 (at EOF)\n" +
		"    \"CREATE TABLE bar\" <---- AROUND HERE"
	if diff := cmp.Diff(err.Error(), expected); diff != "" {
		t.Errorf("unexpected error message: (-want/+got):\n%s", diff)
	}
}

func TestParseError2(t *testing.T) {
	const src = "CREATE TABLE foo (id int PRIMARY KEY);\nCREATE TABLE bar (id int PRIMARY KEY baz TEXT)"
	p := schemalex.New()
	_, err := p.ParseString(src)
	if err == nil {
		t.Fatal("parse should fail")
	}

	expected := "parse error: unexpected column option IDENT at line 2 column 37\n" +
		"    \"CREATE TABLE bar (id int PRIMARY KEY \" <---- AROUND HERE"
	if diff := cmp.Diff(err.Error(), expected); diff != "" {
		t.Errorf("unexpected error message: (-want/+got):\n%s", diff)
	}
}
