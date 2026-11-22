package setting

import (
	"strings"
	"testing"
)

//trace:checks FR-001
func TestParseVariableName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "upper case letters and underscore are accepted", input: "GO_VERSION"},
		{name: "leading underscore is accepted", input: "_X"},
		{name: "digit after the first character is accepted", input: "X1"},
		{name: "GITHUB without the underscore is accepted", input: "GITHUBX"},
		{name: "GITHUB_ after the first character is accepted", input: "_GITHUB_X"},
		{name: "length is not limited", input: strings.Repeat("A", 301)},
		{name: "empty name is rejected", input: "", wantErr: "must not be empty"},
		{name: "hyphen is rejected", input: "A-B", wantErr: "must consist of ASCII letters, digits, and underscores"},
		{name: "non-ASCII letter is rejected", input: "CAFÉ", wantErr: "must consist of ASCII letters, digits, and underscores"},
		{name: "leading digit is rejected", input: "1X", wantErr: "must not start with a digit"},
		{name: "GITHUB_ prefix is rejected", input: "GITHUB_X", wantErr: "must not start with GITHUB_"},
		{name: "lower case GITHUB_ prefix is rejected as the prefix", input: "github_x", wantErr: "must not start with GITHUB_"},
		{name: "lower case letters are rejected with the upper case spelling", input: "go_Version", wantErr: `must be upper case; write "GO_VERSION"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseVariableName(tc.input)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("ParseVariableName(%q) error = %v, want %q", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseVariableName(%q) error = %v", tc.input, err)
			}
			if got.String() != tc.input {
				t.Errorf("String() = %q, want the declared spelling %q", got.String(), tc.input)
			}
		})
	}
}

//trace:checks FR-005
func TestVariableName_Matches(t *testing.T) {
	declared := mustVariableName(t, "GO_VERSION")
	cases := []struct {
		name     string
		observed string
		want     bool
	}{
		{name: "same spelling matches", observed: "GO_VERSION", want: true},
		{name: "lower case spelling matches", observed: "go_version", want: true},
		{name: "name with the declared name as its prefix does not match", observed: "GO_VERSION_2", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := declared.Matches(tc.observed); got != tc.want {
				t.Errorf("Matches(%q) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}

func mustVariableName(t *testing.T, s string) VariableName {
	t.Helper()
	name, err := ParseVariableName(s)
	if err != nil {
		t.Fatalf("ParseVariableName(%q) error = %v", s, err)
	}
	return name
}
