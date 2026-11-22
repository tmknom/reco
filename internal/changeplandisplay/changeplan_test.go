package changeplandisplay

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

func TestPluralize(t *testing.T) {
	cases := []struct {
		name             string
		n                int
		singular, plural string
		want             string
	}{
		{name: "zero is plural", n: 0, singular: "change", plural: "changes", want: "0 changes"},
		{name: "one is singular", n: 1, singular: "change", plural: "changes", want: "1 change"},
		{name: "two uses the given plural form", n: 2, singular: "repository", plural: "repositories", want: "2 repositories"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pluralize(tc.n, tc.singular, tc.plural); got != tc.want {
				t.Errorf("pluralize() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-005 FR-007 QR-002
func TestText(t *testing.T) {
	target := mustReference(t, "owner/repo")
	cases := []struct {
		name     string
		settings []changeplan.SettingPlan
		want     string
	}{
		{
			name: "mismatch is shown as a change with a summary",
			settings: []changeplan.SettingPlan{
				{Unit: setting.Unit{Key: "general.allow_auto_merge"}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n  general.allow_auto_merge: false -> true\n\n" +
				"Plan: 1 repository to update, 1 change\nRepositories checked: 1\n",
		},
		{
			name: "repository variable creation and value update are both counted as changes",
			settings: []changeplan.SettingPlan{
				{
					Unit: setting.Unit{Key: setting.RepositoryVariable, Variable: mustVariableName(t, "GO_VERSION")}, Observed: github.Observed(setting.VariableValue("1.26")),
					Desired: setting.VariableValue("1.27"), Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				},
				{
					Unit: setting.Unit{Key: setting.RepositoryVariable, Variable: mustVariableName(t, "GO_ARCH")}, Observed: github.Observed(setting.AbsentVariable()),
					Desired: setting.VariableValue("amd64"), Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n" +
				"  actions.variables.GO_VERSION: \"1.26\" -> \"1.27\"\n  actions.variables.GO_ARCH: absent -> \"amd64\"\n\n" +
				"Plan: 1 repository to update, 2 changes\nRepositories checked: 1\n",
		},
		{
			name: "environment variables are shown under their environment in the plan order and an environment to create with two variables counts as three changes",
			settings: []changeplan.SettingPlan{
				{
					Unit: setting.Unit{Key: setting.RepositoryVariable, Variable: mustVariableName(t, "REGION")}, Observed: github.Observed(setting.VariableValue("us-east-1")),
					Desired: setting.VariableValue("us-west-2"), Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				},
				{
					Unit:     setting.Unit{Key: setting.EnvironmentVariable, Environment: mustEnvironmentName(t, "production"), Variable: mustVariableName(t, "REGION")},
					Observed: github.Observed(setting.VariableValue("ap-northeast-1")), Desired: setting.VariableValue("ap-northeast-3"), Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				},
				{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "staging")}, Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
				{
					Unit:     setting.Unit{Key: setting.EnvironmentVariable, Environment: mustEnvironmentName(t, "staging"), Variable: mustVariableName(t, "REGION")},
					Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.VariableValue("ap-northeast-1"), Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
				},
				{
					Unit:     setting.Unit{Key: setting.EnvironmentVariable, Environment: mustEnvironmentName(t, "staging"), Variable: mustVariableName(t, "DEBUG")},
					Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.VariableValue("true"), Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
				},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n" +
				"  actions.variables.REGION: \"us-east-1\" -> \"us-west-2\"\n" +
				"  environments.production.variables.REGION: \"ap-northeast-1\" -> \"ap-northeast-3\"\n" +
				"  environments.staging: absent -> present\n" +
				"  environments.staging.variables.REGION: (environment absent) -> \"ap-northeast-1\"\n" +
				"  environments.staging.variables.DEBUG: (environment absent) -> \"true\"\n\n" +
				"Plan: 1 repository to update, 5 changes\nRepositories checked: 1\n",
		},
		{
			name: "general and workflow permissions mismatches are shown in the plan order with enumerated values and counted",
			settings: []changeplan.SettingPlan{
				{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
				{
					Unit:     setting.Unit{Key: setting.DefaultWorkflowPermissions},
					Observed: github.Observed(setting.PermissionLevelValue(setting.PermissionWrite)), Desired: setting.PermissionLevelValue(setting.PermissionRead),
					Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
				},
				{Unit: setting.Unit{Key: setting.CanApprovePullRequestReviews}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false), Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n" +
				"  general.allow_auto_merge: false -> true\n" +
				"  actions.workflow.default_workflow_permissions: write -> read\n" +
				"  actions.workflow.can_approve_pull_request_reviews: true -> false\n\n" +
				"Plan: 1 repository to update, 3 changes\nRepositories checked: 1\n",
		},
		{
			name: "matched settings are not shown as changes",
			settings: []changeplan.SettingPlan{
				{Unit: setting.Unit{Key: "general.delete_branch_on_merge"}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: changeplan.Match},
				{Unit: setting.Unit{Key: "general.allow_auto_merge"}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false), Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n  general.allow_auto_merge: true -> false\n\n" +
				"Plan: 1 repository to update, 1 change\nRepositories checked: 1\n",
		},
		{
			name: "creation and mode changes of environments are shown in the plan order and each counted",
			settings: []changeplan.SettingPlan{
				{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "production")}, Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
				{
					Unit:     setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: mustEnvironmentName(t, "production")},
					Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
					Desired:  setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}),
					Result:   changeplan.NotComparable, Applicability: changeplan.Applicable,
				},
				{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "staging")}, Result: changeplan.Match},
				{
					Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: mustEnvironmentName(t, "staging")},
					Observed: github.Observed(setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{
						{Type: setting.DeploymentPatternBranch, Name: "main"}, {Type: setting.DeploymentPatternTag, Name: "v*"},
					}})),
					Desired: setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}),
					Result:  changeplan.Mismatch, Applicability: changeplan.Applicable,
				},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n" +
				"  environments.production: absent -> present\n" +
				"  environments.production.deployment_branch_policy: (environment absent) -> protected_branches\n" +
				"  environments.staging.deployment_branch_policy:\n    selected [branch \"main\", tag \"v*\"]\n    -> all\n\n" +
				"Plan: 1 repository to update, 3 changes\nRepositories checked: 1\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := changeplan.Plan{Repositories: []changeplan.RepositoryPlan{{Target: target, RepositoryID: 1, Settings: tc.settings}}}
			if got := Text("./reco.yaml", p); got != tc.want {
				t.Errorf("Text() = %q, want %q", got, tc.want)
			}
		})
	}
}

// 期待値は CLI 仕様の plan の表示例から導く。
//
//trace:checks FR-005 FR-007 QR-002 INV-001
func TestText_ActionsPermissions(t *testing.T) {
	target := mustReference(t, "owner/repo")
	criteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: []string{"example/*"}}
	policy := setting.AllowedActionsPolicyValue
	notSelected := github.Unavailable[setting.Value](github.UnavailableNotSelected)
	const header = "Configuration: ./reco.yaml\n\nRepository: owner/repo\n"
	cases := []struct {
		name     string
		settings []changeplan.SettingPlan
		want     string
	}{
		{
			name: "all to selected counts the policy mismatch and the not-comparable criteria group as two changes",
			settings: []changeplan.SettingPlan{
				{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Observed(policy(setting.AllowedActionsAll)), Desired: policy(setting.AllowedActionsSelected), Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
				{Unit: setting.Unit{Key: setting.SelectedActions}, Observed: notSelected, Desired: setting.SelectedActionsCriteriaValue(criteria), Result: changeplan.NotComparable, Applicability: changeplan.Applicable},
			},
			want: header +
				"  actions.permissions.allowed_actions: all -> selected\n" +
				"  actions.permissions.selected_actions:\n" +
				"    (not in use)\n" +
				"    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"]\n\n" +
				"Plan: 1 repository to update, 2 changes\nRepositories checked: 1\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := changeplan.Plan{Repositories: []changeplan.RepositoryPlan{{Target: target, RepositoryID: 1, Settings: tc.settings}}}
			if got := Text("./reco.yaml", p); got != tc.want {
				t.Errorf("Text() = %q, want %q", got, tc.want)
			}
		})
	}
}

