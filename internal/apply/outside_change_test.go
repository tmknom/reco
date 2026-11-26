package apply

import (
	"reflect"
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
		e.Protection.WaitTimer = &wait
		return github.ObservationResult{Environments: environments(e)}
	}
	// general は、merge commit と rebase merge が有効で、ほかの General の設定と Secret scanning の2設定が無効のリポジトリの観測を、change で変えたもの。
	general := func(change func(*github.General)) github.ObservationResult {
		g := github.General{AllowMergeCommit: true, AllowRebaseMerge: true}
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
			name:   "mode change of an environment compares only the protection of the environment",
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
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.wait_timer", Before: "30", After: "10"}},
		},
		{
			name:   "reviewers update does not return the reviewers it changes",
			change: reviewersChange(t, "staging", withReviewers(stagingAll, false, 1), reviewerSet(2)),
			before: []github.ObservationResult{{Environments: environments(withReviewers(stagingAll, false, 1))}},
			after:  github.ObservationResult{Environments: environments(withReviewers(stagingAll, false, 2))},
			want:   nil,
		},
		{
			// 担当者の更新エンドポイントは方式も受け付けるため、保護設定より前に方式を比べる
			name:   "reviewers update returns the changed mode before the changed wait_timer",
			change: reviewersChange(t, "staging", withReviewers(stagingAll, false, 1), reviewerSet(2)),
			before: []github.ObservationResult{stagingWith(withReviewers(stagingAll, false, 1), 30)},
			after:  stagingWith(withReviewers(observedEnvironment(7, "staging", setting.DeploymentBranchPolicyProtectedBranches), false, 2), 10),
			want: []unexpectedOutsideChange{
				{Setting: "environments.staging.deployment_branch_policy", Before: "all", After: "protected_branches"},
				{Setting: "environments.staging.wait_timer", Before: "30", After: "10"},
			},
		},
		{
			name:   "reviewers update returns the changed wait_timer alone",
			change: reviewersChange(t, "staging", withReviewers(stagingAll, false, 1), reviewerSet(2)),
			before: []github.ObservationResult{stagingWith(withReviewers(stagingAll, false, 1), 30)},
			after:  stagingWith(withReviewers(stagingAll, false, 2), 10),
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.wait_timer", Before: "30", After: "10"}},
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
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.required_reviewers", Before: "[User 1]", After: "[User 2]"}},
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
			before: []github.ObservationResult{{SecretScanning: publicSecretScanning(true, true)}},
			after:  github.ObservationResult{SecretScanning: publicSecretScanning(false, false)},
			want:   nil,
		},
		{
			name:   "push protection not shown as a side effect of disabling secret scanning is compared",
			change: singleUpdate(setting.SecretScanningEnabled, setting.BoolValue(false), append(basesOf(setting.SecretScanningEnabled, setting.BoolValue(true)), basesOf(setting.RepositoryPushProtection, setting.BoolValue(false))...)),
			before: []github.ObservationResult{{SecretScanning: publicSecretScanning(true, false)}},
			after:  github.ObservationResult{SecretScanning: publicSecretScanning(false, true)},
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
			// Ruleset の更新操作は管理の単位である Ruleset の全体を送るため、同じ観測のほかの設定が変わっても比べない
			name:   "ruleset creation compares no setting",
			change: rulesetCreationChange(t, "protect"),
			before: []github.ObservationResult{general(func(*github.General) {})},
			after:  general(func(g *github.General) { g.AllowAutoMerge = true }),
			want:   nil,
		},
		{
			name:   "ruleset value update compares no setting",
			change: rulesetValueUpdateChange(t, "protect"),
			before: []github.ObservationResult{general(func(*github.General) {})},
			after:  general(func(g *github.General) { g.AllowAutoMerge = true }),
			want:   nil,
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
		change.Operations[0].Kind = changeplan.RulesetValueUpdate + 1
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
			before:   github.ObservationResult{General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false}, SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false}, SecretScanning: publicSecretScanning(true, true)},
			want:     nil,
		},
		{
			name:     "allow_auto_merge change reports different delete_branch_on_merge with both values",
			change:   allowAutoMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: github.General{DeleteBranchOnMerge: false}, SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: github.General{DeleteBranchOnMerge: true}, SecretScanning: publicSecretScanning(true, true)},
			want:     []unexpectedOutsideChange{{Setting: string(setting.DeleteBranchOnMerge), Before: "false", After: "true"}},
		},
		{
			name:     "allow_auto_merge change does not report its own setting",
			change:   allowAutoMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: github.General{AllowAutoMerge: false}, SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: github.General{AllowAutoMerge: true}, SecretScanning: publicSecretScanning(true, true)},
			want:     nil,
		},
		{
			name:     "delete_branch_on_merge change reports different allow_auto_merge with both values",
			change:   deleteBranchOnMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: github.General{AllowAutoMerge: false}, SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: github.General{AllowAutoMerge: true}, SecretScanning: publicSecretScanning(true, true)},
			want:     []unexpectedOutsideChange{{Setting: string(setting.AllowAutoMerge), Before: "false", After: "true"}},
		},
		{
			name:     "delete_branch_on_merge change does not report its own setting",
			change:   deleteBranchOnMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: github.General{DeleteBranchOnMerge: false}, SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: github.General{DeleteBranchOnMerge: true}, SecretScanning: publicSecretScanning(true, true)},
			want:     nil,
		},
		{
			name:     "default_workflow_permissions change reports different can_approve_pull_request_reviews with both values",
			change:   defaultWorkflowPermissionsChange(),
			endpoint: github.WorkflowPermissionsEndpoint,
			before:   github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: false}},
			after:    github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: true}},
			want:     []unexpectedOutsideChange{{Setting: string(setting.CanApprovePullRequestReviews), Before: "false", After: "true"}},
		},
		{
			name:     "can_approve_pull_request_reviews change reports different default_workflow_permissions with both values",
			change:   canApprovePullRequestReviewsChange(),
			endpoint: github.WorkflowPermissionsEndpoint,
			before:   github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: true}},
			after:    github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: false}},
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
			before:   github.ObservationResult{SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{SecretScanning: publicSecretScanning(false, false)},
			want: []unexpectedOutsideChange{
				{Setting: string(setting.SecretScanningEnabled), Before: "true", After: "false"},
				{Setting: string(setting.RepositoryPushProtection), Before: "true", After: "false"},
			},
		},
		{
			name:     "secret scanning change reports different general settings and the other secret scanning setting",
			change:   singleUpdate(setting.RepositoryPushProtection, setting.BoolValue(false), basesOf(setting.RepositoryPushProtection, setting.BoolValue(true))),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: github.General{AllowAutoMerge: false}, SecretScanning: publicSecretScanning(true, true)},
			after:    github.ObservationResult{General: github.General{AllowAutoMerge: true}, SecretScanning: publicSecretScanning(false, false)},
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
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite},
			},
			after: github.ObservationResult{
				General:             github.General{AllowAutoMerge: true, DeleteBranchOnMerge: true},
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead},
			},
			want: nil,
		},
		{
			name:     "general change does not compare workflow permissions settings",
			change:   allowAutoMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before: github.ObservationResult{
				General:             github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false},
				SecretScanning:      publicSecretScanning(true, true),
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: false},
			},
			after: github.ObservationResult{
				General:             github.General{AllowAutoMerge: true, DeleteBranchOnMerge: false},
				SecretScanning:      publicSecretScanning(true, true),
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: true},
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

