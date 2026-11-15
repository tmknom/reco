package apply

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// actionsEnabled は、Actions が有効で、Actions 許可ポリシーが selected でない Actions の実行許可の観測結果。
func actionsEnabled(policy setting.AllowedActionsPolicy, shaPinningRequired bool) *github.ActionsPermissions {
	return &github.ActionsPermissions{
		Enabled:            true,
		AllowedActions:     github.Observed(policy),
		SHAPinningRequired: github.Observed(shaPinningRequired),
		SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableNotSelected),
	}
}

// actionsSelected は、Actions 許可ポリシーが selected で、選択方式の Actions 許可条件が criteria の Actions の実行許可の観測結果。
func actionsSelected(criteria setting.SelectedActionsCriteria) *github.ActionsPermissions {
	return &github.ActionsPermissions{
		Enabled:            true,
		AllowedActions:     github.Observed(setting.AllowedActionsSelected),
		SHAPinningRequired: github.Observed(false),
		SelectedActions:    github.Observed(criteria),
	}
}

// actionsDisabled は、Actions が無効のリポジトリの Actions の実行許可の観測結果。
func actionsDisabled() *github.ActionsPermissions {
	return &github.ActionsPermissions{
		AllowedActions:     github.Unavailable[setting.AllowedActionsPolicy](github.UnavailableActionsDisabled),
		SHAPinningRequired: github.Unavailable[bool](github.UnavailableActionsDisabled),
		SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableActionsDisabled),
	}
}

// TestSameObservation は、承認時に予定した値と更新直前の設定の観測結果が同じかの規則を確かめる。
//
//trace:checks INV-001
//trace:checks INV-004
//trace:checks FR-010
func TestSameObservation(t *testing.T) {
	disabled := github.Unavailable[setting.Value](github.UnavailableActionsDisabled)
	notSelected := github.Unavailable[setting.Value](github.UnavailableNotSelected)
	criteria := func(patterns ...string) github.SettingObservation[setting.Value] {
		return github.Observed(setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{PatternsAllowed: patterns}))
	}
	cases := []struct {
		name       string
		atApproval github.SettingObservation[setting.Value]
		current    github.SettingObservation[setting.Value]
		want       bool
	}{
		{name: "unavailable for the same reason is the same", atApproval: disabled, current: disabled, want: true},
		{name: "unavailable for another reason differs", atApproval: disabled, current: notSelected, want: false},
		{name: "observed value where unavailable was approved differs", atApproval: disabled, current: criteria("a/*"), want: false},
		{name: "unavailable where an observed value was approved differs", atApproval: criteria("a/*"), current: disabled, want: false},
		{name: "another observed value differs", atApproval: criteria("a/*"), current: criteria("b/*"), want: false},
		{name: "observed values differing only in the order of patterns are the same", atApproval: criteria("a/*", "b/*"), current: criteria("b/*", "a/*"), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameObservation(tc.atApproval, tc.current); got != tc.want {
				t.Errorf("sameObservation() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks INV-001
//trace:checks INV-004
//trace:checks FR-010
func TestCheckPrecondition(t *testing.T) {
	change := allowAutoMergeChange()
	cases := []struct {
		name        string
		observed    github.ObservationResult
		want        preconditionCheck
		wantChanged setting.Key
	}{
		{
			name:     "same identity and current value hold",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: false}},
			want:     preconditionHolds,
		},
		{
			name:     "changed value of another setting alone holds",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true}},
			want:     preconditionHolds,
		},
		{
			name:     "different ID alone is an identity mismatch",
			observed: github.ObservationResult{RepositoryID: 2, General: github.General{AllowAutoMerge: false}},
			want:     preconditionIdentityMismatch,
		},
		{
			name:        "changed current value alone is a current value change of the setting even if it equals the desired value",
			observed:    github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: true}},
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.AllowAutoMerge,
		},
		{
			name:     "different ID and current value is an identity mismatch",
			observed: github.ObservationResult{RepositoryID: 2, General: github.General{AllowAutoMerge: true}},
			want:     preconditionIdentityMismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, changed := checkPrecondition(1, change, tc.observed); got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %q, want %v, %q", got, changed, tc.want, tc.wantChanged)
			}
		})
	}

	// 関係する現在値の設定が複数並ぶ変更と、承認時に予定した値が観測不能の変更。値の比べ方は TestSameObservation が確かめる
	actionsCases := []struct {
		name        string
		change      changeplan.LogicalChange
		observed    *github.ActionsPermissions
		want        preconditionCheck
		wantChanged setting.Key
	}{
		{
			name:        "selected to all with criteria other than shown before approval is a current value change of selected_actions",
			change:      selectedToAllChange(),
			observed:    actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"a/*", "b/*", "c/*"}}),
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.SelectedActions,
		},
		{
			name:        "selected to all with the policy no longer selected is a current value change of allowed_actions",
			change:      selectedToAllChange(),
			observed:    actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.AllowedActions,
		},
		{
			name:     "enabling actions holds while actions is still disabled",
			change:   enableActionsChange(),
			observed: actionsDisabled(),
			want:     preconditionHolds,
		},
	}
	for _, tc := range actionsCases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, ActionsPermissions: tc.observed}
			if got, changed := checkPrecondition(1, tc.change, observed); got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %q, want %v, %q", got, changed, tc.want, tc.wantChanged)
			}
		})
	}

	t.Run("logical change without relevant settings panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("checkPrecondition() did not panic")
			}
		}()
		empty := changeplan.LogicalChange{Key: setting.AllowAutoMerge, Desired: setting.BoolValue(true)}
		checkPrecondition(1, empty, github.ObservationResult{RepositoryID: 1})
	})
}

