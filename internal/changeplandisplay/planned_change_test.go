package changeplandisplay

import (
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// 期待値は CLI 仕様 4章の表示例から導く。
//
//trace:checks FR-005 FR-007 QR-002 INV-001
func TestPlannedChangeText(t *testing.T) {
	criteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: []string{"example/*"}}
	policy := setting.AllowedActionsPolicyValue
	disabled := github.Unavailable[setting.Value](github.UnavailableActionsDisabled)
	notSelected := github.Unavailable[setting.Value](github.UnavailableNotSelected)
	const possibleLoss = "    Side effect: GitHub may discard selected_actions if allowed_actions was \"selected\" before GitHub Actions was disabled (reco cannot read them while GitHub Actions is disabled)\n"
	cases := []struct {
		name string
		s    changeplan.SettingPlan
		want string
	}{
		{
			name: "single setting mismatch is shown as current -> desired on one line",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  general.allow_auto_merge: false -> true\n",
		},
		{
			name: "criteria group mismatch shows the current and desired values on separate lines",
			s: changeplan.SettingPlan{
				Unit:     setting.Unit{Key: setting.SelectedActions},
				Observed: github.Observed(setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}})),
				Desired:  setting.SelectedActionsCriteriaValue(criteria), Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  actions.permissions.selected_actions:\n" +
				"    github_owned_allowed: true, verified_allowed: false, patterns_allowed: []\n" +
				"    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"]\n",
		},
		{
			name: "not-comparable criteria group under a policy other than selected shows not in use as the repository state on separate lines",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.SelectedActions}, Observed: notSelected, Desired: setting.SelectedActionsCriteriaValue(criteria),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
			},
			want: "  actions.permissions.selected_actions:\n" +
				"    (not in use)\n" +
				"    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"]\n",
		},
		{
			name: "not-comparable criteria group on disabled actions shows actions disabled as the repository state",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.SelectedActions}, Observed: disabled, Desired: setting.SelectedActionsCriteriaValue(criteria),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
			},
			want: "  actions.permissions.selected_actions:\n" +
				"    (GitHub Actions disabled)\n" +
				"    -> github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"]\n",
		},
		{
			name: "not-comparable policy that enables actions shows the repository state on one line and marks the desired value",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.AllowedActions}, Observed: disabled, Desired: policy(setting.AllowedActionsSelected),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
			},
			want: "  actions.permissions.allowed_actions: (GitHub Actions disabled) -> selected (enables GitHub Actions)\n",
		},
		{
			name: "known change shows the side effect with the observed criteria under it",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Observed(policy(setting.AllowedActionsSelected)), Desired: policy(setting.AllowedActionsAll),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
				SideEffects: []changeplan.SideEffect{{Kind: changeplan.SideEffectSelectedActionsDiscarded, Observed: github.Observed(setting.SelectedActionsCriteriaValue(criteria))}},
			},
			want: "  actions.permissions.allowed_actions: selected -> all\n" +
				"    Side effect: GitHub discards the current selected_actions (github_owned_allowed: true, verified_allowed: true, patterns_allowed: [\"example/*\"])\n",
		},
		{
			name: "not-comparable setting shows the possible loss when the observed criteria are unavailable",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.AllowedActions}, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
				SideEffects: []changeplan.SideEffect{{Kind: changeplan.SideEffectSelectedActionsDiscarded, Observed: disabled}},
			},
			want: "  actions.permissions.allowed_actions: (GitHub Actions disabled) -> local_only (enables GitHub Actions)\n" +
				possibleLoss,
		},
		{
			name: "policy enabling actions shows the one resumed setting before the side effect",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.AllowedActions}, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
				SideEffects:     []changeplan.SideEffect{{Kind: changeplan.SideEffectSelectedActionsDiscarded, Observed: disabled}},
				ResumedSettings: []setting.Key{setting.SHAPinningRequired},
			},
			want: "  actions.permissions.allowed_actions: (GitHub Actions disabled) -> local_only (enables GitHub Actions)\n" +
				"    Previous settings: GitHub Actions resumes with sha_pinning_required kept from before it was disabled (reco cannot read it while GitHub Actions is disabled)\n" +
				possibleLoss,
		},
		{
			name: "reenabling actions lists the resumed settings",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.ActionsEnabled}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
				ResumedSettings: []setting.Key{setting.AllowedActions, setting.SelectedActions, setting.SHAPinningRequired},
			},
			want: "  actions.permissions.enabled: false -> true\n" +
				"    Previous settings: GitHub Actions resumes with allowed_actions, selected_actions, and sha_pinning_required kept from before it was disabled (reco cannot read them while GitHub Actions is disabled)\n",
		},
		{
			name: "reenabling actions with sha_pinning_required declared lists only the policy and the criteria",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.ActionsEnabled}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
				ResumedSettings: []setting.Key{setting.AllowedActions, setting.SelectedActions},
			},
			want: "  actions.permissions.enabled: false -> true\n" +
				"    Previous settings: GitHub Actions resumes with allowed_actions and selected_actions kept from before it was disabled (reco cannot read them while GitHub Actions is disabled)\n",
		},
		{
			name: "not-applicable change shows the applicability with the reason under it",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.SHAPinningRequired}, Observed: disabled, Desired: setting.BoolValue(true),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicableActionsRemainDisabled),
			},
			want: "  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true\n" +
				"    Applicability: not applicable (GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it)\n",
		},
		{
			name: "access_level of a public repository shows the repository state and the reason to remove the declaration",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.ActionsAccessLevel}, Observed: github.Unavailable[setting.Value](github.UnavailablePublicRepository), Desired: setting.AccessLevelValue(setting.AccessNone),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicablePublicRepository),
			},
			want: "  actions.access.access_level: (public repository) -> none\n" +
				"    Applicability: not applicable (the repository is public; remove access_level, which applies only to private repositories)\n",
		},
		{
			name: "approval_policy of a private repository shows the repository state and that it applies only to public repositories",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.ForkPullRequestApproval}, Observed: github.Unavailable[setting.Value](github.UnavailablePrivateRepository),
				Desired: setting.ForkPullRequestApprovalPolicyValue(setting.ForkPullRequestApprovalAllExternalContributors),
				Result:  changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicablePrivateRepository),
			},
			want: "  actions.fork_pull_requests.approval_policy: (private repository) -> all_external_contributors\n" +
				"    Applicability: not applicable (the repository is private; remove approval_policy, which applies only to public repositories)\n",
		},
		{
			name: "shortening retention days shows one line without a side effect or a note",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.RetentionDays}, Observed: github.Observed(setting.RetentionDaysValue(90)), Desired: setting.RetentionDaysValue(30),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  actions.retention.days: 90 -> 30\n",
		},
		{
			name: "retention days over the maximum shows the declared days and the maximum",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.RetentionDays}, Observed: github.Observed(setting.RetentionDaysValue(90)), Desired: setting.RetentionDaysValue(120),
				Result: changeplan.Mismatch, Applicability: changeplan.NotApplicableAboveMaximum(90),
			},
			want: "  actions.retention.days: 90 -> 120\n" +
				"    Applicability: not applicable (120 days exceeds the maximum of 90 days that GitHub allows for this repository; declare days: 90 or less)\n",
		},
		{
			name: "private fork pull request workflows mismatch shows the values on separate lines and notes each enabled transmission in order",
			s: changeplan.SettingPlan{
				Unit:     setting.Unit{Key: setting.PrivateForkPullRequestWorkflows},
				Observed: github.Observed(setting.PrivateForkPullRequestWorkflowConditionsValue(setting.PrivateForkPullRequestWorkflowConditions{})),
				Desired: setting.PrivateForkPullRequestWorkflowConditionsValue(setting.PrivateForkPullRequestWorkflowConditions{
					RunWorkflowsFromForkPullRequests: true, SendWriteTokensToWorkflows: true, SendSecretsAndVariables: true, RequireApprovalForForkPullRequestWorkflows: true,
				}),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  actions.fork_pull_requests.private_repositories:\n" +
				"    run_workflows_from_fork_pull_requests: false, send_write_tokens_to_workflows: false, send_secrets_and_variables: false, require_approval_for_fork_pr_workflows: false\n" +
				"    -> run_workflows_from_fork_pull_requests: true, send_write_tokens_to_workflows: true, send_secrets_and_variables: true, require_approval_for_fork_pr_workflows: true\n" +
				"    Note: Workflows triggered by pull requests from forks get a GITHUB_TOKEN with write permissions\n" +
				"    Note: Workflows triggered by pull requests from forks can use secrets and variables\n",
		},
		{
			name: "oidc subject mismatch shows the values on separate lines and notes the subject change",
			s: changeplan.SettingPlan{
				Unit:     setting.Unit{Key: setting.OIDCSubject},
				Observed: github.Observed(setting.OIDCSubjectCustomizationValue(setting.OIDCSubjectCustomization{UseDefault: true})),
				Desired:  setting.OIDCSubjectCustomizationValue(setting.OIDCSubjectCustomization{IncludeClaimKeys: []string{"repo", "context"}}),
				Result:   changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  actions.oidc_subject:\n" +
				"    use_default: true\n" +
				"    -> use_default: false, include_claim_keys: [\"repo\", \"context\"]\n" +
				"    Note: This changes how the subject claim of OIDC tokens is composed; trust conditions in external cloud providers that match the current subject claim may stop matching\n",
		},
		{
			name: "private fork pull request workflows of a public repository show the repository state and the reason to remove private_repositories",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.PrivateForkPullRequestWorkflows}, Observed: github.Unavailable[setting.Value](github.UnavailablePublicRepository),
				Desired: setting.PrivateForkPullRequestWorkflowConditionsValue(setting.PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true}),
				Result:  changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicablePublicRepository),
			},
			want: "  actions.fork_pull_requests.private_repositories:\n" +
				"    (public repository)\n" +
				"    -> run_workflows_from_fork_pull_requests: true, send_write_tokens_to_workflows: false, send_secrets_and_variables: false, require_approval_for_fork_pr_workflows: false\n" +
				"    Applicability: not applicable (the repository is public; remove private_repositories, which applies only to private repositories)\n",
		},
		{
			name: "applicable not-comparable sha_pinning_required shows no applicability",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.SHAPinningRequired}, Observed: disabled, Desired: setting.BoolValue(true),
				Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
			},
			want: "  actions.permissions.sha_pinning_required: (GitHub Actions disabled) -> true\n",
		},
		{
			name: "disabling dependabot alerts shows the dependency graph and the paused security updates as side effects in this order",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.DependabotAlerts}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
				SideEffects: []changeplan.SideEffect{
					{Kind: changeplan.SideEffectDependencyGraphDisabled},
					{Kind: changeplan.SideEffectSecurityUpdatesDisabled, Observed: github.Observed(setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesPaused))},
				},
			},
			want: "  security.dependabot_alerts: true -> false\n" +
				"    Side effect: GitHub also disables the dependency graph\n" +
				"    Side effect: GitHub also disables dependabot_security_updates, which is currently true (paused by GitHub)\n",
		},
		{
			name: "enabling dependabot alerts shows that the dependency graph is enabled",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.DependabotAlerts}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
				SideEffects: []changeplan.SideEffect{{Kind: changeplan.SideEffectDependencyGraphEnabled}},
			},
			want: "  security.dependabot_alerts: false -> true\n" +
				"    Side effect: GitHub also enables the dependency graph\n",
		},
		{
			name: "security updates with disabled alerts shows the reason to declare alerts",
			s: changeplan.SettingPlan{
				Unit:     setting.Unit{Key: setting.DependabotSecurityUpdates},
				Observed: github.Observed(setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesDisabled)),
				Desired:  setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesEnabled),
				Result:   changeplan.Mismatch, Applicability: changeplan.NotApplicable(changeplan.NotApplicableDependabotAlertsRemainDisabled),
			},
			want: "  security.dependabot_security_updates: false -> true\n" +
				"    Applicability: not applicable (Dependabot alerts is disabled; declare dependabot_alerts: true to enable it)\n",
		},
		{
			name: "secret scanning of a private repository shows the repository state and the reason to remove its own key",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.SecretScanningEnabled}, Observed: github.Unavailable[setting.Value](github.UnavailablePrivateRepository), Desired: setting.BoolValue(true),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicablePrivateRepository),
			},
			want: "  security.secret_scanning: (private repository) -> true\n" +
				"    Applicability: not applicable (the repository is private; remove secret_scanning, which reco manages only on public repositories)\n",
		},
		{
			name: "private vulnerability reporting of a private repository names its own key to remove",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.PrivateVulnerabilityReportingEnabled}, Observed: github.Unavailable[setting.Value](github.UnavailablePrivateRepository), Desired: setting.BoolValue(true),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicablePrivateRepository),
			},
			want: "  security.private_vulnerability_reporting: (private repository) -> true\n" +
				"    Applicability: not applicable (the repository is private; remove private_vulnerability_reporting, which reco manages only on public repositories)\n",
		},
		{
			name: "push protection with disabled secret scanning shows the reason to declare secret scanning",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.RepositoryPushProtection}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
				Result: changeplan.Mismatch, Applicability: changeplan.NotApplicable(changeplan.NotApplicableSecretScanningRemainsDisabled),
			},
			want: "  security.secret_scanning_push_protection: false -> true\n" +
				"    Applicability: not applicable (Secret scanning is disabled; declare secret_scanning: true to enable it)\n",
		},
		{
			name: "disabling secret scanning shows the push protection as a side effect with its current value",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.SecretScanningEnabled}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
				SideEffects: []changeplan.SideEffect{{Kind: changeplan.SideEffectPushProtectionDisabled, Observed: github.Observed(setting.BoolValue(true))}},
			},
			want: "  security.secret_scanning: true -> false\n" +
				"    Side effect: GitHub also disables secret_scanning_push_protection, which is currently true\n",
		},
		{
			name: "disabling a merge method with one merge method left disabled names it and the declaration to enable it",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.AllowSquashMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false),
				Result: changeplan.Mismatch, Applicability: changeplan.NotApplicable(changeplan.NotApplicableNoMergeMethodEnabled, setting.AllowRebaseMerge),
			},
			want: "  general.allow_squash_merge: true -> false\n" +
				"    Applicability: not applicable (allow_rebase_merge is disabled, so no merge method would remain enabled; declare allow_rebase_merge: true to enable it)\n",
		},
		{
			name: "disabling a merge method with two merge methods left disabled names both and either declaration",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.AllowSquashMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false),
				Result: changeplan.Mismatch, Applicability: changeplan.NotApplicable(changeplan.NotApplicableNoMergeMethodEnabled, setting.AllowMergeCommit, setting.AllowRebaseMerge),
			},
			want: "  general.allow_squash_merge: true -> false\n" +
				"    Applicability: not applicable (allow_merge_commit and allow_rebase_merge are disabled, so no merge method would remain enabled; " +
				"declare allow_merge_commit: true or allow_rebase_merge: true to enable one of them)\n",
		},
		{
			name: "match is empty",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true),
				Result: changeplan.Match,
			},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlannedChangeText(tc.s); got != tc.want {
				t.Errorf("PlannedChangeText() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPlannedChangeText_UndefinedSideEffectPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("PlannedChangeText() did not panic for an undefined side effect")
		}
	}()
	_ = PlannedChangeText(changeplan.SettingPlan{
		Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true),
		Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
		SideEffects: []changeplan.SideEffect{{Observed: github.Observed(setting.BoolValue(true))}},
	})
}

