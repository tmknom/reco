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
	"github.com/tmknom/reco/internal/repository"
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

// TestObserveBeforeUpdate は、2つ目の更新操作の直前の観測で、更新要求の間隔を観測より前に待つ規則と、
// 1つ目の反映の遅れを待って観測し直す規則を確かめる。
// 観測し直すのは読み取りだけで、開始してよいのは最後の観測で変更前提が成立したときだけである。
//
//trace:checks INV-004
//trace:checks INV-005
//trace:checks FR-010
func TestObserveBeforeUpdate(t *testing.T) {
	const (
		getRepository         = "GET /repos/owner/repo"
		getActionsPermissions = "GET /repos/owner/repo/actions/permissions"
		getSelectedActions    = "GET /repos/owner/repo/actions/permissions/selected-actions"
	)
	selected := []response{okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(true, false)}
	localOnly := []response{okRepository(1, false, false), okAllowedActions("local_only", false)}
	readSelected := []string{getRepository, getActionsPermissions, getSelectedActions}
	readNotSelected := []string{getRepository, getActionsPermissions}
	repeat := func(n int, rs []response) []response {
		var out []response
		for range n {
			out = append(out, rs...)
		}
		return out
	}
	repeatRequests := func(n int, rs []string) []string {
		var out []string
		for range n {
			out = append(out, rs...)
		}
		return out
	}
	allDelays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	cases := []struct {
		name                    string
		change                  changeplan.LogicalChange
		responses               []response
		wantRequests            []string
		wantReobservationDelays []time.Duration
		want                    operationRecord
		wantErr                 bool
	}{
		{
			name:         "policy already selected holds without reading again",
			change:       toSelectedChange(),
			responses:    selected,
			wantRequests: readSelected,
			want:         operationRecord{precondition: preconditionHolds},
		},
		{
			name:                    "policy still before the first update and then selected holds after reading again once",
			change:                  toSelectedChange(),
			responses:               append(slices.Clone(localOnly), selected...),
			wantRequests:            append(slices.Clone(readNotSelected), readSelected...),
			wantReobservationDelays: allDelays[:1],
			want:                    operationRecord{precondition: preconditionHolds},
		},
		{
			name:                    "policy still before the first update on all six reads is a current value change of allowed_actions",
			change:                  toSelectedChange(),
			responses:               repeat(6, localOnly),
			wantRequests:            repeatRequests(6, readNotSelected),
			wantReobservationDelays: allDelays,
			want:                    operationRecord{precondition: preconditionCurrentValueChanged, changedSetting: setting.AllowedActions},
		},
		{
			name:         "policy neither before the first update nor selected is a current value change without reading again",
			change:       toSelectedChange(),
			responses:    []response{okRepository(1, false, false), okAllowedActions("all", false)},
			wantRequests: readNotSelected,
			want:         operationRecord{precondition: preconditionCurrentValueChanged, changedSetting: setting.AllowedActions},
		},
		{
			name:         "failed read is unconfirmed without reading again",
			change:       toSelectedChange(),
			responses:    []response{okRepository(1, false, false), {status: http.StatusInternalServerError}},
			wantRequests: readNotSelected,
			want:         operationRecord{precondition: preconditionUnconfirmed},
			wantErr:      true,
		},
		{
			name:         "different ID is an identity mismatch without reading again even if the policy is before the first update",
			change:       toSelectedChange(),
			responses:    []response{okRepository(2, false, false), okAllowedActions("local_only", false)},
			wantRequests: readNotSelected,
			want:         operationRecord{precondition: preconditionIdentityMismatch},
		},
		{
			name:                    "actions still disabled as before the first update and then selected holds after reading again once",
			change:                  enableSelectedChange(),
			responses:               append([]response{okRepository(1, false, false), okActionsDisabled()}, selected...),
			wantRequests:            append(slices.Clone(readNotSelected), readSelected...),
			wantReobservationDelays: allDelays[:1],
			want:                    operationRecord{precondition: preconditionHolds},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)
			delays := &delayRecorder{}

			_, got, err := observeBeforeUpdate(context.Background(), newClient(baseURL), newRepositoryPlan(t), tc.change, 1, observationAreas(tc.change), delays.after, &updateInterval{updateStarted: true})

			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			// 2つ目の更新操作は、1つ目の更新要求のあとのため、観測し直しの待ち時間の前に更新要求の間隔の1秒を待つ。
			// 間隔の1秒は観測し直しの回数と上限に数えない。
			wantDelays := append([]time.Duration{time.Second}, tc.wantReobservationDelays...)
			if !slices.Equal(delays.delays, wantDelays) {
				t.Errorf("delays = %v, want %v", delays.delays, wantDelays)
			}
			if got != tc.want {
				t.Errorf("observeBeforeUpdate() = %+v, want %+v", got, tc.want)
			}
			if (err != nil) != tc.wantErr {
				t.Errorf("observeBeforeUpdate() error = %v, want error %v", err, tc.wantErr)
			}
		})
	}

	t.Run("interruption while waiting to read again is unconfirmed without another read", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, append(slices.Clone(localOnly), selected...)...)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		// 1回目の待機は更新要求の間隔のため経過させ、2回目の観測し直す前の待機で中断する。
		waits := 0
		interrupted := func(d time.Duration) <-chan time.Time {
			waits++
			if waits == 1 {
				return elapsedAtOnce(d)
			}
			cancel()
			return make(chan time.Time)
		}
		change := toSelectedChange()

		_, got, err := observeBeforeUpdate(ctx, newClient(baseURL), newRepositoryPlan(t), change, 1, observationAreas(change), interrupted, &updateInterval{updateStarted: true})

		if !slices.Equal(server.RequestLines(), readNotSelected) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), readNotSelected)
		}
		if got != (operationRecord{precondition: preconditionUnconfirmed}) {
			t.Errorf("observeBeforeUpdate() = %+v, want unconfirmed", got)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("observeBeforeUpdate() error = %v, want it to wrap %v", err, context.Canceled)
		}
	})

	// 要求を一つも送らずに終わることが、間隔を最初の読み取りより前に待つことも示す
	t.Run("interruption while waiting for the interval between update requests is unconfirmed without a read", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, selected...)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		interrupted := func(time.Duration) <-chan time.Time {
			cancel()
			return make(chan time.Time)
		}
		change := toSelectedChange()

		_, got, err := observeBeforeUpdate(ctx, newClient(baseURL), newRepositoryPlan(t), change, 1, observationAreas(change), interrupted, &updateInterval{updateStarted: true})

		if len(server.RequestLines()) != 0 {
			t.Errorf("requests = %v, want none", server.RequestLines())
		}
		if got != (operationRecord{precondition: preconditionUnconfirmed}) {
			t.Errorf("observeBeforeUpdate() = %+v, want unconfirmed", got)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("observeBeforeUpdate() error = %v, want it to wrap %v", err, context.Canceled)
		}
	})
}

