package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestWriteEnvironments(t *testing.T) {
	waitTimer := func(minutes int) *int { return &minutes }
	unprotected := setting.EnvironmentProtection{CanAdminsBypass: true}
	allMode := setting.DeploymentBranchPolicyAll
	// 変数の行の規則は TestWriteEnvironmentVariables が確かめるため、ここでは一つもない Environment にする
	noVariables := &github.Variables{Existing: []github.Variable{}}
	cases := []struct {
		name         string
		environments github.Environments
		want         string
	}{
		{
			name:         "no environments shows none as a comment on the key",
			environments: github.Environments{Existing: []github.Environment{}},
			want:         "    # environments: none\n",
		},
		{
			name: "environments are shown in the given order without sorting",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "staging", DeploymentBranchPolicyMode: allMode, Protection: unprotected, Variables: noVariables},
				{Name: "production", DeploymentBranchPolicyMode: allMode, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"staging\"\n        deployment_branch_policy:\n          mode: all\n        required_reviewers: []\n        # variables: none\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n        required_reviewers: []\n        # variables: none\n",
		},
		{
			name: "protected_branches shows only the mode",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "staging", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicyProtectedBranches, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"staging\"\n        deployment_branch_policy:\n          mode: protected_branches\n        required_reviewers: []\n        # variables: none\n",
		},
		{
			name: "selected shows each pattern as the type and the quoted name",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected,
					DeploymentPatterns: &github.DeploymentPatterns{Existing: []setting.DeploymentPattern{
						{Type: setting.DeploymentPatternBranch, Name: "main"},
						{Type: setting.DeploymentPatternTag, Name: "v*"},
					}},
					Protection: unprotected,
					Variables:  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns:\n" +
				"            - type: branch\n              name: \"main\"\n" +
				"            - type: tag\n              name: \"v*\"\n        required_reviewers: []\n        # variables: none\n",
		},
		{
			name: "selected without patterns shows the empty sequence",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "production", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected, DeploymentPatterns: &github.DeploymentPatterns{}, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns: []\n        required_reviewers: []\n        # variables: none\n",
		},
		{
			name: "wait_timer and can_admins_bypass are not shown",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: allMode,
					Protection:                 setting.EnvironmentProtection{WaitTimer: waitTimer(30), CanAdminsBypass: false},
					Variables:                  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n        required_reviewers: []\n        # variables: none\n",
		},
		{
			name: "reviewers are shown in the given order with the login as a trailing comment",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: allMode,
					Protection: setting.EnvironmentProtection{RequiredReviewers: &setting.RequiredReviewers{
						Reviewers: []setting.Reviewer{
							{Type: setting.ReviewerUser, ID: 1234567},
							{Type: setting.ReviewerUser, ID: 583231},
						},
						Logins: map[setting.Reviewer]string{
							{Type: setting.ReviewerUser, ID: 583231}:  "octocat",
							{Type: setting.ReviewerUser, ID: 1234567}: "hubot",
						},
					}},
					Variables: noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n" +
				"        required_reviewers:\n" +
				"          - type: User\n            id: 1234567 # hubot\n" +
				"          - type: User\n            id: 583231 # octocat\n" +
				"        # variables: none\n",
		},
		{
			name: "a user without an observed login has no comment and a team has none",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: allMode,
					Protection: setting.EnvironmentProtection{RequiredReviewers: &setting.RequiredReviewers{
						Reviewers: []setting.Reviewer{
							{Type: setting.ReviewerUser, ID: 7},
							{Type: setting.ReviewerTeam, ID: 42},
						},
						// チームの ID と同じ値の利用者がいても、チームには付けない
						Logins: map[setting.Reviewer]string{{Type: setting.ReviewerUser, ID: 42}: "someone"},
					}},
					Variables: noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n" +
				"        required_reviewers:\n" +
				"          - type: User\n            id: 7\n" +
				"          - type: Team\n            id: 42\n" +
				"        # variables: none\n",
		},
		{
			name: "a protection rule without reviewers is shown as a comment instead of an empty sequence",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: allMode,
					Protection:                 setting.EnvironmentProtection{RequiredReviewers: &setting.RequiredReviewers{PreventSelfReview: true}},
					Variables:                  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n" +
				"        # required_reviewers: no reviewers (a protection rule without reviewers remains)\n" +
				"        # variables: none\n",
		},
		{
			name: "a name with a newline is escaped onto one line",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "a\nb", DeploymentBranchPolicyMode: allMode, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"a\\nb\"\n        deployment_branch_policy:\n          mode: all\n        required_reviewers: []\n        # variables: none\n",
		},
		{
			name: "a pattern name with a newline is escaped onto one line",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected,
					DeploymentPatterns:         &github.DeploymentPatterns{Existing: []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "a\nb"}}},
					Protection:                 unprotected,
					Variables:                  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns:\n" +
				"            - type: branch\n              name: \"a\\nb\"\n        required_reviewers: []\n        # variables: none\n",
		},
		{
			name: "variables follow the reviewers of their environment and precede the next environment",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected,
					DeploymentPatterns:         &github.DeploymentPatterns{Existing: []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "main"}}},
					Protection:                 unprotected,
					Variables:                  &github.Variables{Existing: []github.Variable{{Name: "REGION", Value: "eu"}}},
				},
				{Name: "staging", DeploymentBranchPolicyMode: allMode, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns:\n" +
				"            - type: branch\n              name: \"main\"\n" +
				"        required_reviewers: []\n" +
				"        variables:\n          - name: \"REGION\"\n            value: \"eu\"\n" +
				"      - name: \"staging\"\n        deployment_branch_policy:\n          mode: all\n        required_reviewers: []\n        # variables: none\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeEnvironments(&b, tc.environments)
			if b.String() != tc.want {
				t.Errorf("writeEnvironments() wrote %q, want %q", b.String(), tc.want)
			}
		})
	}
}
