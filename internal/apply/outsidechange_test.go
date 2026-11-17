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
//trace:checks INV-001
//trace:checks FR-012
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
			want:   []unexpectedOutsideChange{{Key: setting.SHAPinningRequired, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
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
			want:   []unexpectedOutsideChange{{Key: setting.SHAPinningRequired, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
		},
		{
			// 計画が作らない並び。同じ更新エンドポイントの更新操作が並んでも、後の直前の観測を基準にしないことを確かめる
			name: "two updates to the same endpoint compare only with the read before the first of them",
			change: changeplan.LogicalChange{Operations: []changeplan.UpdateOperation{
				allowedActionsChange().Operations[0], allowedActionsChange().Operations[0],
			}},
			before: []github.ObservationResult{localOnly(false), localOnly(true)},
			after:  localOnly(false),
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
}

// TestOutsideChangesAt は、論理的な設定変更が変えない設定を、管理範囲によらず比べる規則を確かめる。
//
//trace:checks INV-001
//trace:checks FR-012
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
			want:     []unexpectedOutsideChange{{Key: setting.DeleteBranchOnMerge, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
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
			want:     []unexpectedOutsideChange{{Key: setting.AllowAutoMerge, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
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
			want:     []unexpectedOutsideChange{{Key: setting.CanApprovePullRequestReviews, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
		},
		{
			name:     "can_approve_pull_request_reviews change reports different default_workflow_permissions with both values",
			change:   canApprovePullRequestReviewsChange(),
			endpoint: github.WorkflowPermissionsEndpoint,
			before:   github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: true}},
			after:    github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: false}},
			want: []unexpectedOutsideChange{{
				Key:    setting.DefaultWorkflowPermissions,
				Before: setting.PermissionLevelValue(setting.PermissionRead),
				After:  setting.PermissionLevelValue(setting.PermissionWrite),
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
			want:     []unexpectedOutsideChange{{Key: setting.SHAPinningRequired, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
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
			before:   github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{AllowedActions: github.Observed(setting.AllowedActionsSelected), SHAPinningRequired: github.Observed(false)}},
			after:    github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{AllowedActions: github.Observed(setting.AllowedActionsAll), SHAPinningRequired: github.Observed(true)}},
			want:     nil,
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
			if got := outsideChangesAt(tc.endpoint, tc.change.Keys(), tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("outsideChangesAt() = %v, want %v", got, tc.want)
			}
		})
	}
}
