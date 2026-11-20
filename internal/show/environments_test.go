package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004
//trace:checks QR-002
func TestWriteEnvironments(t *testing.T) {
	waitTimer := func(minutes int) *int { return &minutes }
	unprotected := setting.EnvironmentProtection{CanAdminsBypass: true}
	allMode := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}
	cases := []struct {
		name         string
		environments github.Environments
		want         string
	}{
		{
			name:         "no environments shows none on the heading",
			environments: github.Environments{Existing: []github.Environment{}},
			want:         "Environments: none\n",
		},
		{
			name: "environments are shown in the given order without sorting",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "staging", DeploymentBranchPolicy: allMode, Protection: unprotected},
				{Name: "production", DeploymentBranchPolicy: allMode, Protection: unprotected},
			}},
			want: "Environments:\n" +
				"  - name: \"staging\"\n    deployment_branch_policy:\n      mode: all\n    wait_timer: none\n    required_reviewers: none\n    can_admins_bypass: true\n" +
				"  - name: \"production\"\n    deployment_branch_policy:\n      mode: all\n    wait_timer: none\n    required_reviewers: none\n    can_admins_bypass: true\n",
		},
		{
			name: "protected_branches shows only the mode",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "staging", DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}, Protection: unprotected},
			}},
			want: "Environments:\n" +
				"  - name: \"staging\"\n    deployment_branch_policy:\n      mode: protected_branches\n    wait_timer: none\n    required_reviewers: none\n    can_admins_bypass: true\n",
		},
		{
			name: "selected shows the patterns with the type and the quoted name",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name: "production",
					DeploymentBranchPolicy: setting.DeploymentBranchPolicy{
						Mode: setting.DeploymentBranchPolicySelected,
						Patterns: []setting.DeploymentPattern{
							{Type: setting.DeploymentPatternBranch, Name: "main"},
							{Type: setting.DeploymentPatternTag, Name: "v*"},
						},
					},
					Protection: unprotected,
				},
			}},
			want: "Environments:\n" +
				"  - name: \"production\"\n    deployment_branch_policy:\n      mode: selected\n      patterns: [branch \"main\", tag \"v*\"]\n    wait_timer: none\n    required_reviewers: none\n    can_admins_bypass: true\n",
		},
		{
			name: "protection rules are shown with the timer, the reviewers, and prevent_self_review",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                   "production",
					DeploymentBranchPolicy: allMode,
					Protection: setting.EnvironmentProtection{
						WaitTimer: waitTimer(30),
						RequiredReviewers: &setting.RequiredReviewers{
							PreventSelfReview: true,
							Reviewers: []setting.Reviewer{
								{Type: setting.ReviewerUser, ID: 583231},
								{Type: setting.ReviewerTeam, ID: 42},
							},
						},
					},
				},
			}},
			want: "Environments:\n" +
				"  - name: \"production\"\n    deployment_branch_policy:\n      mode: all\n    wait_timer: 30\n" +
				"    required_reviewers:\n      reviewers: [User 583231, Team 42]\n      prevent_self_review: true\n    can_admins_bypass: false\n",
		},
		{
			name: "a name with a newline is escaped onto one line",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "a\nb", DeploymentBranchPolicy: allMode, Protection: unprotected},
			}},
			want: "Environments:\n" +
				"  - name: \"a\\nb\"\n    deployment_branch_policy:\n      mode: all\n    wait_timer: none\n    required_reviewers: none\n    can_admins_bypass: true\n",
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
