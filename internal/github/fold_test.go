package github

import (
	"strings"
	"testing"
	"unicode"
)

// TestSimpleFoldKey は、重複検査の鍵が一致することと、strings.EqualFold が同じとみなすことが、同じ判定になることを確かめる。
func TestSimpleFoldKey(t *testing.T) {
	t.Run("every rune shares its key with its simple fold and differs from its neighbor exactly when EqualFold does", func(t *testing.T) {
		for r := range rune(unicode.MaxRune) {
			a, folded, next := string(r), string(unicode.SimpleFold(r)), string(r+1)
			if simpleFoldKey(a) != simpleFoldKey(folded) {
				t.Fatalf("simpleFoldKey(%U) != simpleFoldKey(%U), want equal keys for a simple fold", r, unicode.SimpleFold(r))
			}
			if got, want := simpleFoldKey(a) == simpleFoldKey(next), strings.EqualFold(a, next); got != want {
				t.Fatalf("keys of %U and %U equal = %v, want %v as strings.EqualFold", r, r+1, got, want)
			}
		}
	})

	// 1 文字ずつの比較では現れない複数の文字からなる名前と、大文字化と小文字化の往復で同じになる文字を確かめる
	pairs := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "ASCII letter case is the same", a: "Prod", b: "prod", want: true},
		{name: "dotless i is different from i although upper then lower case maps both to i", a: "\u0131", b: "i", want: false},
		{name: "dotted capital I is different from i although lower case maps it to i", a: "\u0130", b: "i", want: false},
		{name: "trailing space is different", a: "prod", b: "prod ", want: false},
		{name: "precomposed and combining accents are different", a: "\u00e9", b: "e\u0301", want: false},
	}
	for _, p := range pairs {
		t.Run(p.name, func(t *testing.T) {
			if got := simpleFoldKey(p.a) == simpleFoldKey(p.b); got != p.want {
				t.Errorf("keys of %q and %q equal = %v, want %v", p.a, p.b, got, p.want)
			}
		})
	}
}
