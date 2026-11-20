package setting

import "testing"

//trace:checks FR-004
func TestParseAccessLevel(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   AccessLevel
		wantOK bool
	}{
		{name: "none is accepted", input: "none", want: AccessNone, wantOK: true},
		{name: "user is accepted", input: "user", want: AccessUser, wantOK: true},
		{name: "organization is accepted", input: "organization", want: AccessOrganization, wantOK: true},
		{name: "undefined value is rejected", input: "enterprise"},
		{name: "different case is rejected", input: "None"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseAccessLevel(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseAccessLevel(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