// TestDeploymentBranchPolicyModeChanges は、担当者の集合の更新操作の変更対象外の設定として、その名前の Environment のデプロイ元制限の方式を比べる規則を確かめる。
// 個別パターンを比べないことは TestUnexpectedOutsideChanges が確かめる。
//
//trace:checks INV-001 FR-012
func TestDeploymentBranchPolicyModeChanges(t *testing.T) {
	staging := mustEnvironmentName(t, "staging")
	all := observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyAll)
	protected := observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyProtectedBranches)
	cases := []struct {
		name          string
		before, after github.ObservationResult
		want          []unexpectedOutsideChange
	}{
		{
			name:   "changed mode is named under the declared environment name with the mode names",
			before: github.ObservationResult{Environments: environments(all)},
			after:  github.ObservationResult{Environments: environments(protected)},
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.deployment_branch_policy", Before: "all", After: "protected_branches"}},
		},
		{name: "absent before the update is not compared", before: github.ObservationResult{Environments: environments()}, after: github.ObservationResult{Environments: environments(protected)}, want: nil},
		{name: "absent after the update is not compared", before: github.ObservationResult{Environments: environments(all)}, after: github.ObservationResult{Environments: environments()}, want: nil},
		{
			name:   "changed mode of another environment is not compared",
			before: github.ObservationResult{Environments: environments(all, observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll))},
			after:  github.ObservationResult{Environments: environments(all, observedEnvironment(8, "production", setting.DeploymentBranchPolicySelected))},
			want:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deploymentBranchPolicyModeChanges(staging, tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("deploymentBranchPolicyModeChanges() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEnvironmentProtectionChanges は、Environment の更新操作の変更対象外の設定を、その名前の Environment の保護設定に限って比べる規則を確かめる。
//
//trace:checks INV-001 FR-012
func TestEnvironmentProtectionChanges(t *testing.T) {
	staging := mustEnvironmentName(t, "staging")
	wait := func(minutes int) *int { return &minutes }
	reviewers := func(preventSelfReview bool, ids ...int64) *setting.RequiredReviewers {
		r := &setting.RequiredReviewers{PreventSelfReview: preventSelfReview}
		for _, id := range ids {
			r.Reviewers = append(r.Reviewers, setting.Reviewer{Type: setting.ReviewerUser, ID: id})
		}
		return r
	}
	observed := func(protection setting.EnvironmentProtection, others ...github.Environment) github.ObservationResult {
		e := observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyAll)
		e.Protection = protection
		return github.ObservationResult{Environments: environments(append(others, e)...)}
	}
	base := setting.EnvironmentProtection{WaitTimer: wait(30), RequiredReviewers: reviewers(true, 1, 2), CanAdminsBypass: true}
	stagingReviewers := setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: staging}
	// changedReviewers は、base の担当者と can_admins_bypass を変えた保護設定
	changedReviewers := setting.EnvironmentProtection{WaitTimer: wait(30), RequiredReviewers: reviewers(true, 3), CanAdminsBypass: false}
	cases := []struct {
		name          string
		excluded      []setting.Unit
		before, after github.ObservationResult
		want          []unexpectedOutsideChange
	}{
		{
			name:     "required_reviewers changed by the logical change is not compared",
			excluded: []setting.Unit{stagingReviewers},
			before:   observed(base),
			after:    observed(changedReviewers),
			want:     []unexpectedOutsideChange{{Setting: "environments.staging.can_admins_bypass", Before: "true", After: "false"}},
		},
		{
			name:     "required_reviewers of another environment does not exclude the item",
			excluded: []setting.Unit{{Key: setting.EnvironmentRequiredReviewers, Environment: mustEnvironmentName(t, "production")}},
			before:   observed(base),
			after:    observed(changedReviewers),
			want: []unexpectedOutsideChange{
				{Setting: "environments.staging.required_reviewers", Before: "[User 1, User 2]", After: "[User 3]"},
				{Setting: "environments.staging.can_admins_bypass", Before: "true", After: "false"},
			},
		},
		{
			// 同じ Environment でも形の違う単位は、保護設定の項目を除かない
			name:     "another setting of the same environment does not exclude the items",
			excluded: []setting.Unit{{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}},
			before:   observed(base),
			after:    observed(changedReviewers),
			want: []unexpectedOutsideChange{
				{Setting: "environments.staging.required_reviewers", Before: "[User 1, User 2]", After: "[User 3]"},
				{Setting: "environments.staging.can_admins_bypass", Before: "true", After: "false"},
			},
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
			after:  observed(setting.EnvironmentProtection{RequiredReviewers: reviewers(true, 1, 2), CanAdminsBypass: true}),
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.wait_timer", Before: "30", After: "none"}},
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