// TestUpdateIntervalWait は、この実行で更新操作を開始したあとだけ更新要求の間隔を待つ規則を確かめる。
func TestUpdateIntervalWait(t *testing.T) {
	t.Run("no update started does not wait", func(t *testing.T) {
		delays := &delayRecorder{}

		err := (&updateInterval{}).wait(context.Background(), delays.after)

		if len(delays.delays) != 0 || err != nil {
			t.Errorf("wait() delays = %v, error = %v, want no wait and no error", delays.delays, err)
		}
	})

	t.Run("update started waits one second once", func(t *testing.T) {
		delays := &delayRecorder{}

		err := (&updateInterval{updateStarted: true}).wait(context.Background(), delays.after)

		if !slices.Equal(delays.delays, []time.Duration{time.Second}) || err != nil {
			t.Errorf("wait() delays = %v, error = %v, want [1s] and no error", delays.delays, err)
		}
	})
}

// TestObservationAreas は、更新直前と更新後の観測で読む設定の領域を、更新操作と基準の設定から導く規則を確かめる。
func TestObservationAreas(t *testing.T) {
	cases := []struct {
		name    string
		change  changeplan.LogicalChange
		want    []setting.Area
		notWant []setting.Area
	}{
		{
			name:    "general change reads only the general area",
			change:  allowAutoMergeChange(),
			want:    []setting.Area{setting.AllowAutoMerge.Area()},
			notWant: []setting.Area{setting.AllowedActions.Area()},
		},
		{
			name:   "change to selected reads the actions permissions area",
			change: toSelectedChange(),
			want:   []setting.Area{setting.AllowedActions.Area()},
		},
		{
			// どの変更予定にも生じない組。基準の設定の領域を読まないと、観測結果にその値がないことを確かめる
			name:   "relevant setting in another area is also read",
			change: singleUpdate(setting.AllowAutoMerge, setting.BoolValue(true), relevantTo(setting.DefaultWorkflowPermissions, setting.PermissionLevelValue(setting.PermissionRead))),
			want:   []setting.Area{setting.AllowAutoMerge.Area(), setting.DefaultWorkflowPermissions.Area()},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := observationAreas(tc.change)
			for _, area := range tc.want {
				if !slices.Contains(got, area) {
					t.Errorf("observationAreas() = %v, want it to contain %v", got, area)
				}
			}
			for _, area := range tc.notWant {
				if slices.Contains(got, area) {
					t.Errorf("observationAreas() = %v, want it not to contain %v", got, area)
				}
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
			baseURL, server := newSequenceServer(t, tc.responses...)
			p := newRepositoryPlan(t)
			delays := &delayRecorder{}

			got := executeChange(context.Background(), newClient(baseURL), p, tc.change, delays.after, &updateInterval{})

			if !reflect.DeepEqual(got.change, tc.change) {
				t.Errorf("change = %+v, want %+v", got.change, tc.change)
			}
			if !slices.Equal(server.Methods(), tc.wantMethods) {
				t.Errorf("requests = %v, want %v", server.Methods(), tc.wantMethods)
			}
			if !slices.Equal(delays.delays, tc.wantDelays) {
				t.Errorf("delays = %v, want %v", delays.delays, tc.wantDelays)
			}
			if got.operation(0).precondition != tc.wantPrecond {
				t.Errorf("precondition = %v, want %v", got.operation(0).precondition, tc.wantPrecond)
			}
			if got.operation(0).update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.operation(0).update, tc.wantUpdate)
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
		baseURL, server := newSequenceServer(t, okRepository(1, false, false), okRepository(1, true, false), okRepository(1, false, false), okRepository(1, true, false))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		interrupted := func(time.Duration) <-chan time.Time {
			cancel()
			return make(chan time.Time)
		}

		got := executeChange(ctx, newClient(baseURL), newRepositoryPlan(t), allowAutoMergeChange(), interrupted, &updateInterval{})

		if want := []string{"GET", "PATCH", "GET"}; !slices.Equal(server.Methods(), want) {
			t.Errorf("requests = %v, want %v", server.Methods(), want)
		}
		if got.operation(0).update != github.UpdateSucceeded {
			t.Errorf("update = %v, want %v", got.operation(0).update, github.UpdateSucceeded)
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
				okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(true, false, "a/*", "b/*"), noContent,
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
			responses:    []response{okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, false, "a/*", "b/*")},
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
				okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, false, "a/*", "b/*"), noContent,
				okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, true, "a/*", "b/*"),
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
			baseURL, server := newSequenceServer(t, tc.responses...)
			delays := &delayRecorder{}

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), tc.change, delays.after, &updateInterval{})

			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			if tc.wantBodies != nil && !reflect.DeepEqual(server.Bodies(), tc.wantBodies) {
				t.Errorf("request bodies = %v, want %v", server.Bodies(), tc.wantBodies)
			}
			if !slices.Equal(delays.delays, tc.wantDelays) {
				t.Errorf("delays = %v, want %v", delays.delays, tc.wantDelays)
			}
			if got.operation(0).precondition != tc.wantPrecond {
				t.Errorf("precondition = %v, want %v", got.operation(0).precondition, tc.wantPrecond)
			}
			if got.operation(0).changedSetting != tc.wantChanged {
				t.Errorf("changedSetting = %q, want %q", got.operation(0).changedSetting, tc.wantChanged)
			}
			if got.operation(0).update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.operation(0).update, tc.wantUpdate)
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

