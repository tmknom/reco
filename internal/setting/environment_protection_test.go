package setting

import "testing"

//trace:checks FR-004
func TestParseReviewerType(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   ReviewerType
		wantOK bool
	}{
		{name: "User is accepted", input: "User", want: ReviewerUser, wantOK: true},
		{name: "Team is accepted", input: "Team", want: ReviewerTeam, wantOK: true},
		{name: "undefined value is rejected", input: "Bot"},
		{name: "different case is rejected", input: "user"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseReviewerType(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseReviewerType(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
