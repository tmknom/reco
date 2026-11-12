package apply

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

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
			want:     desiredStateRepositoryChanged,
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

//trace:checks INV-001
//trace:checks FR-012
func TestUnexpectedUnmanagedChanges(t *testing.T) {
	cases := []struct {
		name          string
		before, after github.General
		want          []unexpectedUnmanagedChange
	}{
		{
			name:   "same settings have no change",
			before: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false},
			after:  github.General{AllowAutoMerge: false, DeleteBranchOnMerge: false},
			want:   nil,
		},
		{
			name:   "different delete_branch_on_merge is reported with both values",
			before: github.General{DeleteBranchOnMerge: false},
			after:  github.General{DeleteBranchOnMerge: true},
			want:   []unexpectedUnmanagedChange{{Key: setting.DeleteBranchOnMerge, Before: false, After: true}},
		},
		{
			name:   "different allow_auto_merge alone is not an unmanaged change",
			before: github.General{AllowAutoMerge: false},
			after:  github.General{AllowAutoMerge: true},
			want:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := unexpectedUnmanagedChanges(tc.before, tc.after); !slices.Equal(got, tc.want) {
				t.Errorf("unexpectedUnmanagedChanges() = %v, want %v", got, tc.want)
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

	cases := []struct {
		name          string
		responses     []response
		wantMethods   []string
		wantPrecond   preconditionCheck
		wantUpdate    github.UpdateOutcome
		wantDesired   desiredStateConfirmation
		wantUnmanaged []unexpectedUnmanagedChange
		wantResult    changeResult
		// wantCauses は、cause が errors.Is でたどれるべき error。空なら cause は nil。
		wantCauses []error
	}{
		{
			name:        "unreadable repository before the update is unconfirmed",
			responses:   []response{{status: http.StatusNotFound}},
			wantMethods: []string{"GET"},
			wantPrecond: preconditionUnconfirmed,
			wantResult:  notAttempted,
			wantCauses:  []error{github.ErrNotFound},
		},
		{
			name:        "different repository ID sends no update request",
			responses:   []response{okRepository(2, false, false)},
			wantMethods: []string{"GET"},
			wantPrecond: preconditionIdentityMismatch,
			wantResult:  notAttempted,
		},
		{
			name:        "different current value sends no update request",
			responses:   []response{okRepository(1, true, false)},
			wantMethods: []string{"GET"},
			wantPrecond: preconditionCurrentValueChanged,
			wantResult:  notAttempted,
		},
		{
			name:        "update confirmed by the second read is complete",
			responses:   []response{okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false)},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMatches,
			wantResult:  completed,
		},
		{
			name:        "rejected update is not followed by a read",
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
			responses:   []response{okRepository(1, false, false), disconnect, serverError},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateUnknown,
			wantDesired: desiredStateUnconfirmed,
			wantResult:  failed,
			wantCauses:  []error{github.ErrCommunicationFailure, github.ErrUnexpectedResponse},
		},
		{
			name:          "unknown update shows a changed unmanaged setting observed by the diagnosis",
			responses:     []response{okRepository(1, false, false), redirect, okRepository(1, true, true)},
			wantMethods:   []string{"GET", "PATCH", "GET"},
			wantPrecond:   preconditionHolds,
			wantUpdate:    github.UpdateUnknown,
			wantDesired:   desiredStateMatches,
			wantUnmanaged: []unexpectedUnmanagedChange{{Key: setting.DeleteBranchOnMerge, Before: false, After: true}},
			wantResult:    failed,
			wantCauses:    []error{github.ErrRedirect},
		},
		{
			name:        "successful update with an unreadable confirmation is unconfirmed with the read error as the cause",
			responses:   []response{okRepository(1, false, false), okRepository(1, true, false), serverError},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateUnconfirmed,
			wantResult:  failed,
			wantCauses:  []error{github.ErrUnexpectedResponse},
		},
		{
			name:        "successful update with another value is a mismatch",
			responses:   []response{okRepository(1, false, false), okRepository(1, true, false), okRepository(1, false, false)},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateMismatch,
			wantResult:  failed,
		},
		{
			name:          "successful update with a changed unmanaged setting fails",
			responses:     []response{okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, true)},
			wantMethods:   []string{"GET", "PATCH", "GET"},
			wantPrecond:   preconditionHolds,
			wantUpdate:    github.UpdateSucceeded,
			wantDesired:   desiredStateMatches,
			wantUnmanaged: []unexpectedUnmanagedChange{{Key: setting.DeleteBranchOnMerge, Before: false, After: true}},
			wantResult:    failed,
		},
		{
			name:        "successful update with a changed repository ID does not compare unmanaged settings",
			responses:   []response{okRepository(1, false, false), okRepository(1, true, false), okRepository(2, true, true)},
			wantMethods: []string{"GET", "PATCH", "GET"},
			wantPrecond: preconditionHolds,
			wantUpdate:  github.UpdateSucceeded,
			wantDesired: desiredStateRepositoryChanged,
			wantResult:  failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, rec := newSequenceServer(t, tc.responses...)
			p := newPlan(t)

			got := executeChange(context.Background(), newClient(baseURL), p, allowAutoMergeChange())

			if !slices.Equal(rec.Methods(), tc.wantMethods) {
				t.Errorf("requests = %v, want %v", rec.Methods(), tc.wantMethods)
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
			if !slices.Equal(got.unexpectedUnmanagedChanges, tc.wantUnmanaged) {
				t.Errorf("unexpectedUnmanagedChanges = %v, want %v", got.unexpectedUnmanagedChanges, tc.wantUnmanaged)
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
