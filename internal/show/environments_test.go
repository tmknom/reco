package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestWriteEnvironments(t *testing.T) {
	unprotected := github.EnvironmentProtection{ReviewerSet: github.Observed(setting.ReviewerSetOfRule(nil)), CanAdminsBypass: true}
	// patternsOf は、patterns を一覧の全体として読んだ個別パターンの観測結果。
	patternsOf := func(patterns ...setting.DeploymentPattern) *github.SettingObservation[github.DeploymentPatterns] {
		observed := github.Observed(github.DeploymentPatterns{Existing: patterns})
		return &observed
	}
	// ruleOf は、rule の担当者を持つ required_reviewers の保護ルールの担当者の集合の観測結果。
	ruleOf := func(rule setting.RequiredReviewers) github.SettingObservation[setting.ReviewerSet] {
		return github.Observed(setting.ReviewerSetOfRule(&rule))
	}
	allMode := setting.DeploymentBranchPolicyAll
	unknownPatternsObservation := github.Unavailable[github.DeploymentPatterns](github.UnavailableUnknownValue)
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
				"      - name: \"staging\"\n        deployment_branch_policy:\n          mode: all\n        required_reviewers: []\n        prevent_self_review: false\n        wait_timer: 0 # minutes\n        # variables: none\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n        required_reviewers: []\n        prevent_self_review: false\n        wait_timer: 0 # minutes\n        # variables: none\n",
		},
		{
			name: "protected_branches shows only the mode",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "staging", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicyProtectedBranches, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"staging\"\n        deployment_branch_policy:\n          mode: protected_branches\n        required_reviewers: []\n        prevent_self_review: false\n        wait_timer: 0 # minutes\n        # variables: none\n",
		},
		{
			name: "selected shows each pattern as the type and the quoted name",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected,
					DeploymentPatterns: patternsOf(
						setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "main"},
						setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: "v*"},
					),
					Protection: unprotected,
					Variables:  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns:\n" +
				"            - type: branch\n              name: \"main\"\n" +
				"            - type: tag\n              name: \"v*\"\n        required_reviewers: []\n        prevent_self_review: false\n        wait_timer: 0 # minutes\n        # variables: none\n",
		},
		{
			name: "selected without patterns shows the empty sequence",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "production", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected, DeploymentPatterns: patternsOf(), Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns: []\n        required_reviewers: []\n        prevent_self_review: false\n        wait_timer: 0 # minutes\n        # variables: none\n",
		},
		{
			name: "wait_timer is shown in minutes as a trailing comment and can_admins_bypass is not shown",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: allMode,
					Protection:                 github.EnvironmentProtection{WaitTimer: 30, ReviewerSet: unprotected.ReviewerSet, CanAdminsBypass: false},
					Variables:                  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n        required_reviewers: []\n        prevent_self_review: false\n        wait_timer: 30 # minutes\n        # variables: none\n",
		},
		{
			name: "reviewers are shown in the given order with the login as a trailing comment",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: allMode,
					Protection: github.EnvironmentProtection{ReviewerSet: ruleOf(setting.RequiredReviewers{
						Reviewers: []setting.Reviewer{
							{Type: setting.ReviewerUser, ID: 1234567},
							{Type: setting.ReviewerUser, ID: 583231},
						},
						Logins: map[setting.Reviewer]string{
							{Type: setting.ReviewerUser, ID: 583231}:  "octocat",
							{Type: setting.ReviewerUser, ID: 1234567}: "hubot",
						},
					})},
					Variables: noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n" +
				"        required_reviewers:\n" +
				"          - type: User\n            id: 1234567 # hubot\n" +
				"          - type: User\n            id: 583231 # octocat\n" +
				"        prevent_self_review: false\n        wait_timer: 0 # minutes\n" +
				"        # variables: none\n",
		},
		{
			name: "prevent_self_review follows the reviewers and precedes wait_timer",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: allMode,
					Protection: github.EnvironmentProtection{
						WaitTimer:         5,
						ReviewerSet:       ruleOf(setting.RequiredReviewers{Reviewers: []setting.Reviewer{{Type: setting.ReviewerUser, ID: 7}}}),
						PreventSelfReview: true,
					},
					Variables: noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n" +
				"        required_reviewers:\n" +
				"          - type: User\n            id: 7\n" +
				"        prevent_self_review: true\n        wait_timer: 5 # minutes\n" +
				"        # variables: none\n",
		},
		{
			name: "a user without an observed login has no comment and a team has none",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: allMode,
					Protection: github.EnvironmentProtection{ReviewerSet: ruleOf(setting.RequiredReviewers{
						Reviewers: []setting.Reviewer{
							{Type: setting.ReviewerUser, ID: 7},
							{Type: setting.ReviewerTeam, ID: 42},
						},
						// チームの ID と同じ値の利用者がいても、チームには付けない
						Logins: map[setting.Reviewer]string{{Type: setting.ReviewerUser, ID: 42}: "someone"},
					})},
					Variables: noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n" +
				"        required_reviewers:\n" +
				"          - type: User\n            id: 7\n" +
				"          - type: Team\n            id: 42\n" +
				"        prevent_self_review: false\n        wait_timer: 0 # minutes\n" +
				"        # variables: none\n",
		},
		{
			name: "a protection rule without reviewers is shown as a comment and still shows its prevent_self_review",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: allMode,
					Protection:                 github.EnvironmentProtection{WaitTimer: 10, ReviewerSet: ruleOf(setting.RequiredReviewers{}), PreventSelfReview: true},
					Variables:                  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n" +
				"        # required_reviewers: no reviewers (a protection rule without reviewers remains)\n" +
				"        prevent_self_review: true\n        wait_timer: 10 # minutes\n" +
				"        # variables: none\n",
		},
		{
			name: "a name with a newline is escaped onto one line",
			environments: github.Environments{Existing: []github.Environment{
				{Name: "a\nb", DeploymentBranchPolicyMode: allMode, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"a\\nb\"\n        deployment_branch_policy:\n          mode: all\n        required_reviewers: []\n        prevent_self_review: false\n        wait_timer: 0 # minutes\n        # variables: none\n",
		},
		{
			name: "a pattern name with a newline is escaped onto one line",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected,
					DeploymentPatterns:         patternsOf(setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "a\nb"}),
					Protection:                 unprotected,
					Variables:                  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns:\n" +
				"            - type: branch\n              name: \"a\\nb\"\n        required_reviewers: []\n        prevent_self_review: false\n        wait_timer: 0 # minutes\n        # variables: none\n",
		},
		{
			name: "variables follow wait_timer of their environment and precede the next environment",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected,
					DeploymentPatterns:         patternsOf(setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "main"}),
					Protection:                 unprotected,
					Variables:                  &github.Variables{Existing: []github.Variable{{Name: "REGION", Value: "eu"}}},
				},
				{Name: "staging", DeploymentBranchPolicyMode: allMode, Protection: unprotected, Variables: noVariables},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: selected\n          patterns:\n" +
				"            - type: branch\n              name: \"main\"\n" +
				"        required_reviewers: []\n" +
				"        prevent_self_review: false\n        wait_timer: 0 # minutes\n" +
				"        variables:\n          - name: \"REGION\"\n            value: \"eu\"\n" +
				"      - name: \"staging\"\n        deployment_branch_policy:\n          mode: all\n        required_reviewers: []\n        prevent_self_review: false\n        wait_timer: 0 # minutes\n        # variables: none\n",
		},
		{
			name: "an unknown deployment branch policy value is a comment in place of the whole key and the other lines are shown",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected,
					DeploymentPatterns:         &unknownPatternsObservation,
					Protection:                 unprotected,
					Variables:                  noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n" +
				"        # deployment_branch_policy: unavailable (GitHub returned a value that reco does not know)\n" +
				"        required_reviewers: []\n        prevent_self_review: false\n        wait_timer: 0 # minutes\n        # variables: none\n",
		},
		{
			name: "unknown required reviewers are a comment and prevent_self_review and wait_timer are shown",
			environments: github.Environments{Existing: []github.Environment{
				{
					Name:                       "production",
					DeploymentBranchPolicyMode: allMode,
					Protection: github.EnvironmentProtection{
						WaitTimer: 15, PreventSelfReview: true,
						ReviewerSet: github.Unavailable[setting.ReviewerSet](github.UnavailableUnknownValue),
					},
					Variables: noVariables,
				},
			}},
			want: "    environments:\n" +
				"      - name: \"production\"\n        deployment_branch_policy:\n          mode: all\n" +
				"        # required_reviewers: unavailable (GitHub returned a value that reco does not know)\n" +
				"        prevent_self_review: true\n        wait_timer: 15 # minutes\n        # variables: none\n",
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

	privatePatterns := github.Unavailable[github.DeploymentPatterns](github.UnavailablePrivateRepository)
	panics := []struct {
		name        string
		environment github.Environment
	}{
		{
			name: "deployment branch policy unavailable for a reason other than an unknown value panics",
			environment: github.Environment{
				Name: "production", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected, DeploymentPatterns: &privatePatterns, Protection: unprotected, Variables: noVariables,
			},
		},
		{
			name: "required reviewers unavailable for a reason other than an unknown value panics",
			environment: github.Environment{
				Name: "production", DeploymentBranchPolicyMode: allMode, Protection: github.EnvironmentProtection{ReviewerSet: github.Unavailable[setting.ReviewerSet](github.UnavailablePrivateRepository)}, Variables: noVariables,
			},
		},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("writeEnvironments() did not panic")
				}
			}()
			var b strings.Builder
			writeEnvironments(&b, github.Environments{Existing: []github.Environment{tc.environment}})
		})
	}
}
