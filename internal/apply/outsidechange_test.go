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
			name:   "pattern removal and addition do not compare the protection",
			change: deploymentPatternsChange(t, "staging", selectedEnvironment(7, "staging", []setting.DeploymentPattern{branchPattern("feature/*")}, 11), selectedPolicy(branchPattern("main"))),
			before: []github.ObservationResult{stagingWith(selectedStaging, 30), stagingWith(selectedStaging, 30)},
			after:  stagingWith(selectedStaging, 10),
			want:   nil,
		},
		{
			name:   "policy and criteria changed by the logical change are not compared",
			change: toSelectedChange(),
			before: []github.ObservationResult{localOnly(false), selectedWith(false, setBack)},
			after:  selectedWith(false, declaredCriteria),
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
		change.Operations[0].Kind = changeplan.DeploymentPatternRemoval + 1
		unexpectedOutsideChanges(change, []github.ObservationResult{{}}, github.ObservationResult{})
	})
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
			before:   github.ObservationResult{General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false}},
			after:    github.ObservationResult{General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false}},
			want:     nil,
		},
		{
			name:     "allow_auto_merge change reports different delete_branch_on_merge with both values",
			change:   allowAutoMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: github.General{DeleteBranchOnMerge: false}},
			after:    github.ObservationResult{General: github.General{DeleteBranchOnMerge: true}},
			want:     []unexpectedOutsideChange{{Setting: string(setting.DeleteBranchOnMerge), Before: "false", After: "true"}},
		},
		{
			name:     "allow_auto_merge change does not report its own setting",
			change:   allowAutoMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: github.General{AllowAutoMerge: false}},
			after:    github.ObservationResult{General: github.General{AllowAutoMerge: true}},
			want:     nil,
		},
		{
			name:     "delete_branch_on_merge change reports different allow_auto_merge with both values",
			change:   deleteBranchOnMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: github.General{AllowAutoMerge: false}},
			after:    github.ObservationResult{General: github.General{AllowAutoMerge: true}},
			want:     []unexpectedOutsideChange{{Setting: string(setting.AllowAutoMerge), Before: "false", After: "true"}},
		},
		{
			name:     "delete_branch_on_merge change does not report its own setting",
			change:   deleteBranchOnMergeChange(),
			endpoint: github.RepositoryEndpoint,
			before:   github.ObservationResult{General: github.General{DeleteBranchOnMerge: false}},
			after:    github.ObservationResult{General: github.General{DeleteBranchOnMerge: true}},
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
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: false},
			},
			after: github.ObservationResult{
				General:             github.General{AllowAutoMerge: true, DeleteBranchOnMerge: false},
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
			if got := outsideChangesAt(tc.endpoint, tc.change.SettingsToChange(), tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("outsideChangesAt() = %v, want %v", got, tc.want)
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
	cases := []struct {
		name          string
		before, after github.ObservationResult
		want          []unexpectedOutsideChange
	}{
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
			name:   "same protection with reviewers only reordered has no change",
			before: observed(base),
			after:  observed(setting.EnvironmentProtection{WaitTimer: wait(30), RequiredReviewers: reviewers(true, 2, 1), CanAdminsBypass: true}),
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
			name:   "changed wait_timer is one change",
			before: observed(base),
			after:  observed(setting.EnvironmentProtection{RequiredReviewers: reviewers(true, 1, 2), CanAdminsBypass: true}),
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.wait_timer", Before: "30", After: "none"}},
		},
		{
			name:   "changed required_reviewers is one change",
			before: observed(base),
			after:  observed(setting.EnvironmentProtection{WaitTimer: wait(30), RequiredReviewers: reviewers(false, 1, 2), CanAdminsBypass: true}),
			want: []unexpectedOutsideChange{{
				Setting: "environments.staging.required_reviewers",
				Before:  "reviewers: [User 1, User 2], prevent_self_review: true", After: "reviewers: [User 1, User 2], prevent_self_review: false",
			}},
		},
		{
			name:   "changed can_admins_bypass is one change",
			before: observed(base),
			after:  observed(setting.EnvironmentProtection{WaitTimer: wait(30), RequiredReviewers: reviewers(true, 1, 2)}),
			want:   []unexpectedOutsideChange{{Setting: "environments.staging.can_admins_bypass", Before: "true", After: "false"}},
		},
		{
			name:   "all three changed are three changes in the show order",
			before: observed(base),
			after:  observed(setting.EnvironmentProtection{WaitTimer: wait(5)}),
			want: []unexpectedOutsideChange{
				{Setting: "environments.staging.wait_timer", Before: "30", After: "5"},
				{Setting: "environments.staging.required_reviewers", Before: "reviewers: [User 1, User 2], prevent_self_review: true", After: "none"},
				{Setting: "environments.staging.can_admins_bypass", Before: "true", After: "false"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := environmentProtectionChanges(staging, tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("environmentProtectionChanges() = %v, want %v", got, tc.want)
			}
		})
	}
}
