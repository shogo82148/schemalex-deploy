package model

import "testing"

func TestIdent(t *testing.T) {
	if want, got := "`hoge`", Ident("hoge").Quoted(); want != got {
		t.Errorf("want %q, got %q", want, got)
	}

	if want, got := "`ho``ge`", Ident("ho`ge").Quoted(); want != got {
		t.Errorf("want %q, got %q", want, got)
	}
}