// 期待値は CLI 仕様の plan の表示例と、節の並び・数え方の規則から導く。
// 計画は宣言の並びを名前順と逆にし、名前順に並べ替える実装を失敗させる。
//
//trace:checks FR-007 QR-002
func TestText_Repositories(t *testing.T) {
	zeta, alpha := mustReference(t, "owner/zeta"), mustReference(t, "owner/alpha")
	match := changeplan.SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(false), Result: changeplan.Match}
	mismatch := changeplan.SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: changeplan.Mismatch, Applicability: changeplan.Applicable}
	notComparable := changeplan.SettingPlan{
		Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled),
		Desired: setting.AllowedActionsPolicyValue(setting.AllowedActionsAll), Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
	}
	notApplicable := changeplan.SettingPlan{
		Unit: setting.Unit{Key: setting.SHAPinningRequired}, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Desired: setting.BoolValue(true),
		Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable, NotApplicableReason: changeplan.NotApplicableActionsRemainDisabled,
	}
	workflowMismatch := changeplan.SettingPlan{
		Unit: setting.Unit{Key: setting.DefaultWorkflowPermissions}, Observed: github.Observed(setting.PermissionLevelValue(setting.PermissionWrite)),
		Desired: setting.PermissionLevelValue(setting.PermissionRead), Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
	}
	const notApplicableText = "  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true\n" +
		"    Applicability: not applicable (GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it)\n"
	cases := []struct {
		name         string
		repositories []changeplan.RepositoryPlan
		want         string
	}{
		{
			name: "unchanged repository has no section but is counted as checked",
			repositories: []changeplan.RepositoryPlan{
				{Target: zeta, RepositoryID: 1, Settings: []changeplan.SettingPlan{match}},
				{Target: alpha, RepositoryID: 2, Settings: []changeplan.SettingPlan{mismatch}},
			},
			want: "Configuration: ./reco.yaml\n\n" +
				"Repository: owner/alpha\n  general.allow_auto_merge: false -> true\n\n" +
				"Plan: 1 repository to update, 1 change\nRepositories checked: 2\n",
		},
		{
			name: "repositories to update are shown as sections in the declared order and mismatches and not-comparable settings are totaled as changes",
			repositories: []changeplan.RepositoryPlan{
				{Target: zeta, RepositoryID: 1, Settings: []changeplan.SettingPlan{mismatch, notComparable}},
				{Target: alpha, RepositoryID: 2, Settings: []changeplan.SettingPlan{mismatch}},
			},
			want: "Configuration: ./reco.yaml\n\n" +
				"Repository: owner/zeta\n  general.allow_auto_merge: false -> true\n" +
				"  actions.permissions.allowed_actions: (GitHub Actions disabled) -> all (enables GitHub Actions)\n\n" +
				"Repository: owner/alpha\n  general.allow_auto_merge: false -> true\n\n" +
				"Plan: 2 repositories to update, 3 changes\nRepositories checked: 2\n",
		},
		{
			name: "repository with only a not-applicable change is shown and counted, followed by plan cannot be applied",
			repositories: []changeplan.RepositoryPlan{
				{Target: zeta, RepositoryID: 1, Settings: []changeplan.SettingPlan{notApplicable}},
			},
			want: "Configuration: ./reco.yaml\n\n" +
				"Repository: owner/zeta\n" + notApplicableText + "\n" +
				"Plan: 1 repository to update, 1 change\nRepositories checked: 1\n\nPlan cannot be applied.\n",
		},
		{
			name: "applicable and not-applicable changes in one repository are both counted and plan cannot be applied is shown once",
			repositories: []changeplan.RepositoryPlan{
				{Target: zeta, RepositoryID: 1, Settings: []changeplan.SettingPlan{notApplicable, workflowMismatch}},
			},
			want: "Configuration: ./reco.yaml\n\n" +
				"Repository: owner/zeta\n" + notApplicableText + "  actions.workflow.default_workflow_permissions: write -> read\n\n" +
				"Plan: 1 repository to update, 2 changes\nRepositories checked: 1\n\nPlan cannot be applied.\n",
		},
		{
			name: "not-applicable changes in two repositories are both shown and plan cannot be applied is shown once",
			repositories: []changeplan.RepositoryPlan{
				{Target: zeta, RepositoryID: 1, Settings: []changeplan.SettingPlan{notApplicable}},
				{Target: alpha, RepositoryID: 2, Settings: []changeplan.SettingPlan{notApplicable}},
			},
			want: "Configuration: ./reco.yaml\n\n" +
				"Repository: owner/zeta\n" + notApplicableText + "\n" +
				"Repository: owner/alpha\n" + notApplicableText + "\n" +
				"Plan: 2 repositories to update, 2 changes\nRepositories checked: 2\n\nPlan cannot be applied.\n",
		},
		{
			name: "not-applicable change in the first repository is followed by plan cannot be applied even when the next has only applicable changes",
			repositories: []changeplan.RepositoryPlan{
				{Target: zeta, RepositoryID: 1, Settings: []changeplan.SettingPlan{notApplicable}},
				{Target: alpha, RepositoryID: 2, Settings: []changeplan.SettingPlan{mismatch}},
			},
			want: "Configuration: ./reco.yaml\n\n" +
				"Repository: owner/zeta\n" + notApplicableText + "\n" +
				"Repository: owner/alpha\n  general.allow_auto_merge: false -> true\n\n" +
				"Plan: 2 repositories to update, 2 changes\nRepositories checked: 2\n\nPlan cannot be applied.\n",
		},
		{
			name: "two unchanged repositories report no changes and the number checked",
			repositories: []changeplan.RepositoryPlan{
				{Target: zeta, RepositoryID: 1, Settings: []changeplan.SettingPlan{match}},
				{Target: alpha, RepositoryID: 2, Settings: []changeplan.SettingPlan{match}},
			},
			want: "No changes.\nRepositories checked: 2\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Text("./reco.yaml", changeplan.Plan{Repositories: tc.repositories}); got != tc.want {
				t.Errorf("Text() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("no repository plan panics as a broken invariant", func(t *testing.T) {
		defer func() {
			got, ok := recover().(string)
			if !ok || !strings.Contains(got, "no repository plan") {
				t.Errorf("Text() panicked with %q, want a message naming no repository plan", got)
			}
		}()
		_ = Text("./reco.yaml", changeplan.Plan{})
	})
}

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}
