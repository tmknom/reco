package apply

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestUnexpectedOutsideChanges は、二つの更新操作からなる論理的な設定変更で、変更対象外の設定を
// その設定が影響を受け得る最初の更新操作の直前の観測と比べる規則を確かめる。
//
//trace:checks INV-001 FR-012
func TestUnexpectedOutsideChanges(t *testing.T) {
	selectedWith := func(shaPinningRequired bool, criteria setting.SelectedActionsCriteria) github.ObservationResult {
		permissions := actionsSelected(criteria)
		permissions.SHAPinningRequired = github.Observed(shaPinningRequired)
		return github.ObservationResult{ActionsPermissions: permissions}
	}
	localOnly := func(shaPinningRequired bool) github.ObservationResult {
		return github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsLocalOnly, shaPinningRequired)}
	}
	setBack := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}}
	stagingAll := observedEnvironment(7, "staging", setting.DeploymentBranchPolicyAll)
	selectedStaging := selectedEnvironment(7, "staging", nil)
	// stagingWith は、e の wait_timer を wait 分にした staging を一覧の全体とする観測。
	stagingWith := func(e github.Environment, wait int) github.ObservationResult {
		e.Protection.WaitTimer = wait
		return github.ObservationResult{Environments: environments(e)}
	}
	stagingProtected := observedEnvironment(7, "staging", setting.DeploymentBranchPolicyProtectedBranches)
	// general は、merge commit と rebase merge が有効で、ほかの General の設定と Secret scanning の2設定が無効のリポジトリの観測を、change で変えたもの。
	general := func(change func(*github.General)) github.ObservationResult {
		g := observedGeneral(github.General{AllowMergeCommit: true, AllowRebaseMerge: true})
		change(&g)
		return github.ObservationResult{General: g, SecretScanning: publicSecretScanning(false, false)}
	}
	swapped := func(g *github.General) { g.AllowSquashMerge, g.AllowMergeCommit = true, false }
	cases := []struct {
		name   string
		change changeplan.LogicalChange
		before []github.ObservationResult
		after  github.ObservationResult
		want   []unexpectedOutsideChange
	}{
		{
			name:   "sha_pinning_required is compared with the read before the first update rather than the second",
			change: toSelectedChange(),
			before: []github.ObservationResult{localOnly(false), selectedWith(true, setBack)},
			after:  selectedWith(true, declaredCriteria),
			want:   []unexpectedOutsideChange{{Setting: string(setting.SHAPinningRequired), Before: "false", After: "true"}},
		},
		{
			name:   "sha_pinning_required unavailable before the first update is not compared even if read before the second",
			change: enableSelectedChange(),
			before: []github.ObservationResult{{ActionsPermissions: actionsDisabled()}, selectedWith(false, setBack)},
			after:  selectedWith(true, declaredCriteria),
			want:   nil,
		},
		{
			name:   "only the first update started compares its endpoint with the read before it",
			change: toSelectedChange(),
			before: []github.ObservationResult{localOnly(false)},
			after:  selectedWith(true, setBack),
			want:   []unexpectedOutsideChange{{Setting: string(setting.SHAPinningRequired), Before: "false", After: "true"}},
		},
		{
			name:   "every bundled merge method is excluded from the comparison",
			change: mergeMethodSwapChange(t),
			before: []github.ObservationResult{general(func(*github.General) {})},
			after:  general(swapped),
			want:   nil,
		},
		{
			name:   "settings not bundled at the same endpoint are compared",
			change: mergeMethodSwapChange(t),
			before: []github.ObservationResult{general(func(*github.General) {})},
			after: general(func(g *github.General) {
				swapped(g)
				g.AllowAutoMerge, g.AllowRebaseMerge, g.AllowUpdateBranch = true, false, true
			}),
			want: []unexpectedOutsideChange{
				{Setting: string(setting.AllowAutoMerge), Before: "false", After: "true"},
				{Setting: string(setting.AllowRebaseMerge), Before: "true", After: "false"},
				{Setting: string(setting.AllowUpdateBranch), Before: "false", After: "true"},
			},
		},
		{
			// 計画が作らない並び。同じ更新エンドポイントの更新操作が並んでも、後の直前の観測を基準にしないことを確かめる
			name: "two updates to the same endpoint compare only with the read before the first of them",
			change: changeplan.LogicalChange{DesiredState: allowedActionsChange().DesiredState, Operations: []changeplan.UpdateOperation{
				allowedActionsChange().Operations[0], allowedActionsChange().Operations[0],
			}},
			before: []github.ObservationResult{localOnly(false), localOnly(true)},
			after:  localOnly(false),
			want:   nil,
		},
		{
			// Environment の更新操作は UpdateEndpointOf で引けないため、引くと panic で失敗する
			name:   "mode change of an environment returns the changed protection but not the mode it changes",
			change: modeChange(t, "staging", observedEnvironment(7, "staging", setting.DeploymentBranchPolicyAll), setting.DeploymentBranchPolicyProtectedBranches),
			before: []github.ObservationResult{{Environments: environments(observedEnvironment(7, "staging", setting.DeploymentBranchPolicyAll))}},
			after: github.ObservationResult{Environments: environments(func() github.Environment {
				e := observedEnvironment(7, "staging", setting.DeploymentBranchPolicyProtectedBranches)
				e.Protection.CanAdminsBypass = true
				return e
			}())},
			want: []unexpectedOutsideChange{{Setting: "environments.staging.can_admins_bypass", Before: "false", After: "true"}},
		},
		{
			// 個別パターンの更新操作も Environment を宛先とするため、それぞれの直前と比べると変化が更新操作の数だけ並ぶ
			name:   "mode update and two pattern additions compare the protection only with the read before the mode update",
			change: deploymentPatternsChange(t, "staging", stagingAll, selectedPolicy(branchPattern("main"), tagPattern("v*"))),
			before: []github.ObservationResult{stagingWith(stagingAll, 30), stagingWith(selectedStaging, 30), stagingWith(selectedStaging, 30)},
			after:  stagingWith(selectedStaging, 10),
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.wait_timer", Before: "30 minutes", After: "10 minutes"}},
		},
		{
			name:   "reviewers update does not return the reviewers it changes",
			change: reviewersChange(t, "staging", withReviewers(stagingAll, false, 1), reviewerSet(2)),
			before: []github.ObservationResult{{Environments: environments(withReviewers(stagingAll, false, 1))}},
			after:  github.ObservationResult{Environments: environments(withReviewers(stagingAll, false, 2))},
			want:   nil,
		},
		{
			// Environment の PUT は方式も受け付けるため、保護設定より前に方式を比べる
			name:   "reviewers update returns the changed mode before the changed wait_timer",
			change: reviewersChange(t, "staging", withReviewers(stagingAll, false, 1), reviewerSet(2)),
			before: []github.ObservationResult{stagingWith(withReviewers(stagingAll, false, 1), 30)},
			after:  stagingWith(withReviewers(stagingProtected, false, 2), 10),
			want: []unexpectedOutsideChange{
				{Setting: "environments.staging.deployment_branch_policy", Before: "all", After: "protected_branches"},
				{Setting: "environments.staging.wait_timer", Before: "30 minutes", After: "10 minutes"},
			},
		},
		{
			name:   "reviewers update returns the changed wait_timer alone",
			change: reviewersChange(t, "staging", withReviewers(stagingAll, false, 1), reviewerSet(2)),
			before: []github.ObservationResult{stagingWith(withReviewers(stagingAll, false, 1), 30)},
			after:  stagingWith(withReviewers(stagingAll, false, 2), 10),
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.wait_timer", Before: "30 minutes", After: "10 minutes"}},
		},
		{
			name:   "wait_timer update returns the changed mode",
			change: preventSelfReviewOrWaitTimerChange(t, "staging", stagingAll, setting.EnvironmentWaitTimer, setting.WaitTimerValue(30)),
			before: []github.ObservationResult{{Environments: environments(stagingAll)}},
			after:  stagingWith(stagingProtected, 30),
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.deployment_branch_policy", Before: "all", After: "protected_branches"}},
		},
		{
			name:   "prevent_self_review update returns the changed wait_timer",
			change: preventSelfReviewOrWaitTimerChange(t, "staging", withReviewers(stagingAll, false, 1), setting.EnvironmentPreventSelfReview, setting.BoolValue(true)),
			before: []github.ObservationResult{stagingWith(withReviewers(stagingAll, false, 1), 30)},
			after:  stagingWith(withReviewers(stagingAll, true, 1), 10),
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.wait_timer", Before: "30 minutes", After: "10 minutes"}},
		},
		{
			name:   "prevent_self_review update does not return the prevent_self_review it changes",
			change: preventSelfReviewOrWaitTimerChange(t, "staging", withReviewers(stagingAll, false, 1), setting.EnvironmentPreventSelfReview, setting.BoolValue(true)),
			before: []github.ObservationResult{{Environments: environments(withReviewers(stagingAll, false, 1))}},
			after:  github.ObservationResult{Environments: environments(withReviewers(stagingAll, true, 1))},
			want:   nil,
		},
		{
			// 個別パターンは別の更新エンドポイントの設定で、方式の名前だけを比べる
			name:   "reviewers update does not compare the patterns of the same mode",
			change: reviewersChange(t, "staging", withReviewers(selectedStaging, false, 1), reviewerSet(2)),
			before: []github.ObservationResult{{Environments: environments(withReviewers(selectedEnvironment(7, "staging", []setting.DeploymentPattern{branchPattern("main")}, 10), false, 1))}},
			after: github.ObservationResult{Environments: environments(withReviewers(
				selectedEnvironment(7, "staging", []setting.DeploymentPattern{branchPattern("main"), branchPattern("release/*")}, 10, 11), false, 2,
			))},
			want: nil,
		},
		{
			name:   "reviewers update keeping the rule returns the changed prevent_self_review",
			change: reviewersChange(t, "staging", withReviewers(stagingAll, true, 1), reviewerSet(2)),
			before: []github.ObservationResult{{Environments: environments(withReviewers(stagingAll, true, 1))}},
			after:  github.ObservationResult{Environments: environments(withReviewers(stagingAll, false, 2))},
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.prevent_self_review", Before: "true", After: "false"}},
		},
		{
			// 失われた自己承認の禁止は Side effect: で示した変化で、予期しない変化に当たらない
			name:   "removal of all reviewers does not return prevent_self_review discarded with the rule",
			change: reviewersChange(t, "staging", withReviewers(stagingAll, true, 1), reviewerSet()),
			before: []github.ObservationResult{{Environments: environments(withReviewers(stagingAll, true, 1))}},
			after:  github.ObservationResult{Environments: environments(stagingAll)},
			want:   nil,
		},
		{
			name:   "mode change returns the changed reviewers",
			change: modeChange(t, "staging", stagingAll, setting.DeploymentBranchPolicyProtectedBranches),
			before: []github.ObservationResult{{Environments: environments(withReviewers(stagingAll, false, 1))}},
			after:  github.ObservationResult{Environments: environments(withReviewers(observedEnvironment(7, "staging", setting.DeploymentBranchPolicyProtectedBranches), false, 2))},
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.reviewers", Before: "[User 1]", After: "[User 2]"}},
		},
		{
			name:   "pattern removal and addition do not compare the protection",
			change: deploymentPatternsChange(t, "staging", selectedEnvironment(7, "staging", []setting.DeploymentPattern{branchPattern("feature/*")}, 11), selectedPolicy(branchPattern("main"))),
			before: []github.ObservationResult{stagingWith(selectedStaging, 30), stagingWith(selectedStaging, 30)},
			after:  stagingWith(selectedStaging, 10),
			want:   nil,
		},
		{
			name: "push protection shown as a side effect of disabling secret scanning is not compared",
			change: func() changeplan.LogicalChange {
				change := singleUpdate(setting.SecretScanningEnabled, setting.BoolValue(false), basesOf(setting.SecretScanningEnabled, setting.BoolValue(true)))
				change.SideEffectSettings = []setting.Unit{{Key: setting.RepositoryPushProtection}}
				return change
			}(),
			before: []github.ObservationResult{{General: observedGeneral(github.General{}), SecretScanning: publicSecretScanning(true, true)}},
			after:  github.ObservationResult{General: observedGeneral(github.General{}), SecretScanning: publicSecretScanning(false, false)},
			want:   nil,
		},
		{
			name:   "push protection not shown as a side effect of disabling secret scanning is compared",
			change: singleUpdate(setting.SecretScanningEnabled, setting.BoolValue(false), append(basesOf(setting.SecretScanningEnabled, setting.BoolValue(true)), basesOf(setting.RepositoryPushProtection, setting.BoolValue(false))...)),
			before: []github.ObservationResult{{General: observedGeneral(github.General{}), SecretScanning: publicSecretScanning(true, false)}},
			after:  github.ObservationResult{General: observedGeneral(github.General{}), SecretScanning: publicSecretScanning(false, true)},
			want:   []unexpectedOutsideChange{{Setting: string(setting.RepositoryPushProtection), Before: "false", After: "true"}},
		},
		{
			name:   "policy and criteria changed by the logical change are not compared",
			change: toSelectedChange(),
			before: []github.ObservationResult{localOnly(false), selectedWith(false, setBack)},
			after:  selectedWith(false, declaredCriteria),
			want:   nil,
		},
		{
			// 同じ一覧のほかの変数は、変数の更新エンドポイントの変更対象外の設定に当たらない
			name:   "variable value change does not compare other variables of the same list",
			change: variableValueChange(t, "GO_VERSION", "1.26", "1.27"),
			before: []github.ObservationResult{{RepositoryVariables: variables(github.Variable{Name: "GO_VERSION", Value: "1.26"}, github.Variable{Name: "OTHER", Value: "a"})}},
			after: github.ObservationResult{RepositoryVariables: variables(
				github.Variable{Name: "GO_VERSION", Value: "1.27"}, github.Variable{Name: "OTHER", Value: "b"}, github.Variable{Name: "ADDED", Value: "c"},
			)},
			want: nil,
		},
		{
			// 変数の更新エンドポイントは変数ごとに別で、Environment の保護設定もほかの変数も変えない
			name: "environment variable value change does not compare the protection or other variables of the environment",
			change: environmentVariableValueChange(t, "staging",
				withVariables(stagingAll, github.Variable{Name: "REGION", Value: "a"}), "REGION", "a", "b"),
			before: []github.ObservationResult{stagingWith(withVariables(stagingAll, github.Variable{Name: "REGION", Value: "a"}, github.Variable{Name: "OTHER", Value: "x"}), 30)},
			after:  stagingWith(withVariables(stagingAll, github.Variable{Name: "REGION", Value: "b"}, github.Variable{Name: "OTHER", Value: "y"}), 10),
			want:   nil,
		},
		{
			name:   "environment variable creation does not compare the protection of the environment",
			change: createdEnvironmentVariableChange(t, "staging", "REGION", "b"),
			before: []github.ObservationResult{stagingWith(withVariables(stagingAll), 30)},
			after:  stagingWith(withVariables(stagingAll, github.Variable{Name: "REGION", Value: "b"}), 10),
			want:   nil,
		},
		{
			name:   "oidc subject update includes the change of the omitted item",
			change: oidcSubjectChange(),
			before: []github.ObservationResult{oidcSubjectObservation(false)},
			after:  oidcSubjectObservation(true),
			want:   []unexpectedOutsideChange{{Setting: "actions.oidc_subject.use_immutable_subject", Before: "false", After: "true"}},
		},
		{
			// Ruleset の更新エンドポイントは Ruleset の値だけを受け付けるため、同じ観測のほかの設定が変わっても比べない
			name:   "ruleset creation compares no setting",
			change: rulesetCreationChange(t, "protect"),
			before: []github.ObservationResult{general(func(*github.General) {})},
			after:  general(func(g *github.General) { g.AllowAutoMerge = true }),
			want:   nil,
		},
		{
			// 値の全体を置き換えるため、宛先の Ruleset の値は期待状態の確認が比べる
			name:   "ruleset value update compares no setting",
			change: rulesetValueUpdateChange(t, "protect"),
			before: []github.ObservationResult{withRulesets(general(func(*github.General) {}), statusChecksRulesetObservation(false))},
			after:  withRulesets(general(func(g *github.General) { g.AllowAutoMerge = true }), statusChecksRulesetObservation(true)),
			want:   nil,
		},
		{
			// 削除は値を送らず、更新後に比べる値がない。消えた Ruleset の値を比べる実装は panic するか変化を返す
			name:   "ruleset deletion compares no setting",
			change: rulesetDeletionChange(t, "protect"),
			before: []github.ObservationResult{withRulesets(general(func(*github.General) {}), statusChecksRulesetObservation(false))},
			after: withRulesets(general(func(g *github.General) { g.AllowAutoMerge = true }),
				github.ObservationResult{Rulesets: &github.Rulesets{Existing: github.Observed([]github.Ruleset{})}}),
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := unexpectedOutsideChanges(tc.change, tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("unexpectedOutsideChanges() = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("undefined kind of an update operation panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("unexpectedOutsideChanges() did not panic")
			}
		}()
		change := allowAutoMergeChange()
		change.Operations[0].Kind = changeplan.RulesetDeletion + 1
		unexpectedOutsideChanges(change, []github.ObservationResult{{}}, github.ObservationResult{})
	})
}

