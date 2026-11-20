package setting

import (
	"strings"
	"testing"
)

//trace:checks FR-001
func TestParseEnvironmentName(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "plain name is accepted", input: "production"},
		{name: "upper case letters keep the declared spelling", input: "Production"},
		{name: "inner space and slash are accepted", input: "my env/a"},
		{name: "lowest and highest printable ASCII inside the name are accepted", input: "a b~c"},
		{name: "dots other than . and .. are accepted", input: "a.."},
		{name: "255 characters are accepted", input: strings.Repeat("a", 255)},
		{name: "empty name is rejected", input: "", wantErr: "must not be empty"},
		{name: "256 characters are rejected", input: strings.Repeat("a", 256), wantErr: "must be at most 255 characters"},
		{name: "leading space is rejected", input: " production", wantErr: "must not have leading or trailing spaces"},
		{name: "trailing space is rejected", input: "production ", wantErr: "must not have leading or trailing spaces"},
		{name: "control character below space is rejected", input: "prod\x1fuction", wantErr: "must consist of printable ASCII characters"},
		{name: "tab is rejected", input: "prod\tuction", wantErr: "must consist of printable ASCII characters"},
		{name: "DEL is rejected", input: "prod\x7fuction", wantErr: "must consist of printable ASCII characters"},
		{name: "non-ASCII letter is rejected", input: "café", wantErr: "must consist of printable ASCII characters"},
		{name: "single dot is rejected", input: ".", wantErr: `must not be "." or ".."`},
		{name: "double dot is rejected", input: "..", wantErr: `must not be "." or ".."`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseEnvironmentName(tc.input)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("ParseEnvironmentName(%q) error = %v, want %q", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseEnvironmentName(%q) error = %v", tc.input, err)
			}
			if got.String() != tc.input {
				t.Errorf("String() = %q, want the declared spelling %q", got.String(), tc.input)
			}
		})
	}
}

//trace:checks FR-001
func TestEnvironmentName_EqualFold(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{name: "names differing only in ASCII letter case are the same", a: "Prod", b: "prod", want: true},
		{name: "a name and its prefix differ", a: "prod", b: "prod-", want: false},
		{name: "different names of the same length differ", a: "prod", b: "prot", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustEnvironmentName(t, tc.a).EqualFold(mustEnvironmentName(t, tc.b)); got != tc.want {
				t.Errorf("EqualFold() = %t, want %t", got, tc.want)
			}
		})
	}
}

//trace:checks FR-004
func TestEnvironmentName_Compare(t *testing.T) {
	cases := []struct {
		name     string
		observed string
		want     EnvironmentNameComparison
	}{
		{name: "same spelling is the same", observed: "staging", want: EnvironmentNameSame},
		{name: "different ASCII letter case is the same", observed: "STAGING", want: EnvironmentNameSame},
		{name: "different comparable name is different", observed: "production", want: EnvironmentNameDifferent},
		{name: "comparable name of the same length is different", observed: "stagine", want: EnvironmentNameDifferent},
		{name: "accented letter is undetermined", observed: "stàging", want: EnvironmentNameUndetermined},
		{name: "long s that Unicode folds to s is undetermined", observed: "ſtaging", want: EnvironmentNameUndetermined},
		{name: "trailing space is undetermined", observed: "staging ", want: EnvironmentNameUndetermined},
		{name: "leading space is undetermined", observed: " staging", want: EnvironmentNameUndetermined},
		{name: "control character is undetermined", observed: "staging\x01", want: EnvironmentNameUndetermined},
	}
	declared := mustEnvironmentName(t, "staging")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := declared.Compare(tc.observed); got != tc.want {
				t.Errorf("Compare(%q) = %v, want %v", tc.observed, got, tc.want)
			}
		})
	}
}

func mustEnvironmentName(t *testing.T, s string) EnvironmentName {
	t.Helper()
	name, err := ParseEnvironmentName(s)
	if err != nil {
		t.Fatal(err)
	}
	return name
}
