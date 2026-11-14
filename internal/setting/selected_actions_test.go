package setting

import "testing"

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