// TestOIDCSubjectOmittedItemChanges は、更新操作の値が省略した項目を、更新直前と更新後の観測値で比べる規則を確かめる。
//
//trace:checks INV-001 FR-012
func TestOIDCSubjectOmittedItemChanges(t *testing.T) {
	observed := oidcSubjectObservation
	omitting := oidcSubjectChange().Operations[0]
	declaring := omitting
	declaring.Value = setting.OIDCSubjectCustomizationValue(setting.OIDCSubjectCustomization{UseDefault: true, UseImmutableSubject: new(true)})
	cases := []struct {
		name      string
		operation changeplan.UpdateOperation
		before    github.ObservationResult
		after     github.ObservationResult
		want      []unexpectedOutsideChange
	}{
		{
			name:      "omitted item changed from false to true returns the change",
			operation: omitting,
			before:    observed(false),
			after:     observed(true),
			want:      []unexpectedOutsideChange{{Setting: "actions.oidc_subject.use_immutable_subject", Before: "false", After: "true"}},
		},
		{
			name:      "omitted item kept returns nothing",
			operation: omitting,
			before:    observed(true),
			after:     observed(true),
			want:      nil,
		},
		{
			name:      "declared item changed returns nothing",
			operation: declaring,
			before:    observed(false),
			after:     observed(true),
			want:      nil,
		},
		{
			// OIDCSubject を読まない観測で Key を確かめずに読むと、nil の参照で止まる
			name:      "update of another setting returns nothing",
			operation: allowAutoMergeChange().Operations[0],
			before:    github.ObservationResult{},
			after:     github.ObservationResult{},
			want:      nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := oidcSubjectOmittedItemChanges(tc.operation, tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("oidcSubjectOmittedItemChanges() = %v, want %v", got, tc.want)
			}
		})
	}
}

