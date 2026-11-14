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

func TestObservedValue(t *testing.T) {
	t.Run("observed value is returned", func(t *testing.T) {
		got := observedValue(github.ObservationResult{General: github.General{AllowAutoMerge: true}}, setting.AllowAutoMerge)
		if want := setting.BoolValue(true); !reflect.DeepEqual(got, want) {
			t.Errorf("observedValue() = %v, want %v", got, want)
		}
	})

	t.Run("unavailable setting panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("observedValue() did not panic for an unavailable setting")
			}
		}()
		observed := github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{
			AllowedActions: github.Unavailable[setting.AllowedActionsPolicy](github.UnavailableActionsDisabled),
		}}
		_ = observedValue(observed, setting.AllowedActions)
	})
}

//trace:checks INV-004
//trace:checks FR-010
func TestCheckPrecondition(t *testing.T) {
	change := allowAutoMergeChange()
	cases := []struct {
		name     string
		observed github.ObservationResult
		want     preconditionCheck
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
			name:     "changed current value alone is a current value change even if it equals the desired value",
			observed: github.ObservationResult{RepositoryID: 1, General: github.General{AllowAutoMerge: true}},
			want:     preconditionCurrentValueChanged,
		},
		{
			name:     "different ID and current value is an identity mismatch",
			observed: github.ObservationResult{RepositoryID: 2, General: github.General{AllowAutoMerge: true}},
			want:     preconditionIdentityMismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkPrecondition(1, change, tc.observed); got != tc.want {
				t.Errorf("checkPrecondition() = %v, want %v", got, tc.want)
			}
		})
	}
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

	t.Run("change without an update operation panics before sending a request", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t)
		change := changeplan.LogicalChange{Key: unknownKey, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true)}
		defer func() {
			if recover() == nil {
				t.Error("executeChange() did not panic")
			}
			if len(rec.Methods()) != 0 {
				t.Errorf("requests = %v, want none", rec.Methods())
			}
		}()

		executeChange(context.Background(), newClient(baseURL), newPlan(t), change, elapsedAtOnce)
	})

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
