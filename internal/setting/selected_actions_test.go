package setting

import (
	"slices"
	"testing"
)

//trace:checks FR-004
func TestSelectedActionsCriteria_PatternsAllowedString(t *testing.T) {
	cases := []struct {
		name     string
		patterns []string
		want     string
	}{
		{name: "empty set is shown as brackets", patterns: nil, want: `[]`},
		{name: "patterns keep the given order", patterns: []string{"z/*", "a/*"}, want: `["z/*", "a/*"]`},
		{name: "double quote is escaped", patterns: []string{`a"b`}, want: `["a\"b"]`},
		{name: "backslash is escaped", patterns: []string{`a\b`}, want: `["a\\b"]`},
		{name: "newline is escaped instead of breaking the line", patterns: []string{"a\nb"}, want: `["a\nb"]`},
		{name: "printable non-ASCII is kept", patterns: []string{"日本"}, want: `["日本"]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := SelectedActionsCriteria{PatternsAllowed: tc.patterns}
			if got := c.PatternsAllowedString(); got != tc.want {
				t.Errorf("PatternsAllowedString() = %s, want %s", got, tc.want)
			}
		})
	}
}

//trace:checks FR-005
func TestSelectedActionsCriteria_equal(t *testing.T) {
	base := SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: []string{"a/*", "b/*"}}
	with := func(patterns ...string) SelectedActionsCriteria {
		c := base
		c.PatternsAllowed = patterns
		return c
	}
	cases := []struct {
		name  string
		a, b  SelectedActionsCriteria
		equal bool
	}{
		{name: "same items in the same order are equal", a: base, b: with("a/*", "b/*"), equal: true},
		{name: "patterns differing only in order are equal", a: base, b: with("b/*", "a/*"), equal: true},
		{name: "different github_owned_allowed differs", a: base, b: SelectedActionsCriteria{GitHubOwnedAllowed: false, VerifiedAllowed: true, PatternsAllowed: base.PatternsAllowed}, equal: false},
		{name: "different verified_allowed differs", a: base, b: SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: base.PatternsAllowed}, equal: false},
		{name: "extra pattern on this side differs", a: with("a/*", "b/*"), b: with("a/*"), equal: false},
		{name: "patterns differing only in duplicates are equal", a: with("a/*", "a/*"), b: with("a/*"), equal: true},
		{name: "nil and empty patterns are equal", a: with(), b: SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: []string{}}, equal: true},
		{name: "empty set and an empty string pattern differ", a: with(), b: with(""), equal: false},
		{name: "wildcard inclusion is not interpreted", a: with("a/*"), b: with("a/*", "a/b"), equal: false},
		{name: "patterns differing only in case differ", a: with("a/*"), b: with("A/*"), equal: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.equal(tc.b); got != tc.equal {
				t.Errorf("equal() = %v, want %v", got, tc.equal)
			}
		})
	}
}

func TestSelectedActionsCriteria_equalKeepsOrder(t *testing.T) {
	a := SelectedActionsCriteria{PatternsAllowed: []string{"z/*", "a/*"}}
	b := SelectedActionsCriteria{PatternsAllowed: []string{"y/*", "b/*"}}
	_ = a.equal(b)
	if want := []string{"z/*", "a/*"}; !slices.Equal(a.PatternsAllowed, want) {
		t.Errorf("receiver patterns = %q, want %q", a.PatternsAllowed, want)
	}
	if want := []string{"y/*", "b/*"}; !slices.Equal(b.PatternsAllowed, want) {
		t.Errorf("argument patterns = %q, want %q", b.PatternsAllowed, want)
	}
}
