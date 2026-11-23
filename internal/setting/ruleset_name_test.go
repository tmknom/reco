package setting

import (
	"strings"
	"testing"
)

//trace:checks FR-001
func TestParseRulesetName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "plain name is accepted", input: "protect-default-branch"},
		{name: "upper case letters keep the declared spelling", input: "Protect"},
		{name: "inner space, slash, and colon are accepted", input: "protect main/release: v1"},
		{name: "lowest and highest printable ASCII inside the name are accepted", input: "a b~c"},
		{name: "single dot is accepted", input: "."},
		{name: "double dot is accepted", input: ".."},
		{name: "256 characters are accepted", input: strings.Repeat("a", 256)},
		{name: "empty name is rejected", input: "", wantErr: "must not be empty"},
		{name: "non-ASCII letter is rejected", input: "protéct", wantErr: "must consist of printable ASCII characters"},
		{name: "tab is rejected", input: "pro\ttect", wantErr: "must consist of printable ASCII characters"},
		{name: "DEL is rejected", input: "pro\x7ftect", wantErr: "must consist of printable ASCII characters"},
		{name: "leading space is rejected", input: " protect", wantErr: "must not have leading or trailing spaces"},
		{name: "trailing space is rejected", input: "protect ", wantErr: "must not have leading or trailing spaces"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRulesetName(tc.input)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("ParseRulesetName(%q) error = %v, want %q", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRulesetName(%q) error = %v", tc.input, err)
			}
			if got.String() != tc.input {
				t.Errorf("String() = %q, want the declared spelling %q", got.String(), tc.input)
			}
		})
	}
}

//trace:checks FR-001
func TestRulesetName_EqualFold(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "names differing only in ASCII letter case are the same", a: "Protect", b: "protect", want: true},
		{name: "a name and its prefix differ", a: "protect", b: "protect-", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustRulesetName(t, tc.a).EqualFold(mustRulesetName(t, tc.b)); got != tc.want {
				t.Errorf("EqualFold() = %t, want %t", got, tc.want)
			}
		})
	}
}

// TestRulesetName_Compare は、宣言の名前を宣言の側、引数を観測の側として突き合わせることを確かめる。
func TestRulesetName_Compare(t *testing.T) {
	cases := []struct {
		name     string
		observed string
		want     NameComparison
	}{
		{name: "different ASCII letter case is the same", observed: "PROTECT", want: NameSame},
		{name: "observed name that is not comparable is undetermined", observed: "protéct", want: NameUndetermined},
	}
	declared := mustRulesetName(t, "protect")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := declared.Compare(tc.observed); got != tc.want {
				t.Errorf("Compare(%q) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}
