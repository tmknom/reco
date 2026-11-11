package changeplan

import "testing"

//trace:checks FR-006
func TestIsApplicable(t *testing.T) {
	cases := []struct {
		name string
		in   ReconciliationResult
		want bool
	}{
		{name: "mismatch is applicable", in: Mismatch, want: true},
		{name: "match is not judged", in: Match, want: false},
		{name: "not comparable is not applicable", in: NotComparable, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isApplicable(tc.in); got != tc.want {
				t.Errorf("isApplicable(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
