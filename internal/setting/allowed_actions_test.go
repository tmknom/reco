package setting

import "testing"

func TestParseAllowedActionsPolicy(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   AllowedActionsPolicy
		wantOK bool
	}{
		{name: "all is accepted", input: "all", want: AllowedActionsAll, wantOK: true},
		{name: "local_only is accepted", input: "local_only", want: AllowedActionsLocalOnly, wantOK: true},
		{name: "selected is accepted", input: "selected", want: AllowedActionsSelected, wantOK: true},
		{name: "undefined value is rejected", input: "none"},
		{name: "different case is rejected", input: "All"},
		{name: "surrounding space is rejected", input: " all"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseAllowedActionsPolicy(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseAllowedActionsPolicy(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