//trace:checks FR-012
func TestConfirmDesiredState(t *testing.T) {
	change := allowAutoMergeChange()
	cases := []struct {
		name     string
		observed github.ObservationResult
		want     desiredStateConfirmation
	}{
		{
			name:     "desired value matches",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: true}},
			want:     desiredStateMatches,
		},
		{
			name:     "other value is a mismatch",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: false}},
			want:     desiredStateMismatch,
		},
		{
			name:     "different ID is not confirmed even if the value matches",
			observed: github.ObservationResult{RepositoryID: 2, General: github.General{AllowAutoMerge: true}},
			want:     desiredStateIdentityMismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := confirmDesiredState(1, change, tc.observed); got != tc.want {
				t.Errorf("confirmDesiredState() = %v, want %v", got, tc.want)
			}
		})
	}

	selectedCases := []struct {
		name     string
		observed *github.ActionsPermissions
		want     desiredStateConfirmation
	}{
		{
			name:     "criteria differing from the desired value only in the order of patterns match",
			observed: actionsSelected(setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{"b/*", "a/*"}}),
			want:     desiredStateMatches,
		},
		{
			name:     "unavailable criteria are unavailable rather than a mismatch",
			observed: actionsDisabled(),
			want:     desiredStateUnavailable,
		},
	}
	for _, tc := range selectedCases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, ActionsPermissions: tc.observed}
			if got := confirmDesiredState(1, selectedActionsChange(), observed); got != tc.want {
				t.Errorf("confirmDesiredState() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestUnexpectedOutsideChanges は、論理的な設定変更が変えない設定を、管理範囲によらず比べる規則を確かめる。
//
//trace:checks INV-001
//trace:checks FR-012
func TestUnexpectedOutsideChanges(t *testing.T) {
	cases := []struct {
		name          string
		change        changeplan.LogicalChange
		endpoint      updateEndpoint
		before, after github.ObservationResult
		want          []unexpectedOutsideChange
	}{
		{
			name:     "same settings have no change",
			change:   allowAutoMergeChange(),
			endpoint: repositoryEndpoint,
			before:   github.ObservationResult{General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false}},
			after:    github.ObservationResult{General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false}},
			want:     nil,
		},
		{
			name:     "allow_auto_merge change reports different delete_branch_on_merge with both values",
			change:   allowAutoMergeChange(),
			endpoint: repositoryEndpoint,
			before:   github.ObservationResult{General: github.General{DeleteBranchOnMerge: false}},
			after:    github.ObservationResult{General: github.General{DeleteBranchOnMerge: true}},
			want:     []unexpectedOutsideChange{{Key: setting.DeleteBranchOnMerge, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
		},
		{
			name:     "allow_auto_merge change does not report its own setting",
			change:   allowAutoMergeChange(),
			endpoint: repositoryEndpoint,
			before:   github.ObservationResult{General: github.General{AllowAutoMerge: false}},
			after:    github.ObservationResult{General: github.General{AllowAutoMerge: true}},
			want:     nil,
		},
		{
			name:     "delete_branch_on_merge change reports different allow_auto_merge with both values",
			change:   deleteBranchOnMergeChange(),
			endpoint: repositoryEndpoint,
			before:   github.ObservationResult{General: github.General{AllowAutoMerge: false}},
			after:    github.ObservationResult{General: github.General{AllowAutoMerge: true}},
			want:     []unexpectedOutsideChange{{Key: setting.AllowAutoMerge, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
		},
		{
			name:     "delete_branch_on_merge change does not report its own setting",
			change:   deleteBranchOnMergeChange(),
			endpoint: repositoryEndpoint,
			before:   github.ObservationResult{General: github.General{DeleteBranchOnMerge: false}},
			after:    github.ObservationResult{General: github.General{DeleteBranchOnMerge: true}},
			want:     nil,
		},
		{
			name:     "default_workflow_permissions change reports different can_approve_pull_request_reviews with both values",
			change:   defaultWorkflowPermissionsChange(),
			endpoint: workflowPermissionsEndpoint,
			before:   github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: false}},
			after:    github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: true}},
			want:     []unexpectedOutsideChange{{Key: setting.CanApprovePullRequestReviews, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
		},
		{
			name:     "can_approve_pull_request_reviews change reports different default_workflow_permissions with both values",
			change:   canApprovePullRequestReviewsChange(),
			endpoint: workflowPermissionsEndpoint,
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
			endpoint: workflowPermissionsEndpoint,
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
			endpoint: repositoryEndpoint,
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
			endpoint: actionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsAll, false)},
			after:    github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsLocalOnly, true)},
			want:     []unexpectedOutsideChange{{Key: setting.SHAPinningRequired, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
		},
		{
			// 比べられない値を変化なしと推測しない。完了にしないことは、期待状態の確認が観測不能になることで保たれる
			name:     "sha_pinning_required unavailable after the update is not reported",
			change:   allowedActionsChange(),
			endpoint: actionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsAll, false)},
			after:    github.ObservationResult{ActionsPermissions: actionsDisabled()},
			want:     nil,
		},
		{
			name:     "sha_pinning_required unavailable before the update is not reported",
			change:   allowedActionsChange(),
			endpoint: actionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsDisabled()},
			after:    github.ObservationResult{ActionsPermissions: actionsEnabled(setting.AllowedActionsLocalOnly, true)},
			want:     nil,
		},
		{
			name:     "sha_pinning_required unavailable both before and after the update is not reported",
			change:   allowedActionsChange(),
			endpoint: actionsPermissionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsDisabled()},
			after:    github.ObservationResult{ActionsPermissions: actionsDisabled()},
			want:     nil,
		},
		{
			name:     "selected_actions change does not compare allowed_actions or sha_pinning_required",
			change:   selectedActionsChange(),
			endpoint: selectedActionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{AllowedActions: github.Observed(setting.AllowedActionsSelected), SHAPinningRequired: github.Observed(false)}},
			after:    github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{AllowedActions: github.Observed(setting.AllowedActionsAll), SHAPinningRequired: github.Observed(true)}},
			want:     nil,
		},
		{
			// apply の実行からは到達しない組。選択方式の Actions 許可条件を変更対象外の設定にする更新エンドポイントがないため、
			// 比較が値の判定を通ることだけを、一覧に依らない入力で確かめる
			name:     "outside criteria differing only in the order of patterns are not reported",
			change:   allowedActionsChange(),
			endpoint: selectedActionsEndpoint,
			before:   github.ObservationResult{ActionsPermissions: actionsSelected(setting.SelectedActionsCriteria{PatternsAllowed: []string{"a/*", "b/*"}})},
			after:    github.ObservationResult{ActionsPermissions: actionsSelected(setting.SelectedActionsCriteria{PatternsAllowed: []string{"b/*", "a/*"}})},
			want:     nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := unexpectedOutsideChanges(tc.endpoint, tc.change, tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("unexpectedOutsideChanges() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks INV-001
//trace:checks INV-004
//trace:checks INV-005
//trace:checks FR-010
//trace:checks FR-012
//trace:checks FR-013
func TestExecuteChange(t *testing.T) {
	disconnect := response{disconnect: true}
	serverError := response{status: http.StatusInternalServerError}
	redirect := response{status: http.StatusTemporaryRedirect, location: "https://example.com/"}

	noContent := response{status: http.StatusNoContent}
	// allDelays は、アーキテクチャ設計が定める、期待状態と一致しないときに観測し直す前に待つ時間。
	allDelays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	cases := []struct {
		name        string
		change      changeplan.LogicalChange
		responses   []response
		wantMethods []string
		// wantDelays は、観測し直す前に待つよう求めた時間の並び。待たなければ nil。
		wantDelays  []time.Duration
		wantPrecond preconditionCheck
		wantUpdate  github.UpdateOutcome
		wantDesired desiredStateConfirmation
		wantOutside []unexpectedOutsideChange
		wantResult  changeResult
		// wantCauses は、cause が errors.Is でたどれるべき error。空なら cause は nil。
		wantCauses []error
	}{
		{
			name:        "unreadable repository before the update is unconfirmed",
			change:      allowAutoMergeChange(),
			responses:   []response{{status: http.StatusNotFound}},
			wantMethods: []string{"GET"},
			wantPrecond: preconditionUnconfirmed,
			wantResult:  notAttempted,
			wantCauses:  []error{github.ErrNotFound},
		},
		{
			name:        "different repository ID sends no update request",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(2, false, false)},
			wantMethods: []string{"GET"},
			wantPrecond: preconditionIdentityMismatch,
			wantResult:  notAttempted,
		},
		{
			name:        "different current value sends no update request",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(1, true, false)},
			wantMethods: []string{"GET"},
			wantPrecond: preconditionCurrentValueChanged,
			wantResult:  notAttempted,
		},
		{
			name:        "update confirmed by the second read is complete",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false)},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMatches,
			wantResult:  completed,
		},
		{
			name:        "rejected update is not followed by a read",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(1, false, false), {status: http.StatusForbidden}},
			wantMethods: []string{"GET", "PATCH"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateRejected,
			wantDesired: desiredStateNotChecked,
			wantResult:  failed,
			wantCauses:  []error{github.ErrForbidden},
		},
		{
			name:        "redirected update is unknown and stays failed even if the state matches",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(1, false, false), redirect, okRepository(1, true, false)},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateUnknown,
			wantDesired: desiredStateMatches,
			wantResult:  failed,
			wantCauses:  []error{github.ErrRedirect},
		},
		{
			name:        "disconnected update with an unreadable diagnosis keeps both the update and the read errors",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(1, false, false), disconnect, serverError},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateUnknown,
			wantDesired: desiredStateUnconfirmed,
			wantResult:  failed,
			wantCauses:  []error{github.ErrCommunicationFailure, github.ErrUnexpectedResponse},
		},
		{
			name:        "unknown update shows a changed other setting observed by the diagnosis",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(1, false, false), redirect, okRepository(1, true, true)},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateUnknown,
			wantDesired: desiredStateMatches,
			wantOutside: []unexpectedOutsideChange{{Key: setting.DeleteBranchOnMerge, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
			wantResult:  failed,
			wantCauses:  []error{github.ErrRedirect},
		},
		{
			name:        "successful update with an unreadable confirmation is unconfirmed with the read error as the cause",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(1, false, false), okRepository(1, true, false), serverError},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateUnconfirmed,
			wantResult:  failed,
			wantCauses:  []error{github.ErrUnexpectedResponse},
		},
		{
			name:   "successful update with another value on all six reads is a mismatch",
			change: allowAutoMergeChange(),
			responses: []response{
				okRepository(1, false, false), okRepository(1, true, false),
				okRepository(1, false, false), okRepository(1, false, false), okRepository(1, false, false),
				okRepository(1, false, false), okRepository(1, false, false), okRepository(1, false, false),
			},
			wantMethods: []string{"GET", "PATCH", "GET", "GET", "GET", "GET", "GET", "GET"},
			wantDelays:  allDelays,
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMismatch,
			wantResult:  failed,
		},
		{
			name:   "successful update with the desired value on the third read is complete",
			change: allowAutoMergeChange(),
			responses: []response{
				okRepository(1, false, false), okRepository(1, true, false),
				okRepository(1, false, false), okRepository(1, false, false), okRepository(1, true, false),
			},
			wantMethods: []string{"GET", "PATCH", "GET", "GET", "GET"},
			wantDelays:  allDelays[:2],
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMatches,
			wantResult:  completed,
		},
		{
			name:   "other setting changed only on a mismatching read is not an unexpected outside change",
			change: allowAutoMergeChange(),
			responses: []response{
				okRepository(1, false, false), okRepository(1, true, false),
				okRepository(1, false, true), okRepository(1, true, false),
			},
			wantMethods: []string{"GET", "PATCH", "GET", "GET"},
			wantDelays:  allDelays[:1],
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMatches,
			wantResult:  completed,
		},
		{
			name:   "unreadable read after a mismatching read is unconfirmed without further reads",
			change: allowAutoMergeChange(),
			responses: []response{
				okRepository(1, false, false), okRepository(1, true, false),
				okRepository(1, false, false), serverError,
			},
			wantMethods: []string{"GET", "PATCH", "GET", "GET"},
			wantDelays:  allDelays[:1],
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateUnconfirmed,
			wantResult:  failed,
			wantCauses:  []error{github.ErrUnexpectedResponse},
		},
		{
			name:        "unknown update with another value is read only once",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(1, false, false), redirect, okRepository(1, false, false)},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateUnknown,
			wantDesired: desiredStateMismatch,
			wantResult:  failed,
			wantCauses:  []error{github.ErrRedirect},
		},
		{
			name:        "successful update with a changed other setting fails",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, true)},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMatches,
			wantOutside: []unexpectedOutsideChange{{Key: setting.DeleteBranchOnMerge, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
			wantResult:  failed,
		},
		{
			name:        "workflow permissions update confirmed by two reads before and after is complete",
			change:      defaultWorkflowPermissionsChange(),
			responses:   []response{okRepository(1, false, false), okWorkflowPermissions("write", false), noContent, okRepository(1, false, false), okWorkflowPermissions("read", false)},
			wantMethods: []string{"GET", "GET", "PUT", "GET", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMatches,
			wantResult:  completed,
		},
		{
			name:        "unreadable workflow permissions before the update is unconfirmed",
			change:      defaultWorkflowPermissionsChange(),
			responses:   []response{okRepository(1, false, false), serverError},
			wantMethods: []string{"GET", "GET"},
			wantPrecond: preconditionUnconfirmed,
			wantResult:  notAttempted,
			wantCauses:  []error{github.ErrUnexpectedResponse},
		},
		{
			name:   "workflow permissions update answered with 200 is unknown and stays failed even if the state matches",
			change: defaultWorkflowPermissionsChange(),
			responses: []response{
				okRepository(1, false, false), okWorkflowPermissions("write", false), {status: http.StatusOK},
				okRepository(1, false, false), okWorkflowPermissions("read", false),
			},
			wantMethods: []string{"GET", "GET", "PUT", "GET", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateUnknown,
			wantDesired: desiredStateMatches,
			wantResult:  failed,
			wantCauses:  []error{github.ErrUnexpectedResponse},
		},
		{
			name:   "successful workflow permissions update with unreadable workflow permissions after it is unconfirmed",
			change: defaultWorkflowPermissionsChange(),
			responses: []response{
				okRepository(1, false, false), okWorkflowPermissions("write", false), noContent,
				okRepository(1, false, false), serverError,
			},
			wantMethods: []string{"GET", "GET", "PUT", "GET", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateUnconfirmed,
			wantResult:  failed,
			wantCauses:  []error{github.ErrUnexpectedResponse},
		},
		{
			name:   "successful workflow permissions update with a changed unsent workflow permission fails",
			change: defaultWorkflowPermissionsChange(),
			responses: []response{
				okRepository(1, false, false), okWorkflowPermissions("write", false), noContent,
				okRepository(1, false, false), okWorkflowPermissions("read", true),
			},
			wantMethods: []string{"GET", "GET", "PUT", "GET", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMatches,
			wantOutside: []unexpectedOutsideChange{{Key: setting.CanApprovePullRequestReviews, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
			wantResult:  failed,
		},
		{
			name:        "successful update with a changed repository ID does not compare other settings",
			change:      allowAutoMergeChange(),
			responses:   []response{okRepository(1, false, false), okRepository(1, true, false), okRepository(2, true, true)},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateIdentityMismatch,
			wantResult:  failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, rec := newSequenceServer(t, tc.responses...)
			p := newPlan(t)
			delays := &delayRecorder{}

			got := executeChange(context.Background(), newClient(baseURL), p, tc.change, delays.after)

			if !reflect.DeepEqual(got.change, tc.change) {
				t.Errorf("change = %+v, want %+v", got.change, tc.change)
			}
			if !slices.Equal(rec.Methods(), tc.wantMethods) {
				t.Errorf("requests = %v, want %v", rec.Methods(), tc.wantMethods)
			}
			if !slices.Equal(delays.delays, tc.wantDelays) {
				t.Errorf("delays = %v, want %v", delays.delays, tc.wantDelays)
			}
			if got.precondition != tc.wantPrecond {
				t.Errorf("precondition = %v, want %v", got.precondition, tc.wantPrecond)
			}
			if got.update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.update, tc.wantUpdate)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if !reflect.DeepEqual(got.unexpectedOutsideChanges, tc.wantOutside) {
				t.Errorf("unexpectedOutsideChanges = %v, want %v", got.unexpectedOutsideChanges, tc.wantOutside)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
			for _, want := range tc.wantCauses {
				if !errors.Is(got.cause, want) {
					t.Errorf("cause = %v, want it to wrap %v", got.cause, want)
				}
			}
			if len(tc.wantCauses) == 0 && got.cause != nil {
				t.Errorf("cause = %v, want nil", got.cause)
			}
		})
	}

	t.Run("interruption while waiting to read again after a mismatch is unconfirmed without another read", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t, okRepository(1, false, false), okRepository(1, true, false), okRepository(1, false, false), okRepository(1, true, false))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		interrupted := func(time.Duration) <-chan time.Time {
			cancel()
			return make(chan time.Time)
		}

		got := executeChange(ctx, newClient(baseURL), newPlan(t), allowAutoMergeChange(), interrupted)

		if want := []string{"GET", "PATCH", "GET"}; !slices.Equal(rec.Methods(), want) {
			t.Errorf("requests = %v, want %v", rec.Methods(), want)
		}
		if got.update != github.UpdateSucceeded {
			t.Errorf("update = %v, want %v", got.update, github.UpdateSucceeded)
		}
		if got.confirmation != desiredStateUnconfirmed {
			t.Errorf("confirmation = %v, want %v", got.confirmation, desiredStateUnconfirmed)
		}
		if !errors.Is(got.cause, context.Canceled) {
			t.Errorf("cause = %v, want it to wrap %v", got.cause, context.Canceled)
		}
	})
}

// TestExecuteChange_ActionsPermissions は、Actions の実行許可の領域の設定を一つの更新操作で変える論理的な設定変更について、
// 観測と更新の要求の送り先と、観測不能を伴う変更結果を確かめる。
//
//trace:checks INV-001
//trace:checks INV-004
//trace:checks INV-005
//trace:checks FR-010
//trace:checks FR-012
//trace:checks FR-013
func TestExecuteChange_ActionsPermissions(t *testing.T) {
	const (
		getRepository         = "GET /repos/owner/repo"
		getActionsPermissions = "GET /repos/owner/repo/actions/permissions"
		getSelectedActions    = "GET /repos/owner/repo/actions/permissions/selected-actions"
		putActionsPermissions = "PUT /repos/owner/repo/actions/permissions"
		putSelectedActions    = "PUT /repos/owner/repo/actions/permissions/selected-actions"
	)
	noContent := response{status: http.StatusNoContent}
	allDelays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	cases := []struct {
		name         string
		change       changeplan.LogicalChange
		responses    []response
		wantRequests []string
		// wantBodies は要求ごとの本文。nil なら本文を確かめない。
		wantBodies  []map[string]any
		wantDelays  []time.Duration
		wantPrecond preconditionCheck
		wantChanged setting.Key
		wantUpdate  github.UpdateOutcome
		wantDesired desiredStateConfirmation
		wantReason  github.UnavailableReason
		wantOutside []unexpectedOutsideChange
		wantResult  changeResult
		wantCauses  []error
	}{
		{
			// 更新後は Actions 許可ポリシーが selected でないため選択方式の Actions 許可条件を読まず、その消失は承認した副作用として比べない
			name:   "selected to all reads the lost criteria only before the update and completes without an unexpected outside change",
			change: selectedToAllChange(),
			responses: []response{
				okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(true, false, `["a/*", "b/*"]`), noContent,
				okRepository(1, false, false), okAllowedActions("all", false),
			},
			wantRequests: []string{getRepository, getActionsPermissions, getSelectedActions, putActionsPermissions, getRepository, getActionsPermissions},
			wantBodies:   []map[string]any{nil, nil, nil, {"enabled": true, "allowed_actions": "all"}, nil, nil},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			name:         "selected to all with criteria other than shown before approval sends no update request",
			change:       selectedToAllChange(),
			responses:    []response{okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, false, `["a/*", "b/*"]`)},
			wantRequests: []string{getRepository, getActionsPermissions, getSelectedActions},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  setting.SelectedActions,
			wantResult:   notAttempted,
		},
		{
			// 更新直前は sha_pinning_required が観測不能で比べる観測値がなく、保持の確認結果を根拠に保全されるとみなす
			name:   "enabling actions with local_only completes even if sha_pinning_required is read as true after the update",
			change: enableActionsChange(),
			responses: []response{
				okRepository(1, false, false), okActionsDisabled(), noContent,
				okRepository(1, false, false), okAllowedActions("local_only", true),
			},
			wantRequests: []string{getRepository, getActionsPermissions, putActionsPermissions, getRepository, getActionsPermissions},
			wantBodies:   []map[string]any{nil, nil, {"enabled": true, "allowed_actions": "local_only"}, nil, nil},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			name:   "all to local_only reads and updates only actions permissions and completes",
			change: allowedActionsChange(),
			responses: []response{
				okRepository(1, false, false), okAllowedActions("all", false), noContent,
				okRepository(1, false, false), okAllowedActions("local_only", false),
			},
			wantRequests: []string{getRepository, getActionsPermissions, putActionsPermissions, getRepository, getActionsPermissions},
			wantBodies:   []map[string]any{nil, nil, {"enabled": true, "allowed_actions": "local_only"}, nil, nil},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			name:   "changed sha_pinning_required fails even if the policy matches",
			change: allowedActionsChange(),
			responses: []response{
				okRepository(1, false, false), okAllowedActions("all", false), noContent,
				okRepository(1, false, false), okAllowedActions("local_only", true),
			},
			wantRequests: []string{getRepository, getActionsPermissions, putActionsPermissions, getRepository, getActionsPermissions},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantOutside:  []unexpectedOutsideChange{{Key: setting.SHAPinningRequired, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
			wantResult:   failed,
		},
		{
			name:   "actions disabled on all six reads after the update is unavailable rather than a mismatch",
			change: allowedActionsChange(),
			responses: []response{
				okRepository(1, false, false), okAllowedActions("all", false), noContent,
				okRepository(1, false, false), okActionsDisabled(), okRepository(1, false, false), okActionsDisabled(),
				okRepository(1, false, false), okActionsDisabled(), okRepository(1, false, false), okActionsDisabled(),
				okRepository(1, false, false), okActionsDisabled(), okRepository(1, false, false), okActionsDisabled(),
			},
			wantRequests: []string{
				getRepository, getActionsPermissions, putActionsPermissions,
				getRepository, getActionsPermissions, getRepository, getActionsPermissions, getRepository, getActionsPermissions,
				getRepository, getActionsPermissions, getRepository, getActionsPermissions, getRepository, getActionsPermissions,
			},
			wantDelays:  allDelays,
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateUnavailable,
			wantReason:  github.UnavailableActionsDisabled,
			wantResult:  failed,
		},
		{
			name:   "actions disabled only on the first read after the update is complete once the desired value is read",
			change: allowedActionsChange(),
			responses: []response{
				okRepository(1, false, false), okAllowedActions("all", false), noContent,
				okRepository(1, false, false), okActionsDisabled(), okRepository(1, false, false), okAllowedActions("local_only", false),
			},
			wantRequests: []string{
				getRepository, getActionsPermissions, putActionsPermissions,
				getRepository, getActionsPermissions, getRepository, getActionsPermissions,
			},
			wantDelays:  allDelays[:1],
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMatches,
			wantResult:  completed,
		},
		{
			name:   "selected criteria change sends only the selected actions update and completes",
			change: selectedActionsChange(),
			responses: []response{
				okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, false, `["a/*", "b/*"]`), noContent,
				okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, true, `["a/*", "b/*"]`),
			},
			wantRequests: []string{
				getRepository, getActionsPermissions, getSelectedActions, putSelectedActions,
				getRepository, getActionsPermissions, getSelectedActions,
			},
			wantBodies: []map[string]any{
				nil, nil, nil, {"github_owned_allowed": false, "verified_allowed": true, "patterns_allowed": []any{"a/*", "b/*"}},
				nil, nil, nil,
			},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMatches,
			wantResult:  completed,
		},
		{
			name:         "selected criteria change with the policy no longer selected sends no update request",
			change:       selectedActionsChange(),
			responses:    []response{okRepository(1, false, false), okAllowedActions("all", false)},
			wantRequests: []string{getRepository, getActionsPermissions},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  setting.SelectedActions,
			wantResult:   notAttempted,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, rec := newSequenceServer(t, tc.responses...)
			delays := &delayRecorder{}

			got := executeChange(context.Background(), newClient(baseURL), newPlan(t), tc.change, delays.after)

			if !slices.Equal(rec.Requests(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", rec.Requests(), tc.wantRequests)
			}
			if tc.wantBodies != nil && !reflect.DeepEqual(rec.Bodies(), tc.wantBodies) {
				t.Errorf("request bodies = %v, want %v", rec.Bodies(), tc.wantBodies)
			}
			if !slices.Equal(delays.delays, tc.wantDelays) {
				t.Errorf("delays = %v, want %v", delays.delays, tc.wantDelays)
			}
			if got.precondition != tc.wantPrecond {
				t.Errorf("precondition = %v, want %v", got.precondition, tc.wantPrecond)
			}
			if got.changedSetting != tc.wantChanged {
				t.Errorf("changedSetting = %q, want %q", got.changedSetting, tc.wantChanged)
			}
			if got.update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.update, tc.wantUpdate)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if got.desiredUnavailableReason != tc.wantReason {
				t.Errorf("desiredUnavailableReason = %v, want %v", got.desiredUnavailableReason, tc.wantReason)
			}
			if !reflect.DeepEqual(got.unexpectedOutsideChanges, tc.wantOutside) {
				t.Errorf("unexpectedOutsideChanges = %v, want %v", got.unexpectedOutsideChanges, tc.wantOutside)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
			for _, want := range tc.wantCauses {
				if !errors.Is(got.cause, want) {
					t.Errorf("cause = %v, want it to wrap %v", got.cause, want)
				}
			}
			if len(tc.wantCauses) == 0 && got.cause != nil {
				t.Errorf("cause = %v, want nil", got.cause)
			}
		})
	}
}

// TestExecuteChanges は、論理的な設定変更を順に実行し、完了以外の変更結果が出たら残りに要求を送らずに止める規則を確かめる。
// 打ち切りの条件は変更結果が完了かどうかだけで、異常の種類から変更結果への対応は TestExecuteChange が確かめるため、
// 未実行と失敗の代表だけを確かめる。
//
//trace:checks INV-004
//trace:checks INV-005
//trace:checks FR-010
//trace:checks FR-012
//trace:checks FR-013
func TestExecuteChanges(t *testing.T) {
	redirect := response{status: http.StatusTemporaryRedirect, location: "https://example.com/"}
	noContent := response{status: http.StatusNoContent}
	generalChanges := []changeplan.LogicalChange{deleteBranchOnMergeChange(), allowAutoMergeChange()}
	cases := []struct {
		name        string
		changes     []changeplan.LogicalChange
		responses   []response
		wantMethods []string
		wantBodies  []map[string]any
		wantPrecond []preconditionCheck
		wantResults []changeResult
	}{
		{
			name:    "both changes complete with a fresh read before each update",
			changes: generalChanges,
			responses: []response{
				okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
				okRepository(1, false, true), okRepository(1, true, true), okRepository(1, true, true),
			},
			wantMethods: []string{"GET", "PATCH", "GET", "GET", "PATCH", "GET"},
			wantBodies:  []map[string]any{nil, {"delete_branch_on_merge": true}, nil, nil, {"allow_auto_merge": true}, nil},
			wantPrecond: []preconditionCheck{preconditionHolds, preconditionHolds},
			wantResults: []changeResult{completed, completed},
		},
		{
			name:    "second current value changed after the first completes is not attempted",
			changes: generalChanges,
			responses: []response{
				okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
				okRepository(1, true, true),
			},
			wantMethods: []string{"GET", "PATCH", "GET", "GET"},
			wantBodies:  []map[string]any{nil, {"delete_branch_on_merge": true}, nil, nil},
			wantPrecond: []preconditionCheck{preconditionHolds, preconditionCurrentValueChanged},
			wantResults: []changeResult{completed, notAttempted},
		},
		{
			name:        "first precondition failure stops the second without a request",
			changes:     generalChanges,
			responses:   []response{okRepository(2, false, false)},
			wantMethods: []string{"GET"},
			wantBodies:  []map[string]any{nil},
			wantPrecond: []preconditionCheck{preconditionIdentityMismatch, preconditionNotChecked},
			wantResults: []changeResult{notAttempted, notAttempted},
		},
		{
			name:        "first unknown update outcome stops the second without a request",
			changes:     generalChanges,
			responses:   []response{okRepository(1, false, false), redirect, okRepository(1, false, true)},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantBodies:  []map[string]any{nil, {"delete_branch_on_merge": true}, nil},
			wantPrecond: []preconditionCheck{preconditionHolds, preconditionNotChecked},
			wantResults: []changeResult{failed, notAttempted},
		},
		{
			name: "general and workflow permissions changes all complete with patches before puts",
			changes: []changeplan.LogicalChange{
				deleteBranchOnMergeChange(), allowAutoMergeChange(), defaultWorkflowPermissionsChange(), canApprovePullRequestReviewsChange(),
			},
			responses: []response{
				okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
				okRepository(1, false, true), okRepository(1, true, true), okRepository(1, true, true),
				okRepository(1, true, true), okWorkflowPermissions("write", true), noContent, okRepository(1, true, true), okWorkflowPermissions("read", true),
				okRepository(1, true, true), okWorkflowPermissions("read", true), noContent, okRepository(1, true, true), okWorkflowPermissions("read", false),
			},
			wantMethods: []string{
				"GET", "PATCH", "GET",
				"GET", "PATCH", "GET",
				"GET", "GET", "PUT", "GET", "GET",
				"GET", "GET", "PUT", "GET", "GET",
			},
			wantBodies: []map[string]any{
				nil, {"delete_branch_on_merge": true}, nil,
				nil, {"allow_auto_merge": true}, nil,
				nil, nil, {"default_workflow_permissions": "read"}, nil, nil,
				nil, nil, {"can_approve_pull_request_reviews": false}, nil, nil,
			},
			wantPrecond: []preconditionCheck{preconditionHolds, preconditionHolds, preconditionHolds, preconditionHolds},
			wantResults: []changeResult{completed, completed, completed, completed},
		},
		{
			name: "unconfirmed general change stops the workflow permissions change without a read or an update",
			changes: []changeplan.LogicalChange{
				deleteBranchOnMergeChange(), allowAutoMergeChange(), defaultWorkflowPermissionsChange(),
			},
			responses: []response{
				okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
				okRepository(1, false, true), okRepository(1, false, true), {status: http.StatusInternalServerError},
			},
			wantMethods: []string{"GET", "PATCH", "GET", "GET", "PATCH", "GET"},
			wantBodies:  []map[string]any{nil, {"delete_branch_on_merge": true}, nil, nil, {"allow_auto_merge": true}, nil},
			wantPrecond: []preconditionCheck{preconditionHolds, preconditionHolds, preconditionNotChecked},
			wantResults: []changeResult{completed, failed, notAttempted},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, rec := newSequenceServer(t, tc.responses...)
			changes := tc.changes

			got := executeChanges(context.Background(), newClient(baseURL), newPlan(t), changes, elapsedAtOnce)

			if !slices.Equal(rec.Methods(), tc.wantMethods) {
				t.Errorf("requests = %v, want %v", rec.Methods(), tc.wantMethods)
			}
			if !reflect.DeepEqual(rec.Bodies(), tc.wantBodies) {
				t.Errorf("request bodies = %v, want %v", rec.Bodies(), tc.wantBodies)
			}
			if len(got) != len(changes) {
				t.Fatalf("executeChanges() returned %d records, want %d", len(got), len(changes))
			}
			for i, record := range got {
				if !reflect.DeepEqual(record.change, changes[i]) {
					t.Errorf("records[%d].change = %+v, want %+v", i, record.change, changes[i])
				}
				if record.precondition != tc.wantPrecond[i] {
					t.Errorf("records[%d].precondition = %v, want %v", i, record.precondition, tc.wantPrecond[i])
				}
				if record.result() != tc.wantResults[i] {
					t.Errorf("records[%d].result() = %v, want %v", i, record.result(), tc.wantResults[i])
				}
			}
		})
	}
}
