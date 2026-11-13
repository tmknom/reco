package setting

import "testing"

func TestParsePermissionLevel(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   PermissionLevel
		wantOK bool
	}{
		{name: "read is accepted", input: "read", want: PermissionRead, wantOK: true},
		{name: "write is accepted", input: "write", want: PermissionWrite, wantOK: true},
		{name: "undefined value is rejected", input: "admin"},
		{name: "different case is rejected", input: "Read"},
		{name: "surrounding space is rejected", input: " read"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParsePermissionLevel(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParsePermissionLevel(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