// statusChecksRulesetValue は、do_not_enforce_on_create が doNotEnforceOnCreate の required_status_checks のルールだけを持つ Ruleset の値。
func statusChecksRulesetValue(doNotEnforceOnCreate bool) setting.Ruleset {
	return setting.Ruleset{
		Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
		Conditions:   setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{}}},
		BypassActors: []setting.BypassActor{},
		Rules:        []setting.RulesetRule{setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{DoNotEnforceOnCreate: doNotEnforceOnCreate})},
	}
}

// statusChecksRulesetObservation は、protect を、do_not_enforce_on_create が doNotEnforceOnCreate の値として読んだ観測。
func statusChecksRulesetObservation(doNotEnforceOnCreate bool) github.ObservationResult {
	ruleset := github.InterpretedRuleset(42, "protect", statusChecksRulesetValue(doNotEnforceOnCreate))
	return github.ObservationResult{Rulesets: &github.Rulesets{Existing: github.Observed([]github.Ruleset{ruleset})}}
}

// withRulesets は、observed に、rulesets の Rulesets の観測を加える。
func withRulesets(observed, rulesets github.ObservationResult) github.ObservationResult {
	observed.Rulesets = rulesets.Rulesets
	return observed
}

// TestOutsideChangesAt は、論理的な設定変更が変えない設定を、管理範囲によらず比べる規則を確かめる。
//
//trace:checks INV-001 FR-012
func TestOutsideChangesAt(t *testing.T) {
	cases := []struct {
		name          string
		change        changeplan.LogicalChange
		endpoint      github.UpdateEndpoint
		before, after github.ObservationResult
		want          []unexpectedOutsideChange
	}{
		{
			name:     "same settings have no change",
			change:   allowAutoMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: observedGeneral(github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false}), SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: observedGeneral(github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false}), SecretScanning: publicSecretScanning(true, true)},
			want:     nil,
		},
		{
			name:     "allow_auto_merge change reports different delete_branch_on_merge with both values",
			change:   allowAutoMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: observedGeneral(github.General{DeleteBranchOnMerge: false}), SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: observedGeneral(github.General{DeleteBranchOnMerge: true}), SecretScanning: publicSecretScanning(true, true)},
			want:     []unexpectedOutsideChange{{Setting: string(setting.DeleteBranchOnMerge), Before: "false", After: "true"}},
		},
		{
			name:     "allow_auto_merge change does not report its own setting",
			change:   allowAutoMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: observedGeneral(github.General{AllowAutoMerge: false}), SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: observedGeneral(github.General{AllowAutoMerge: true}), SecretScanning: publicSecretScanning(true, true)},
			want:     nil,
		},
		{
			name:     "delete_branch_on_merge change reports different allow_auto_merge with both values",
			change:   deleteBranchOnMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: observedGeneral(github.General{AllowAutoMerge: false}), SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: observedGeneral(github.General{AllowAutoMerge: true}), SecretScanning: publicSecretScanning(true, true)},
			want:     []unexpectedOutsideChange{{Setting: string(setting.AllowAutoMerge), Before: "false", After: "true"}},
		},
		{
			name:     "delete_branch_on_merge change does not report its own setting",
			change:   deleteBranchOnMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: observedGeneral(github.General{DeleteBranchOnMerge: false}), SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: observedGeneral(github.General{DeleteBranchOnMerge: true}), SecretScanning: publicSecretScanning(true, true)},
			want:     nil,
		},
		{
			name:     "default_workflow_permissions change reports different can_approve_pull_request_reviews with both values",
			change:   defaultWorkflowPermissionsChange(),
			endpoint: github.WorkflowPermissionsEndpoint,
			before:   github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(setting.PermissionWrite), CanApprovePullRequestReviews: false}},
			after:    github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(setting.PermissionRead), CanApprovePullRequestReviews: true}},
			want:     []unexpectedOutsideChange{{Setting: string(setting.CanApprovePullRequestReviews), Before: "false", After: "true"}},
		},
		{
			name:     "can_approve_pull_request_reviews change reports different default_workflow_permissions with both values",
			change:   canApprovePullRequestReviewsChange(),
			endpoint: github.WorkflowPermissionsEndpoint,
			before:   github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(setting.PermissionRead), CanApprovePullRequestReviews: true}},
			after:    github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(setting.PermissionWrite), CanApprovePullRequestReviews: false}},
			want: []unexpectedOutsideChange{{
				Setting: string(setting.DefaultWorkflowPermissions),
				Before:  "read",
				After:   "write",
			}},
		},
		{
			name:     "general change reports different secret scanning settings at the same endpoint",
			change:   allowAutoMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: observedGeneral(github.General{}), SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: observedGeneral(github.General{}), SecretScanning: publicSecretScanning(false, false)},
			want: []unexpectedOutsideChange{
				{Setting: string(setting.SecretScanningEnabled), Before: "true", After: "false"},
				{Setting: string(setting.RepositoryPushProtection), Before: "true", After: "false"},
			},
		},
		{
			name:     "secret scanning change reports different general settings and the other secret scanning setting",
			change:   singleUpdate(setting.RepositoryPushProtection, setting.BoolValue(false), basesOf(setting.RepositoryPushProtection, setting.BoolValue(true))),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: observedGeneral(github.General{AllowAutoMerge: false}), SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: observedGeneral(github.General{AllowAutoMerge: true}), SecretScanning: publicSecretScanning(false, false)},
			want: []unexpectedOutsideChange{
				{Setting: string(setting.AllowAutoMerge), Before: "false", After: "true"},
				{Setting: string(setting.SecretScanningEnabled), Before: "true", After: "false"},
			},
		},
		{
			name:     "workflow permissions change does not compare general settings",
			change:   defaultWorkflowPermissionsChange(),
			endpoint: github.WorkflowPermissionsEndpoint,
			before: github.ObservationResult{
				General:             github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false},
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(setting.PermissionWrite)},
			},
			after: github.ObservationResult{
				General:             github.General{AllowAutoMerge: true, DeleteBranchOnMerge: true},
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(setting.PermissionRead)},
			},
			want: nil,
		},
		{
			name:     "general change does not compare workflow permissions settings",
			change:   allowAutoMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before: github.ObservationResult{
				General:             observedGeneral(github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false}),
				SecretScanning:      publicSecretScanning(true, true),
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(setting.PermissionWrite), CanApprovePullRequestReviews: false},
			},
			after: github.ObservationResult{
				General:             observedGeneral(github.General{AllowAutoMerge: true, DeleteBranchOnMerge: false}),
				SecretScanning:      publicSecretScanning(true, true),
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(setting.PermissionRead), CanApprovePullRequestReviews: true},
			},
			want: nil,
		},
		{
			name:     "allowed_actions change reports different sha_pinning_required with both values",
			change:   allowedActionsChange(),
			endpoint: github.ActionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsAll, false)},
			after:    github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsLocalOnly, true)},
			want:     []unexpectedOutsideChange{{Setting: string(setting.SHAPinningRequired), Before: "false", After: "true"}},
		},
		{
			// 比べられない値を変化なしと推測しない。完了にしないことは、期待状態の確認が観測不能になることで保たれる
			name:     "sha_pinning_required unavailable after the update is not reported",
			change:   allowedActionsChange(),
			endpoint: github.ActionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsAll, false)},
			after:    github.ObservationResult{ActionsPermissions: actionsDisabled()},
			want:     nil,
		},
		{
			name:     "sha_pinning_required unavailable before the update is not reported",
			change:   allowedActionsChange(),
			endpoint: github.ActionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsDisabled()},
			after:    github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsLocalOnly, true)},
			want:     nil,
		},
		{
			name:     "sha_pinning_required unavailable both before and after the update is not reported",
			change:   allowedActionsChange(),
			endpoint: github.ActionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsDisabled()},
			after:    github.ObservationResult{ActionsPermissions: actionsDisabled()},
			want:     nil,
		},
		{
			// 観測不能を含む組の比べ方は TestObservationChange が確かめ、ここでは設定の観測結果がその判定を通ることだけを確かめる
			name:     "allowed_actions of an unknown value before the update and observed after it is reported as unknown value",
			change:   shaPinningRequiredChange(),
			endpoint: github.ActionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsWithUnknownPolicy(false)},
			after:    github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsAll, true)},
			want:     []unexpectedOutsideChange{{Setting: string(setting.AllowedActions), Before: "(unknown value)", After: "all"}},
		},
		{
			name:     "selected_actions change does not compare allowed_actions or sha_pinning_required",
			change:   selectedActionsChange(),
			endpoint: github.SelectedActionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsSelected(setting.SelectedActionsCriteria{})},
			after:    github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsAll, true)},
			want:     nil,
		},
		{
			// 値の保全は、GitHub が省略した値を保持するという確認結果を根拠にする
			name:     "disabling actions does not report allowed_actions or sha_pinning_required unavailable after the update",
			change:   enabledStateChange(false),
			endpoint: github.ActionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsAll, true)},
			after:    github.ObservationResult{ActionsPermissions: actionsDisabled()},
			want:     nil,
		},
		{
			name:     "disabling actions not reflected reports different sha_pinning_required with both values",
			change:   enabledStateChange(false),
			endpoint: github.ActionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsAll, false)},
			after:    github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsAll, true)},
			want:     []unexpectedOutsideChange{{Setting: string(setting.SHAPinningRequired), Before: "false", After: "true"}},
		},
		{
			// apply の実行からは到達しない組。選択方式の Actions 許可条件を変更対象外の設定にする更新エンドポイントがないため、
			// 比較が値の判定を通ることだけを、一覧に依らない入力で確かめる
			name:     "outside criteria differing only in the order of patterns are not reported",
			change:   allowedActionsChange(),
			endpoint: github.SelectedActionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsSelected(setting.SelectedActionsCriteria{PatternsAllowed: []string{"a/*", "b/*"}})},
			after:    github.ObservationResult{ActionsPermissions: actionsSelected(setting.SelectedActionsCriteria{PatternsAllowed: []string{"b/*", "a/*"}})},
			want:     nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := outsideChangesAt(tc.endpoint, tc.change.ExcludedFromOutsideSettings(), tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("outsideChangesAt() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEnvironmentPutOutsideChanges は、Environment の PUT の更新操作の変更対象外の設定を、その名前の Environment のデプロイ元制限の方式と保護設定に限って比べる規則を確かめる。
// 個別パターンを比べないことは TestUnexpectedOutsideChanges が確かめる。
//
//trace:checks INV-001 FR-012
func TestEnvironmentPutOutsideChanges(t *testing.T) {
	staging := mustEnvironmentName(t, "staging")
	// observedWith は、方式 mode と保護設定 protection を持つ綴り Staging の Environment と others を一覧の全体とする観測。
	observedWith := func(mode setting.DeploymentBranchPolicyMode, protection github.EnvironmentProtection, others ...github.Environment) github.ObservationResult {
		e := observedEnvironment(7, "Staging", mode)
		e.Protection = protection
		return github.ObservationResult{Environments: environments(append(others, e)...)}
	}
	observed := func(protection github.EnvironmentProtection, others ...github.Environment) github.ObservationResult {
		return observedWith(setting.DeploymentBranchPolicyAll, protection, others...)
	}
	base := github.EnvironmentProtection{WaitTimer: 30, ReviewerSet: github.Observed(reviewerRule(1, 2)), PreventSelfReview: true, CanAdminsBypass: true}
	stagingReviewers := setting.Unit{Key: setting.EnvironmentReviewerSet, Environment: staging}
	stagingPolicy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}
	// changedReviewers は、base の担当者と can_admins_bypass を変えた保護設定
	changedReviewers := github.EnvironmentProtection{WaitTimer: 30, ReviewerSet: github.Observed(reviewerRule(3)), PreventSelfReview: true, CanAdminsBypass: false}
	cases := []struct {
		name          string
		excluded      []setting.Unit
		before, after github.ObservationResult
		want          []unexpectedOutsideChange
	}{
		{
			name:     "reviewers changed by the logical change is not compared",
			excluded: []setting.Unit{stagingReviewers},
			before:   observed(base),
			after:    observed(changedReviewers),
			want:     []unexpectedOutsideChange{{Setting: "environments.staging.can_admins_bypass", Before: "true", After: "false"}},
		},
		{
			name:     "reviewers of another environment does not exclude the item",
			excluded: []setting.Unit{{Key: setting.EnvironmentReviewerSet, Environment: mustEnvironmentName(t, "production")}},
			before:   observed(base),
			after:    observed(changedReviewers),
			want: []unexpectedOutsideChange{
				{Setting: "environments.staging.reviewers", Before: "[User 1, User 2]", After: "[User 3]"},
				{Setting: "environments.staging.can_admins_bypass", Before: "true", After: "false"},
			},
		},
		{
			// 同じ Environment でも形の違う単位は、保護設定の項目を除かない
			name:     "another setting of the same environment does not exclude the items",
			excluded: []setting.Unit{stagingPolicy},
			before:   observed(base),
			after:    observed(changedReviewers),
			want: []unexpectedOutsideChange{
				{Setting: "environments.staging.reviewers", Before: "[User 1, User 2]", After: "[User 3]"},
				{Setting: "environments.staging.can_admins_bypass", Before: "true", After: "false"},
			},
		},
		{
			name:   "changed mode is named under the declared environment name with the mode names before the protection",
			before: observed(base),
			after:  observedWith(setting.DeploymentBranchPolicyProtectedBranches, github.EnvironmentProtection{WaitTimer: 10, ReviewerSet: github.Observed(reviewerRule(1, 2)), PreventSelfReview: true, CanAdminsBypass: true}),
			want: []unexpectedOutsideChange{
				{Setting: "environments.staging.deployment_branch_policy", Before: "all", After: "protected_branches"},
				{Setting: "environments.staging.wait_timer", Before: "30 minutes", After: "10 minutes"},
			},
		},
		{
			name:     "mode changed by the logical change is not compared",
			excluded: []setting.Unit{stagingPolicy},
			before:   observed(base),
			after:    observedWith(setting.DeploymentBranchPolicyProtectedBranches, base),
			want:     nil,
		},
		{
			// 方式の単位だけを除き、保護設定の項目の単位では除かない
			name:     "reviewers changed by the logical change does not exclude the mode",
			excluded: []setting.Unit{stagingReviewers},
			before:   observed(base),
			after:    observedWith(setting.DeploymentBranchPolicySelected, base),
			want:     []unexpectedOutsideChange{{Setting: "environments.staging.deployment_branch_policy", Before: "all", After: "selected"}},
		},
		{
			name:   "changed mode of another environment is not compared",
			before: observed(base, observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll)),
			after:  observed(base, observedEnvironment(8, "production", setting.DeploymentBranchPolicySelected)),
			want:   nil,
		},
		{
			name:   "absent before the update is not compared",
			before: github.ObservationResult{Environments: environments()},
			after:  observed(base),
			want:   nil,
		},
		{
			name:   "absent after the update is not compared",
			before: observed(base),
			after:  github.ObservationResult{Environments: environments()},
			want:   nil,
		},
		{
			name:   "undetermined after the update is not compared",
			before: observed(base),
			after:  github.ObservationResult{Environments: environments(observedEnvironment(9, "stagïng", setting.DeploymentBranchPolicyAll))},
			want:   nil,
		},
		{
			name:   "changed protection of another environment is not compared",
			before: observed(base, observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll)),
			after: observed(base, func() github.Environment {
				e := observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll)
				e.Protection.CanAdminsBypass = true
				return e
			}()),
			want: nil,
		},
		{
			name:   "a changed item is named under the declared environment name",
			before: observed(base),
			after:  observed(github.EnvironmentProtection{ReviewerSet: github.Observed(reviewerRule(1, 2)), PreventSelfReview: true, CanAdminsBypass: true}),
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.wait_timer", Before: "30 minutes", After: "0 minutes"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := environmentPutOutsideChanges(staging, tc.excluded, tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("environmentPutOutsideChanges() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEnvironmentProtectionChanges は、Environment の保護設定の項目ごとの差を、CLI 仕様の変更対象外の設定の名前と値の表記で返す規則を確かめる。
// 観測不能を含む組の比べ方は TestObservationChange が確かめ、ここでは担当者の集合がその判定を通ることだけを確かめる。
//
//trace:checks INV-001 FR-012
func TestEnvironmentProtectionChanges(t *testing.T) {
	staging := mustEnvironmentName(t, "staging")
	unknown := github.Unavailable[setting.ReviewerSet](github.UnavailableUnknownValue)
	noRule := github.Observed(setting.ReviewerSetOfRule(nil))
	base := github.EnvironmentProtection{WaitTimer: 30, ReviewerSet: github.Observed(reviewerRule(1, 2)), PreventSelfReview: true, CanAdminsBypass: true}
	// with は、base を change で変えた保護設定。
	with := func(change func(*github.EnvironmentProtection)) github.EnvironmentProtection {
		p := base
		change(&p)
		return p
	}
	cases := []struct {
		name          string
		excluded      []setting.Unit
		before, after github.EnvironmentProtection
		want          []unexpectedOutsideChange
	}{
		{
			name:     "excluded items are not compared while the other items are",
			excluded: []setting.Unit{{Key: setting.EnvironmentWaitTimer, Environment: staging}, {Key: setting.EnvironmentPreventSelfReview, Environment: staging}},
			before:   base,
			after:    github.EnvironmentProtection{WaitTimer: 5, ReviewerSet: github.Observed(reviewerRule(1, 2)), CanAdminsBypass: false},
			want:     []unexpectedOutsideChange{{Setting: "environments.staging.can_admins_bypass", Before: "true", After: "false"}},
		},
		{
			name:   "same protection with reviewers only reordered has no change",
			before: base,
			after:  with(func(p *github.EnvironmentProtection) { p.ReviewerSet = github.Observed(reviewerRule(2, 1)) }),
		},
		{
			name: "reviewers are shown without logins",
			before: with(func(p *github.EnvironmentProtection) {
				p.ReviewerSet = github.Observed(reviewerSetWithLogin(1, "octocat"))
			}),
			after: with(func(p *github.EnvironmentProtection) {
				p.ReviewerSet = github.Observed(reviewerSetWithLogin(2, "hubot"))
			}),
			want: []unexpectedOutsideChange{{Setting: "environments.staging.reviewers", Before: "[User 1]", After: "[User 2]"}},
		},
		{
			name:   "no rule is the empty set",
			before: base,
			after:  with(func(p *github.EnvironmentProtection) { p.ReviewerSet = noRule }),
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.reviewers", Before: "[User 1, User 2]", After: "[]"}},
		},
		{
			name:   "all four changed are four changes in the order wait_timer, reviewers, prevent_self_review, can_admins_bypass",
			before: base,
			after:  github.EnvironmentProtection{WaitTimer: 5, ReviewerSet: github.Observed(reviewerRule(1))},
			want: []unexpectedOutsideChange{
				{Setting: "environments.staging.wait_timer", Before: "30 minutes", After: "5 minutes"},
				{Setting: "environments.staging.reviewers", Before: "[User 1, User 2]", After: "[User 1]"},
				{Setting: "environments.staging.prevent_self_review", Before: "true", After: "false"},
				{Setting: "environments.staging.can_admins_bypass", Before: "true", After: "false"},
			},
		},
		{
			name:   "reviewers of an unknown value before the update and observed after it are reported as unknown value",
			before: with(func(p *github.EnvironmentProtection) { p.ReviewerSet = unknown }),
			after:  with(func(p *github.EnvironmentProtection) { p.WaitTimer = 10 }),
			want: []unexpectedOutsideChange{
				{Setting: "environments.staging.wait_timer", Before: "30 minutes", After: "10 minutes"},
				{Setting: "environments.staging.reviewers", Before: "(unknown value)", After: "[User 1, User 2]"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := environmentProtectionChanges(staging, tc.excluded, tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("environmentProtectionChanges() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestObservationChange は、変更対象外の設定の更新の前後の観測結果が予期しない変化かを判定する規則を確かめる。
// 知らない値は reco の知るどの値とも異なるため、観測値との差は値が変わったことを表す（ドメインモデルの変更計画・論理的な設定変更・変更結果）。
//
//trace:checks INV-001 FR-012
func TestObservationChange(t *testing.T) {
	unknown := github.Unavailable[bool](github.UnavailableUnknownValue)
	disabled := github.Unavailable[bool](github.UnavailableActionsDisabled)
	cases := []struct {
		name                  string
		before, after         github.SettingObservation[bool]
		wantBefore, wantAfter string
		wantChanged           bool
	}{
		{name: "same observed values are not a change", before: github.Observed(true), after: github.Observed(true), wantBefore: "true", wantAfter: "true"},
		{name: "different observed values are a change", before: github.Observed(true), after: github.Observed(false), wantBefore: "true", wantAfter: "false", wantChanged: true},
		{name: "an unknown value before and an observed value after are a change", before: unknown, after: github.Observed(true), wantBefore: "(unknown value)", wantAfter: "true", wantChanged: true},
		{name: "an observed value before and an unknown value after are a change", before: github.Observed(true), after: unknown, wantBefore: "true", wantAfter: "(unknown value)", wantChanged: true},
		{name: "unknown values on both sides are not compared", before: unknown, after: unknown},
		{name: "an unknown value and another reason are not compared", before: unknown, after: disabled},
		{name: "another reason and an observed value are not compared", before: disabled, after: github.Observed(true)},
		{name: "an observed value and another reason are not compared", before: github.Observed(true), after: disabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before, after, changed := observationChange(tc.before, tc.after, func(a, b bool) bool { return a == b }, strconv.FormatBool)
			if before != tc.wantBefore || after != tc.wantAfter || changed != tc.wantChanged {
				t.Errorf("observationChange() = (%q, %q, %t), want (%q, %q, %t)", before, after, changed, tc.wantBefore, tc.wantAfter, tc.wantChanged)
			}
		})
	}
}

// reviewerSetWithLogin は、種類が利用者で ID が id の担当者に login を添えた、required_reviewers の保護ルールの担当者の集合。
func reviewerSetWithLogin(id int64, login string) setting.ReviewerSet {
	reviewer := setting.Reviewer{Type: setting.ReviewerUser, ID: id}
	return setting.ReviewerSetOfRule(&setting.RequiredReviewers{Reviewers: []setting.Reviewer{reviewer}, Logins: map[setting.Reviewer]string{reviewer: login}})
}

// observedGeneral は、g に GitHub の既定のマージメッセージと Web コミットのサインオフ必須の観測値を足した観測を返す。
// 観測はこれらを必ず観測値か観測不能にするため、どれも持たない General は観測として生じない。
func observedGeneral(g github.General) github.General {
	g.SquashMergeCommitDefault = github.Observed(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitleCommitOrPRTitle, Message: setting.SquashMergeCommitMessageCommitMessages})
	g.MergeCommitDefault = github.Observed(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle})
	g.WebCommitSignoffRequired = github.Observed(false)
	return g
}