// TestExecuteChange_TwoUpdates は、二つの更新操作からなる論理的な設定変更について、
// 更新操作ごとの直前の観測と変更前提、先行する更新操作が成功したときだけ後続へ進むこと、更新後の確認のつなぎ目を確かめる。
//
//trace:checks INV-001
//trace:checks INV-004
//trace:checks INV-005
//trace:checks FR-010
//trace:checks FR-012
//trace:checks FR-013
func TestExecuteChange_TwoUpdates(t *testing.T) {
	const (
		getRepository         = "GET /repos/owner/repo"
		getActionsPermissions = "GET /repos/owner/repo/actions/permissions"
		getSelectedActions    = "GET /repos/owner/repo/actions/permissions/selected-actions"
		putActionsPermissions = "PUT /repos/owner/repo/actions/permissions"
		putSelectedActions    = "PUT /repos/owner/repo/actions/permissions/selected-actions"
	)
	noContent := response{status: http.StatusNoContent}
	redirect := response{status: http.StatusTemporaryRedirect, location: "https://example.com/"}
	beforeFirst := []response{okRepository(1, false, false), okAllowedActions("local_only", false)}
	// GitHub がポリシーの更新で戻した3項目。宣言した declaredCriteria と3項目とも異なる
	setBack := okSelectedActions(true, false)
	beforeSecond := []response{okRepository(1, false, false), okAllowedActions("selected", false), setBack}
	// 宣言した値と許可パターンの順序だけが異なる
	afterBoth := []response{okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, true, "b/*", "a/*")}
	readNotSelected := []string{getRepository, getActionsPermissions}
	readSelected := []string{getRepository, getActionsPermissions, getSelectedActions}
	join := func(parts ...[]response) []response { return slices.Concat(parts...) }
	requests := func(parts ...[]string) []string { return slices.Concat(parts...) }
	firstBody := map[string]any{"enabled": true, "allowed_actions": "selected"}
	secondBody := map[string]any{"github_owned_allowed": false, "verified_allowed": true, "patterns_allowed": []any{"a/*", "b/*"}}
	holds := func(update github.UpdateOutcome) operationRecord {
		return operationRecord{precondition: preconditionHolds, update: update}
	}
	// 2つ目の更新直前の観測に進むときは、その前に更新要求の間隔の1秒を一度待つ。
	interval := []time.Duration{time.Second}
	cases := []struct {
		name         string
		change       changeplan.LogicalChange
		responses    []response
		wantRequests []string
		// wantBodies は要求ごとの本文。nil なら本文を確かめない。
		wantBodies     []map[string]any
		wantDelays     []time.Duration
		wantOperations []operationRecord
		wantDesired    desiredStateConfirmation
		wantOutside    []unexpectedOutsideChange
		wantResult     changeResult
		wantCause      error
	}{
		{
			// 遷移元が local_only で github_owned_allowed: false を宣言した遷移も、ほかの遷移と同じ扱いで実行する
			name:         "local_only to selected sends both updates in order and completes",
			wantDelays:   interval,
			change:       toSelectedChange(),
			responses:    join(beforeFirst, []response{noContent}, beforeSecond, []response{noContent}, afterBoth),
			wantRequests: requests(readNotSelected, []string{putActionsPermissions}, readSelected, []string{putSelectedActions}, readSelected),
			wantBodies: []map[string]any{
				nil, nil, firstBody,
				nil, nil, nil, secondBody,
				nil, nil, nil,
			},
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), holds(github.UpdateSucceeded)},
			wantDesired:    desiredStateMatches,
			wantResult:     completed,
		},
		{
			// 1つ目が状態を変えているため、後続を開始しなくても一度だけ観測して確認結果と変更対象外の設定を与える
			name:       "policy other than selected before the second update fails without the second update and reads once after it",
			wantDelays: interval,
			change:     toSelectedChange(),
			responses: join(beforeFirst, []response{noContent, okRepository(1, false, false), okAllowedActions("all", false)},
				[]response{okRepository(1, false, false), okAllowedActions("all", false)}),
			wantRequests:   requests(readNotSelected, []string{putActionsPermissions}, readNotSelected, readNotSelected),
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), {precondition: preconditionCurrentValueChanged, changedSetting: setting.AllowedActions}},
			wantDesired:    desiredStateMismatch,
			wantResult:     failed,
		},
		{
			name:       "failed read before the second update fails without the second update and reads once after it",
			wantDelays: interval,
			change:     toSelectedChange(),
			responses: join(beforeFirst, []response{noContent, okRepository(1, false, false), {status: http.StatusInternalServerError}},
				beforeSecond),
			wantRequests:   requests(readNotSelected, []string{putActionsPermissions}, readNotSelected, readSelected),
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), {precondition: preconditionUnconfirmed}},
			wantDesired:    desiredStateMismatch,
			wantResult:     failed,
			wantCause:      github.ErrUnexpectedResponse,
		},
		{
			name:           "rejected first update sends no read before the second",
			change:         toSelectedChange(),
			responses:      join(beforeFirst, []response{{status: http.StatusForbidden, body: `{"message": "Forbidden"}`}}),
			wantRequests:   requests(readNotSelected, []string{putActionsPermissions}),
			wantOperations: []operationRecord{holds(github.UpdateRejected)},
			wantResult:     failed,
			wantCause:      github.ErrForbidden,
		},
		{
			// 診断の観測で selected が見えても、成否不明の更新の後続を開始しない
			name:           "unknown first update reads once for diagnosis and does not start the second even if selected is read",
			change:         toSelectedChange(),
			responses:      join(beforeFirst, []response{redirect}, beforeSecond),
			wantRequests:   requests(readNotSelected, []string{putActionsPermissions}, readSelected),
			wantOperations: []operationRecord{holds(github.UpdateUnknown)},
			wantDesired:    desiredStateMismatch,
			wantResult:     failed,
			wantCause:      github.ErrRedirect,
		},
		{
			// 1つ目の更新エンドポイントの sha_pinning_required を1つ目の直前と比べる機会を残す
			name:       "rejected second update reads once after it and reports sha_pinning_required changed from before the first",
			wantDelays: interval,
			change:     toSelectedChange(),
			responses: join(beforeFirst, []response{noContent}, beforeSecond, []response{{status: http.StatusConflict, body: `{"message": "Conflict"}`}},
				[]response{okRepository(1, false, false), okAllowedActions("selected", true), setBack}),
			wantRequests:   requests(readNotSelected, []string{putActionsPermissions}, readSelected, []string{putSelectedActions}, readSelected),
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), holds(github.UpdateRejected)},
			wantDesired:    desiredStateMismatch,
			wantOutside:    []unexpectedOutsideChange{{Key: setting.SHAPinningRequired, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
			wantResult:     failed,
			wantCause:      github.ErrConflict,
		},
		{
			name:           "unknown second update reads once for diagnosis",
			wantDelays:     interval,
			change:         toSelectedChange(),
			responses:      join(beforeFirst, []response{noContent}, beforeSecond, []response{redirect}, afterBoth),
			wantRequests:   requests(readNotSelected, []string{putActionsPermissions}, readSelected, []string{putSelectedActions}, readSelected),
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), holds(github.UpdateUnknown)},
			wantDesired:    desiredStateMatches,
			wantResult:     failed,
			wantCause:      github.ErrRedirect,
		},
		{
			name:       "sha_pinning_required changed from the read before the first update fails even if unchanged since the second",
			wantDelays: interval,
			change:     toSelectedChange(),
			responses: join(beforeFirst, []response{noContent},
				[]response{okRepository(1, false, false), okAllowedActions("selected", true), setBack}, []response{noContent},
				[]response{okRepository(1, false, false), okAllowedActions("selected", true), okSelectedActions(false, true, "a/*", "b/*")}),
			wantRequests:   requests(readNotSelected, []string{putActionsPermissions}, readSelected, []string{putSelectedActions}, readSelected),
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), holds(github.UpdateSucceeded)},
			wantDesired:    desiredStateMatches,
			wantOutside:    []unexpectedOutsideChange{{Key: setting.SHAPinningRequired, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
			wantResult:     failed,
		},
		{
			name:   "policy read as before the first update and then selected starts the second without sending the first again",
			change: toSelectedChange(),
			responses: join(beforeFirst, []response{noContent},
				[]response{okRepository(1, false, false), okAllowedActions("local_only", false)}, beforeSecond, []response{noContent}, afterBoth),
			wantRequests: requests(readNotSelected, []string{putActionsPermissions},
				readNotSelected, readSelected, []string{putSelectedActions}, readSelected),
			// 間隔の1秒のあとに、観測し直しの1秒が続く
			wantDelays:     []time.Duration{time.Second, time.Second},
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), holds(github.UpdateSucceeded)},
			wantDesired:    desiredStateMatches,
			wantResult:     completed,
		},
		{
			// 1つ目の直前は sha_pinning_required が観測不能で比べる観測値がなく、更新後に true でも予期しない変化にしない
			name:       "disabled to selected enables actions with the policy in one update and completes",
			wantDelays: interval,
			change:     enableSelectedChange(),
			responses: join([]response{okRepository(1, false, false), okActionsDisabled(), noContent}, beforeSecond, []response{noContent},
				[]response{okRepository(1, false, false), okAllowedActions("selected", true), okSelectedActions(false, true, "a/*", "b/*")}),
			wantRequests: requests(readNotSelected, []string{putActionsPermissions}, readSelected, []string{putSelectedActions}, readSelected),
			wantBodies: []map[string]any{
				nil, nil, firstBody,
				nil, nil, nil, secondBody,
				nil, nil, nil,
			},
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), holds(github.UpdateSucceeded)},
			wantDesired:    desiredStateMatches,
			wantResult:     completed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)
			delays := &delayRecorder{}

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), tc.change, delays.after, &updateInterval{})

			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			if tc.wantBodies != nil && !reflect.DeepEqual(server.Bodies(), tc.wantBodies) {
				t.Errorf("request bodies = %v, want %v", server.Bodies(), tc.wantBodies)
			}
			if !slices.Equal(delays.delays, tc.wantDelays) {
				t.Errorf("delays = %v, want %v", delays.delays, tc.wantDelays)
			}
			if !slices.Equal(got.operations, tc.wantOperations) {
				t.Errorf("operations = %+v, want %+v", got.operations, tc.wantOperations)
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
			if tc.wantCause != nil && !errors.Is(got.cause, tc.wantCause) {
				t.Errorf("cause = %v, want it to wrap %v", got.cause, tc.wantCause)
			}
			if tc.wantCause == nil && got.cause != nil {
				t.Errorf("cause = %v, want nil", got.cause)
			}
		})
	}

	// 1つ目は成功しているため変更結果は失敗。更新後の観測は一度だけ試みるが、中断を受けているため読み取りも失敗する。
	t.Run("interruption while waiting for the interval before the second update fails without the second update", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, join(beforeFirst, []response{noContent}, beforeSecond)...)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		interrupted := func(time.Duration) <-chan time.Time {
			cancel()
			return make(chan time.Time)
		}
		change := toSelectedChange()

		got := executeChange(ctx, newClient(baseURL), newRepositoryPlan(t), change, interrupted, &updateInterval{})

		if want := requests(readNotSelected, []string{putActionsPermissions}); !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		if want := []operationRecord{holds(github.UpdateSucceeded), {precondition: preconditionUnconfirmed}}; !slices.Equal(got.operations, want) {
			t.Errorf("operations = %+v, want %+v", got.operations, want)
		}
		if got.result() != failed {
			t.Errorf("result() = %v, want %v", got.result(), failed)
		}
		if !errors.Is(got.cause, context.Canceled) {
			t.Errorf("cause = %v, want it to wrap %v", got.cause, context.Canceled)
		}
		wantDetails := []string{
			"Update request (" + string(setting.AllowedActions) + "): succeeded",
			"Update request (" + string(setting.SelectedActions) + "): not started",
			"Precondition: not confirmed (read failed)",
			"Desired state: not confirmed (read failed)",
		}
		if !slices.Equal(details(got), wantDetails) {
			t.Errorf("details() = %q, want %q", details(got), wantDetails)
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
		wantDelays  []time.Duration
		// wantRequestsAtWait は、待機ごとに、それを求めた時点で GitHub が受けていた要求の数。待機の位置を示す。
		wantRequestsAtWait []int
	}{
		{
			// 間隔は1件目の更新後の観測のあと、2件目の更新直前の観測より前に一度だけ入り、最後の更新のあとには入らない
			name:    "both changes complete with a fresh read before each update and the interval before the second read",
			changes: generalChanges,
			responses: []response{
				okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
				okRepository(1, false, true), okRepository(1, true, true), okRepository(1, true, true),
			},
			wantMethods:        []string{"GET", "PATCH", "GET", "GET", "PATCH", "GET"},
			wantBodies:         []map[string]any{nil, {"delete_branch_on_merge": true}, nil, nil, {"allow_auto_merge": true}, nil},
			wantPrecond:        []preconditionCheck{preconditionHolds, preconditionHolds},
			wantResults:        []changeResult{completed, completed},
			wantDelays:         []time.Duration{time.Second},
			wantRequestsAtWait: []int{3},
		},
		{
			// 観測し直しで待っても、経過時間を測らずに間隔の1秒を待つ
			name:    "first change read again after the update still waits the interval before the second",
			changes: generalChanges,
			responses: []response{
				okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, false), okRepository(1, false, true),
				okRepository(1, false, true), okRepository(1, true, true), okRepository(1, true, true),
			},
			wantMethods:        []string{"GET", "PATCH", "GET", "GET", "GET", "PATCH", "GET"},
			wantBodies:         []map[string]any{nil, {"delete_branch_on_merge": true}, nil, nil, nil, {"allow_auto_merge": true}, nil},
			wantPrecond:        []preconditionCheck{preconditionHolds, preconditionHolds},
			wantResults:        []changeResult{completed, completed},
			wantDelays:         []time.Duration{time.Second, time.Second},
			wantRequestsAtWait: []int{3, 4},
		},
		{
			name:    "second current value changed after the first completes is not attempted",
			changes: generalChanges,
			responses: []response{
				okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
				okRepository(1, true, true),
			},
			wantMethods:        []string{"GET", "PATCH", "GET", "GET"},
			wantBodies:         []map[string]any{nil, {"delete_branch_on_merge": true}, nil, nil},
			wantPrecond:        []preconditionCheck{preconditionHolds, preconditionCurrentValueChanged},
			wantResults:        []changeResult{completed, notAttempted},
			wantDelays:         []time.Duration{time.Second},
			wantRequestsAtWait: []int{3},
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
			wantPrecond:        []preconditionCheck{preconditionHolds, preconditionHolds, preconditionHolds, preconditionHolds},
			wantResults:        []changeResult{completed, completed, completed, completed},
			wantDelays:         []time.Duration{time.Second, time.Second, time.Second},
			wantRequestsAtWait: []int{3, 6, 11},
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
			wantMethods:        []string{"GET", "PATCH", "GET", "GET", "PATCH", "GET"},
			wantBodies:         []map[string]any{nil, {"delete_branch_on_merge": true}, nil, nil, {"allow_auto_merge": true}, nil},
			wantPrecond:        []preconditionCheck{preconditionHolds, preconditionHolds, preconditionNotChecked},
			wantResults:        []changeResult{completed, failed, notAttempted},
			wantDelays:         []time.Duration{time.Second},
			wantRequestsAtWait: []int{3},
		},
		{
			name:    "failed change of two updates stops the workflow permissions change without a read or an update",
			changes: []changeplan.LogicalChange{toSelectedChange(), defaultWorkflowPermissionsChange()},
			responses: []response{
				okRepository(1, false, false), okAllowedActions("local_only", false), noContent,
				okRepository(1, false, false), okAllowedActions("all", false),
				okRepository(1, false, false), okAllowedActions("all", false),
			},
			wantMethods: []string{"GET", "GET", "PUT", "GET", "GET", "GET", "GET"},
			wantBodies:  []map[string]any{nil, nil, {"enabled": true, "allowed_actions": "selected"}, nil, nil, nil, nil},
			wantPrecond: []preconditionCheck{preconditionHolds, preconditionNotChecked},
			wantResults: []changeResult{failed, notAttempted},
			// 2つ目の更新操作の直前の間隔だけで、完了しなかった変更のあとには入らない
			wantDelays:         []time.Duration{time.Second},
			wantRequestsAtWait: []int{3},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)
			changes := tc.changes
			var delays []time.Duration
			var requestsAtWait []int
			after := func(d time.Duration) <-chan time.Time {
				delays = append(delays, d)
				requestsAtWait = append(requestsAtWait, len(server.RequestLines()))
				return elapsedAtOnce(d)
			}

			got := executeChanges(context.Background(), newClient(baseURL), newRepositoryPlan(t), changes, after, &updateInterval{})

			if !slices.Equal(delays, tc.wantDelays) {
				t.Errorf("delays = %v, want %v", delays, tc.wantDelays)
			}
			if !slices.Equal(requestsAtWait, tc.wantRequestsAtWait) {
				t.Errorf("requests received at each wait = %v, want %v", requestsAtWait, tc.wantRequestsAtWait)
			}
			if !slices.Equal(server.Methods(), tc.wantMethods) {
				t.Errorf("requests = %v, want %v", server.Methods(), tc.wantMethods)
			}
			if !reflect.DeepEqual(server.Bodies(), tc.wantBodies) {
				t.Errorf("request bodies = %v, want %v", server.Bodies(), tc.wantBodies)
			}
			if len(got) != len(changes) {
				t.Fatalf("executeChanges() returned %d records, want %d", len(got), len(changes))
			}
			for i, record := range got {
				if !reflect.DeepEqual(record.change, changes[i]) {
					t.Errorf("records[%d].change = %+v, want %+v", i, record.change, changes[i])
				}
				if record.operation(0).precondition != tc.wantPrecond[i] {
					t.Errorf("records[%d].precondition = %v, want %v", i, record.operation(0).precondition, tc.wantPrecond[i])
				}
				if record.result() != tc.wantResults[i] {
					t.Errorf("records[%d].result() = %v, want %v", i, record.result(), tc.wantResults[i])
				}
			}
		})
	}

	// 先行する変更は完了しているため、先行する異常による停止ではなく、次の変更の変更前提の確認不能になる
	t.Run("interruption while waiting for the interval between changes leaves the second not attempted without a request", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		interrupted := func(time.Duration) <-chan time.Time {
			cancel()
			return make(chan time.Time)
		}

		got := executeChanges(ctx, newClient(baseURL), newRepositoryPlan(t), generalChanges, interrupted, &updateInterval{})

		if want := []string{"GET", "PATCH", "GET"}; !slices.Equal(server.Methods(), want) {
			t.Errorf("requests = %v, want %v", server.Methods(), want)
		}
		if got[0].result() != completed || got[1].result() != notAttempted {
			t.Errorf("results = %v, %v, want %v, %v", got[0].result(), got[1].result(), completed, notAttempted)
		}
		if got[1].operation(0).precondition != preconditionUnconfirmed {
			t.Errorf("records[1].precondition = %v, want %v", got[1].operation(0).precondition, preconditionUnconfirmed)
		}
		if !errors.Is(got[1].cause, context.Canceled) {
			t.Errorf("records[1].cause = %v, want it to wrap %v", got[1].cause, context.Canceled)
		}
	})
}

