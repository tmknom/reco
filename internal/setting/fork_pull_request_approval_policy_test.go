package setting

import "testing"

//trace:checks FR-004
func TestParseForkPullRequestApprovalPolicy(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   ForkPullRequestApprovalPolicy
		wantOK bool
	}{
		{
			name:   "first_time_contributors_new_to_github is accepted",
			input:  "first_time_contributors_new_to_github",
			want:   ForkPullRequestApprovalFirstTimeContributorsNewToGitHub,
			wantOK: true,
		},
		{name: "first_time_contributors is accepted", input: "first_time_contributors", want: ForkPullRequestApprovalFirstTimeContributors, wantOK: true},
		{name: "all_external_contributors is accepted", input: "all_external_contributors", want: ForkPullRequestApprovalAllExternalContributors, wantOK: true},
		{name: "undefined value is rejected", input: "everyone"},
		{name: "different case is rejected", input: "FIRST_TIME_CONTRIBUTORS"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseForkPullRequestApprovalPolicy(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseForkPullRequestApprovalPolicy(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