func TestNotApplicableReasonText_UndefinedReasonPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("notApplicableReasonText() did not panic for an undefined not-applicable reason")
		}
	}()
	_ = notApplicableReasonText(changeplan.SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}})
}

func TestPlannedChangeNoteText_UndefinedNotePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("plannedChangeNoteText() did not panic for an undefined planned change note")
		}
	}()
	_ = plannedChangeNoteText(changeplan.PlannedChangeNote(0))
}

// 変更予定のマージ方式は宣言されているため、3つすべてが無効のまま残ることはない。
func TestNoMergeMethodEnabledText_AllMergeMethodsLeftDisabledPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("noMergeMethodEnabledText() did not panic for all merge methods left disabled")
		}
	}()
	_ = noMergeMethodEnabledText(setting.MergeMethods())
}

func TestRepositoryStateText_UndefinedReasonPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("repositoryStateText() did not panic for an undefined unavailable reason")
		}
	}()
	_ = repositoryStateText(0)
}

// 期待値は CLI 仕様の Repository Variable の表示例から導く。
//
//trace:checks FR-005 FR-007 QR-002
func TestPlannedChangeText_RepositoryVariables(t *testing.T) {
	unit := setting.Unit{Key: setting.RepositoryVariable, Variable: mustVariableName(t, "GO_VERSION")}
	variable := func(observed setting.Value, desired string) changeplan.SettingPlan {
		return changeplan.SettingPlan{Unit: unit, Observed: github.Observed(observed), Desired: setting.VariableValue(desired), Result: changeplan.Mismatch, Applicability: changeplan.Applicable()}
	}
	cases := []struct {
		name string
		s    changeplan.SettingPlan
		want string
	}{
		{name: "value update shows the quoted current and desired values on one line", s: variable(setting.VariableValue("1.26"), "1.27"), want: "  actions.variables.GO_VERSION: \"1.26\" -> \"1.27\"\n"},
		{name: "creation shows absent without parentheses as the current value", s: variable(setting.Absent(), "1.27"), want: "  actions.variables.GO_VERSION: absent -> \"1.27\"\n"},
		{
			name: "deletion shows the value to be lost escaped on one line and absent as the desired value",
			s:    changeplan.SettingPlan{Unit: unit, Observed: github.Observed(setting.VariableValue("line 1\nline 2\x01")), Desired: setting.Absent(), Result: changeplan.Mismatch, Applicability: changeplan.Applicable()},
			want: "  actions.variables.GO_VERSION: \"line 1\\nline 2\\x01\" -> absent\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlannedChangeText(tc.s); got != tc.want {
				t.Errorf("PlannedChangeText() = %q, want %q", got, tc.want)
			}
		})
	}
}