// TestExecuteRepositories は、対象リポジトリを宣言順に実行し、完了以外の変更結果のあとは残りの対象リポジトリへ要求を送らない規則と、
// 更新要求の間隔を対象リポジトリをまたいで保つ規則を確かめる。
// 完了以外の種類ごとの変更結果は executeChange のテストが確かめているため、停止の分岐を誤らせる代表だけを使う。
// 対象リポジトリごとに ID を変え、先頭の計画の ID で変更前提を確かめる実装を失敗させる。
//
//trace:checks INV-004
//trace:checks INV-005
//trace:checks FR-010
//trace:checks FR-013
func TestExecuteRepositories(t *testing.T) {
	forbidden := response{status: http.StatusForbidden, body: `{"message": "Forbidden"}`}
	redirect := response{status: http.StatusTemporaryRedirect, location: "https://example.com/"}
	aam := []changeplan.LogicalChange{allowAutoMergeChange()}
	generalChanges := []changeplan.LogicalChange{deleteBranchOnMergeChange(), allowAutoMergeChange()}
	// completes は、ID が id の対象リポジトリで allowAutoMergeChange が完了する応答。
	completes := func(id int64) []response {
		return []response{okRepository(id, false, false), okRepository(id, true, false), okRepository(id, true, false)}
	}
	completesRequests := func(path string) []string {
		return []string{"GET " + path, "PATCH " + path, "GET " + path}
	}
	const first, second, third = "/repos/owner/first", "/repos/owner/second", "/repos/owner/third"
	type repositoryInput struct {
		name    string
		id      repository.ID
		changes []changeplan.LogicalChange
	}
	cases := []struct {
		name         string
		repositories []repositoryInput
		responses    []response
		wantRequests []string
		// wantRequestsAtWait は、待機ごとに、それを求めた時点で GitHub が受けていた要求の数。待機の位置を示す。
		wantRequestsAtWait []int
		wantResults        []repositoryResult
	}{
		{
			name:               "both repositories complete in the declared order with one interval between them",
			repositories:       []repositoryInput{{"owner/first", 1, aam}, {"owner/second", 2, aam}},
			responses:          slices.Concat(completes(1), completes(2)),
			wantRequests:       slices.Concat(completesRequests(first), completesRequests(second)),
			wantRequestsAtWait: []int{3},
			wantResults:        []repositoryResult{repositoryCompleted, repositoryCompleted},
		},
		{
			name:         "rejected update in the first stops the second without a request",
			repositories: []repositoryInput{{"owner/first", 1, aam}, {"owner/second", 2, aam}},
			responses:    []response{okRepository(1, false, false), forbidden},
			wantRequests: []string{"GET " + first, "PATCH " + first},
			wantResults:  []repositoryResult{repositoryNotCompleted, repositoryStopped},
		},
		{
			name:         "unknown update outcome in the first stops the second without a request",
			repositories: []repositoryInput{{"owner/first", 1, aam}, {"owner/second", 2, aam}},
			responses:    []response{okRepository(1, false, false), redirect, okRepository(1, true, false)},
			wantRequests: []string{"GET " + first, "PATCH " + first, "GET " + first},
			wantResults:  []repositoryResult{repositoryNotCompleted, repositoryStopped},
		},
		{
			// 原因の error を持たない失敗でも止める
			name:         "unexpected outside change in the first stops the second without a request",
			repositories: []repositoryInput{{"owner/first", 1, aam}, {"owner/second", 2, aam}},
			responses:    []response{okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, true)},
			wantRequests: []string{"GET " + first, "PATCH " + first, "GET " + first},
			wantResults:  []repositoryResult{repositoryNotCompleted, repositoryStopped},
		},
		{
			// 更新を一つも開始していなくても止める
			name:         "precondition not holding in the first stops the second without a request",
			repositories: []repositoryInput{{"owner/first", 1, aam}, {"owner/second", 2, aam}},
			responses:    []response{okRepository(2, false, false)},
			wantRequests: []string{"GET " + first},
			wantResults:  []repositoryResult{repositoryNotCompleted, repositoryStopped},
		},
		{
			name:         "second change failing after the first completes in the first repository stops the second repository",
			repositories: []repositoryInput{{"owner/first", 1, generalChanges}, {"owner/second", 2, aam}},
			responses: []response{
				okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
				okRepository(1, false, true), forbidden,
			},
			wantRequests:       []string{"GET " + first, "PATCH " + first, "GET " + first, "GET " + first, "PATCH " + first},
			wantRequestsAtWait: []int{3},
			wantResults:        []repositoryResult{repositoryNotCompleted, repositoryStopped},
		},
		{
			name:         "first change failing in the first repository stops its second change and the second repository",
			repositories: []repositoryInput{{"owner/first", 1, generalChanges}, {"owner/second", 2, generalChanges}},
			responses:    []response{okRepository(1, false, false), forbidden},
			wantRequests: []string{"GET " + first, "PATCH " + first},
			wantResults:  []repositoryResult{repositoryNotCompleted, repositoryStopped},
		},
		{
			name:               "failure in the second of three repositories stops only the third",
			repositories:       []repositoryInput{{"owner/first", 1, aam}, {"owner/second", 2, aam}, {"owner/third", 3, aam}},
			responses:          slices.Concat(completes(1), []response{okRepository(2, false, false), forbidden}),
			wantRequests:       slices.Concat(completesRequests(first), []string{"GET " + second, "PATCH " + second}),
			wantRequestsAtWait: []int{3},
			wantResults:        []repositoryResult{repositoryCompleted, repositoryNotCompleted, repositoryStopped},
		},
		{
			name:               "repository without changes between two is sent no request and the interval is waited once",
			repositories:       []repositoryInput{{"owner/first", 1, aam}, {"owner/second", 2, nil}, {"owner/third", 3, aam}},
			responses:          slices.Concat(completes(1), completes(3)),
			wantRequests:       slices.Concat(completesRequests(first), completesRequests(third)),
			wantRequestsAtWait: []int{3},
			wantResults:        []repositoryResult{repositoryCompleted, repositoryNoChange, repositoryCompleted},
		},
		{
			name:         "repository without changes after a failure stays no change and the next one is stopped",
			repositories: []repositoryInput{{"owner/first", 1, aam}, {"owner/second", 2, nil}, {"owner/third", 3, aam}},
			responses:    []response{okRepository(1, false, false), forbidden},
			wantRequests: []string{"GET " + first, "PATCH " + first},
			wantResults:  []repositoryResult{repositoryNotCompleted, repositoryNoChange, repositoryStopped},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)
			var repositories []repositoryChanges
			for _, r := range tc.repositories {
				repositories = append(repositories, repositoryChanges{plan: newRepositoryPlanOf(t, r.name, r.id), changes: r.changes})
			}
			var requestsAtWait []int
			after := func(d time.Duration) <-chan time.Time {
				requestsAtWait = append(requestsAtWait, len(server.RequestLines()))
				return elapsedAtOnce(d)
			}

			got := executeRepositories(context.Background(), newClient(baseURL), repositories, after)

			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			if !slices.Equal(requestsAtWait, tc.wantRequestsAtWait) {
				t.Errorf("requests received at each wait = %v, want %v", requestsAtWait, tc.wantRequestsAtWait)
			}
			if len(got) != len(repositories) {
				t.Fatalf("executeRepositories() returned %d records, want %d", len(got), len(repositories))
			}
			for i, record := range got {
				if record.target != repositories[i].plan.Target {
					t.Errorf("records[%d].target = %v, want %v", i, record.target, repositories[i].plan.Target)
				}
				if len(record.changes) != len(repositories[i].changes) {
					t.Errorf("records[%d] has %d change records, want %d", i, len(record.changes), len(repositories[i].changes))
				}
				if record.result() != tc.wantResults[i] {
					t.Errorf("records[%d].result() = %v, want %v", i, record.result(), tc.wantResults[i])
				}
			}
		})
	}

	// 先行する対象リポジトリは完了しているため、先行する異常による停止ではなく、次の対象リポジトリの変更前提の確認不能になる
	t.Run("interruption while waiting for the interval between repositories leaves the next not completed and stops the rest", func(t *testing.T) {
		baseURL, server := newSequenceServer(t, completes(1)...)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		interrupted := func(time.Duration) <-chan time.Time {
			cancel()
			return make(chan time.Time)
		}
		repositories := []repositoryChanges{
			{plan: newRepositoryPlanOf(t, "owner/first", 1), changes: aam},
			{plan: newRepositoryPlanOf(t, "owner/second", 2), changes: aam},
			{plan: newRepositoryPlanOf(t, "owner/third", 3), changes: aam},
		}

		got := executeRepositories(ctx, newClient(baseURL), repositories, interrupted)

		if want := completesRequests(first); !slices.Equal(server.RequestLines(), want) {
			t.Errorf("requests = %v, want %v", server.RequestLines(), want)
		}
		want := []repositoryResult{repositoryCompleted, repositoryNotCompleted, repositoryStopped}
		for i, record := range got {
			if record.result() != want[i] {
				t.Errorf("records[%d].result() = %v, want %v", i, record.result(), want[i])
			}
		}
		if got[1].changes[0].operation(0).precondition != preconditionUnconfirmed {
			t.Errorf("records[1] precondition = %v, want %v", got[1].changes[0].operation(0).precondition, preconditionUnconfirmed)
		}
		if !errors.Is(got[1].changes[0].cause, context.Canceled) {
			t.Errorf("records[1] cause = %v, want it to wrap %v", got[1].changes[0].cause, context.Canceled)
		}
	})
}
