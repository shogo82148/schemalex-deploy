package util

import "testing"

func TestUtil(t *testing.T) {
	if want, got := "`hoge`", Backquote("hoge"); want != got {
		t.Errorf("want %q, got %q", want, got)
	}

	if want, got := "`ho``ge`", Backquote("ho`ge"); want != got {
		t.Errorf("want %q, got %q", want, got)
	}
}