// 期待値は CLI 仕様 4章の Environment Variable の表示例から導く。
//
//trace:checks FR-005 FR-006 FR-007
func TestPlannedChangeText_EnvironmentVariables(t *testing.T) {
	unit := setting.Unit{Key: setting.EnvironmentVariable, Environment: mustEnvironmentName(t, "production"), Variable: mustVariableName(t, "REGION")}
	variable := func(observed github.SettingObservation[setting.Value], desired string, result changeplan.ReconciliationResult, applicability changeplan.Applicability) changeplan.SettingPlan {
		return changeplan.SettingPlan{Unit: unit, Observed: observed, Desired: setting.VariableValue(desired), Result: result, Applicability: applicability}
	}
	undetermined := variable(github.Unavailable[setting.Value](github.UnavailableNameUndetermined), "ap-northeast-1", changeplan.NotComparable, changeplan.NotApplicable(changeplan.NotApplicableNameUndetermined))
	cases := []struct {
		name string
		s    changeplan.SettingPlan
		want string
	}{
		{
			name: "value update of an existing environment shows the quoted values under the environment path",
			s:    variable(github.Observed(setting.VariableValue("ap-northeast-1")), "ap-northeast-3", changeplan.Mismatch, changeplan.Applicable()),
			want: "  environments.production.variables.REGION: \"ap-northeast-1\" -> \"ap-northeast-3\"\n",
		},
		{
			name: "creation in an existing environment shows absent without parentheses",
			s:    variable(github.Observed(setting.Absent()), "ap-northeast-1", changeplan.Mismatch, changeplan.Applicable()),
			want: "  environments.production.variables.REGION: absent -> \"ap-northeast-1\"\n",
		},
		{
			name: "variable of an environment to create shows environment absent as the state",
			s:    variable(github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), "ap-northeast-1", changeplan.NotComparable, changeplan.Applicable()),
			want: "  environments.production.variables.REGION: (environment absent) -> \"ap-northeast-1\"\n",
		},
		{
			name: "declared absence in an environment to create shows environment absent and absent",
			s: changeplan.SettingPlan{
				Unit: unit, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.Absent(), Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
			},
			want: "  environments.production.variables.REGION: (environment absent) -> absent\n",
		},
		{
			name: "variable of an environment whose existence is unknown shows the state and why it is not applicable",
			s:    undetermined,
			want: "  environments.production.variables.REGION: (existence unknown) -> \"ap-northeast-1\"\n" +
				"    Applicability: not applicable (an environment on GitHub has a name reco cannot compare, such as one with non-ASCII characters; remove this environment from the configuration)\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlannedChangeText(tc.s); got != tc.want {
				t.Errorf("PlannedChangeText() =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// 期待値は CLI 仕様 4章の Environment の表示例から導く。
//
//trace:checks FR-005 FR-006 FR-007
func TestPlannedChangeText_EnvironmentRequiredReviewers(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	octocat := setting.Reviewer{Type: setting.ReviewerUser, ID: 583231}
	hubot := setting.Reviewer{Type: setting.ReviewerUser, ID: 1234567}
	newcomer := setting.Reviewer{Type: setting.ReviewerUser, ID: 7654321}
	observedOf := func(logins map[setting.Reviewer]string, reviewers ...setting.Reviewer) github.SettingObservation[setting.Value] {
		return github.Observed(setting.ReviewerSetValue(setting.ReviewerSetOfRule(&setting.RequiredReviewers{Reviewers: reviewers, Logins: logins})))
	}
	desiredOf := func(reviewers ...setting.Reviewer) setting.Value {
		return setting.ReviewerSetValue(setting.ReviewerSetOf(reviewers))
	}
	logins := map[setting.Reviewer]string{octocat: "octocat", hubot: "hubot"}
	plan := func(observed github.SettingObservation[setting.Value], desired setting.Value, result changeplan.ReconciliationResult) changeplan.SettingPlan {
		return changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: production}, Observed: observed,
			Desired: desired, Result: result, Applicability: changeplan.Applicable(),
		}
	}
	const head = "  environments.production.required_reviewers:\n"
	cases := []struct {
		name string
		s    changeplan.SettingPlan
		want string
	}{
		{
			name: "matched reviewers are not shown",
			s:    plan(observedOf(logins, octocat), desiredOf(octocat), changeplan.Match),
			want: "",
		},
		{
			name: "removed and added reviewers show remove then add with logins copied to the desired value",
			s:    plan(observedOf(logins, octocat, hubot), desiredOf(octocat, newcomer), changeplan.Mismatch),
			want: head + "    [User 583231 (octocat), User 1234567 (hubot)]\n    -> [User 583231 (octocat), User 7654321]\n" +
				"    Reviewers to remove: [User 1234567 (hubot)]\n    Reviewers to add: [User 7654321]\n",
		},
		{
			name: "only added reviewers shows only Reviewers to add",
			s:    plan(observedOf(logins, octocat), desiredOf(octocat, newcomer), changeplan.Mismatch),
			want: head + "    [User 583231 (octocat)]\n    -> [User 583231 (octocat), User 7654321]\n    Reviewers to add: [User 7654321]\n",
		},
		{
			name: "reordering only shows no remove or add line",
			s:    plan(observedOf(logins, octocat, hubot), desiredOf(hubot, octocat), changeplan.Mismatch),
			want: head + "    [User 583231 (octocat), User 1234567 (hubot)]\n    -> [User 1234567 (hubot), User 583231 (octocat)]\n",
		},
		{
			name: "removing every reviewer shows an empty desired value and all current reviewers to remove",
			s:    plan(observedOf(logins, octocat), desiredOf(), changeplan.Mismatch),
			want: head + "    [User 583231 (octocat)]\n    -> []\n    Reviewers to remove: [User 583231 (octocat)]\n",
		},
		{
			name: "removing every reviewer with prevent_self_review true shows the side effect after the remove line",
			s: func() changeplan.SettingPlan {
				s := plan(observedOf(logins, octocat), desiredOf(), changeplan.Mismatch)
				s.SideEffects = []changeplan.SideEffect{{Kind: changeplan.SideEffectPreventSelfReviewDiscarded, Observed: github.Observed(setting.BoolValue(true))}}
				return s
			}(),
			want: head + "    [User 583231 (octocat)]\n    -> []\n    Reviewers to remove: [User 583231 (octocat)]\n" +
				"    Side effect: GitHub also discards prevent_self_review, which is currently true\n",
		},
		{
			name: "protection rule without reviewers shows the remark and no remove or add line",
			s:    plan(observedOf(nil), desiredOf(), changeplan.Mismatch),
			want: head + "    [] (protection rule without reviewers)\n    -> []\n",
		},
		{
			name: "environment absent shows the state and the desired value without logins",
			s:    plan(github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), desiredOf(octocat), changeplan.NotComparable),
			want: head + "    (environment absent)\n    -> [User 583231]\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlannedChangeText(tc.s); got != tc.want {
				t.Errorf("PlannedChangeText() =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// 期待値は CLI 仕様 4章の Environment の表示例から導く。
//
//trace:checks FR-005 FR-006 FR-007
func TestPlannedChangeText_Environments(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	protected := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches})
	all := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll})
	selected := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{
		{Type: setting.DeploymentPatternBranch, Name: "main"}, {Type: setting.DeploymentPatternTag, Name: "v*"},
	}})
	selectedOf := func(patterns ...setting.DeploymentPattern) setting.Value {
		return setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: patterns})
	}
	branch := func(name string) setting.DeploymentPattern {
		return setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: name}
	}
	tag := func(name string) setting.DeploymentPattern {
		return setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: name}
	}
	const notApplicable = "    Applicability: not applicable (an environment on GitHub has a name reco cannot compare, such as one with non-ASCII characters; remove this environment from the configuration)\n"
	cases := []struct {
		name string
		s    changeplan.SettingPlan
		want string
	}{
		{
			name: "matched environment is not shown",
			s:    changeplan.SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Result: changeplan.Match},
			want: "",
		},
		{
			name: "creation shows absent -> present",
			s:    changeplan.SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: changeplan.Mismatch, Applicability: changeplan.Applicable()},
			want: "  environments.production: absent -> present\n",
		},
		{
			name: "mode of an environment to create shows environment absent as the state",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
				Desired: protected, Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
			},
			want: "  environments.production.deployment_branch_policy: (environment absent) -> protected_branches\n",
		},
		{
			name: "mode changed from selected shows the patterns in the current value without a side effect",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(selected),
				Desired: all, Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  environments.production.deployment_branch_policy:\n    selected [branch \"main\", tag \"v*\"]\n    -> all\n",
		},
		{
			name: "mode changed to selected shows three lines without pattern lines",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(all),
				Desired: selected, Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  environments.production.deployment_branch_policy:\n    all\n    -> selected [branch \"main\", tag \"v*\"]\n",
		},
		{
			name: "environment absent to selected shows three lines without pattern lines",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
				Desired: selected, Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
			},
			want: "  environments.production.deployment_branch_policy:\n    (environment absent)\n    -> selected [branch \"main\", tag \"v*\"]\n",
		},
		{
			name: "selected to selected with only added patterns shows only Patterns to add",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(selectedOf(branch("main"))),
				Desired: selectedOf(branch("main"), tag("v*")), Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  environments.production.deployment_branch_policy:\n    selected [branch \"main\"]\n    -> selected [branch \"main\", tag \"v*\"]\n    Patterns to add: [tag \"v*\"]\n",
		},
		{
			name: "selected to selected with only removed patterns shows only Patterns to remove",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(selectedOf(branch("main"), tag("v*"))),
				Desired: selectedOf(branch("main")), Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  environments.production.deployment_branch_policy:\n    selected [branch \"main\", tag \"v*\"]\n    -> selected [branch \"main\"]\n    Patterns to remove: [tag \"v*\"]\n",
		},
		{
			name: "selected to selected with both shows remove then add, in observed then declared order",
			s: changeplan.SettingPlan{
				Unit:     setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production},
				Observed: github.Observed(selectedOf(branch("a"), branch("main"), branch("z"))),
				Desired:  selectedOf(branch("y"), branch("main"), branch("b")), Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  environments.production.deployment_branch_policy:\n    selected [branch \"a\", branch \"main\", branch \"z\"]\n    -> selected [branch \"y\", branch \"main\", branch \"b\"]\n" +
				"    Patterns to remove: [branch \"a\", branch \"z\"]\n    Patterns to add: [branch \"y\", branch \"b\"]\n",
		},
		{
			name: "environment whose existence is unknown shows the state and why it is not applicable",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined), Desired: setting.Present(), Result: changeplan.NotComparable,
				Applicability: changeplan.NotApplicable(changeplan.NotApplicableNameUndetermined),
			},
			want: "  environments.production: (existence unknown) -> present\n" + notApplicable,
		},
		{
			name: "mode of an environment whose existence is unknown shows the same state",
			s: changeplan.SettingPlan{
				Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined),
				Desired: protected, Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicableNameUndetermined),
			},
			want: "  environments.production.deployment_branch_policy: (existence unknown) -> protected_branches\n" + notApplicable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlannedChangeText(tc.s); got != tc.want {
				t.Errorf("PlannedChangeText() =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}
