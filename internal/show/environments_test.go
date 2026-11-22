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
	allMode := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}
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
				{Name: "staging", DeploymentBranchPolicy: allMode, Protection: unprotected, Variables: noVariables},
				{Name: "production", DeploymentBranchPolicy: allMode, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"staging\"\n        deployment_branch_policy:\n          mode: all\n        # variables: none\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n        # variables: none\n",
		},
		{
			name: "protected_branches shows only the mode",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "staging", DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"staging\"\n        deployment_branch_policy:\n          mode: protected_branches\n        # variables: none\n",
		},
		{
			name: "selected shows each pattern as the type and the quoted name",
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
					Variables:  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns:\n" +
				"            - type: branch\n              name: \"main\"\n" +
				"            - type: tag\n              name: \"v*\"\n        # variables: none\n",
		},
		{
			name: "selected without patterns shows the empty sequence",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "production", DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected}, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns: []\n        # variables: none\n",
		},
		{
			name: "protection rules are not shown",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                   "production",
					DeploymentBranchPolicy: allMode,
					Protection: setting.EnvironmentProtection{
						WaitTimer: waitTimer(30),
						RequiredReviewers: &setting.RequiredReviewers{
							PreventSelfReview: true,
							Reviewers:         []setting.Reviewer{{Type: setting.ReviewerUser, ID: 583231}},
						},
					},
					Variables: noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n        # variables: none\n",
		},
		{
			name: "a name with a newline is escaped onto one line",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "a\nb", DeploymentBranchPolicy: allMode, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"a\\nb\"\n        deployment_branch_policy:\n          mode: all\n        # variables: none\n",
		},
		{
			name: "a pattern name with a newline is escaped onto one line",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name: "production",
					DeploymentBranchPolicy: setting.DeploymentBranchPolicy{
						Mode:     setting.DeploymentBranchPolicySelected,
						Patterns: []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "a\nb"}},
					},
					Protection: unprotected,
					Variables:  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns:\n" +
				"            - type: branch\n              name: \"a\\nb\"\n        # variables: none\n",
		},
		{
			name: "variables follow the deployment patterns of their environment and precede the next environment",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name: "production",
					DeploymentBranchPolicy: setting.DeploymentBranchPolicy{
						Mode:     setting.DeploymentBranchPolicySelected,
						Patterns: []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "main"}},
					},
					Protection: unprotected,
					Variables:  &github.Variables{Existing: []github.Variable{{Name: "REGION", Value: "eu"}}},
				},
				{Name: "staging", DeploymentBranchPolicy: allMode, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns:\n" +
				"            - type: branch\n              name: \"main\"\n" +
				"        # reco does not accept the environment variables in the configuration file yet\n" +
				"        # variables:\n        #   - name: \"REGION\"\n        #     value: \"eu\"\n" +
				"      - name: \"staging\"\n        deployment_branch_policy:\n          mode: all\n        # variables: none\n",
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
