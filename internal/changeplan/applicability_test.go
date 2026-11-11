package changeplan

import "testing"

//trace:checks FR-006
func TestJudgeApplicability(t *testing.T) {
	cases := []struct {
		name string
		in   ReconciliationResult
		want Applicability
	}{
		{name: "mismatch is applicable", in: Mismatch, want: Applicable},
		{name: "match is not judged", in: Match, want: ApplicabilityNotJudged},
		{name: "not comparable is not applicable", in: NotComparable, want: NotApplicable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := judgeApplicability(tc.in); got != tc.want {
				t.Errorf("judgeApplicability(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
