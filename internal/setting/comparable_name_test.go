package setting

import "testing"

//trace:checks FR-004 FR-005
func TestCompareName(t *testing.T) {
	cases := []struct {
		name     string
		observed string
		want     NameComparison
	}{
		{name: "same spelling is the same", observed: "staging", want: NameSame},
		{name: "different ASCII letter case is the same", observed: "STAGING", want: NameSame},
		{name: "different comparable name is different", observed: "production", want: NameDifferent},
		{name: "comparable name of the same length is different", observed: "stagine", want: NameDifferent},
		{name: "accented letter is undetermined", observed: "stàging", want: NameUndetermined},
		{name: "long s that Unicode folds to s is undetermined", observed: "ſtaging", want: NameUndetermined},
		{name: "trailing space is undetermined", observed: "staging ", want: NameUndetermined},
		{name: "leading space is undetermined", observed: " staging", want: NameUndetermined},
		{name: "control character is undetermined", observed: "staging\x01", want: NameUndetermined},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := compareName("staging", tc.observed); got != tc.want {
				t.Errorf("compareName(%q, %q) = %v, want %v", "staging", tc.observed, got, tc.want)
			}
		})
	}
}
