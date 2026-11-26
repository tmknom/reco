package apply

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// TestObserveBeforeUpdate は、2つ目の更新操作の直前の観測で、更新要求の間隔を観測より前に待つ規則と、
// 1つ目の反映の遅れを待って観測し直す規則を確かめる。
// 観測し直すのは読み取りだけで、開始してよいのは最後の観測で変更前提が成立したときだけである。
//
//trace:checks INV-004 INV-005 FR-010
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
			want:                    operationRecord{precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.AllowedActions}},
		},
		{
			name:         "policy neither before the first update nor selected is a current value change without reading again",
			change:       toSelectedChange(),
			responses:    []response{okRepository(1, false, false), okAllowedActions("all", false)},
			wantRequests: readNotSelected,
			want:         operationRecord{precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.AllowedActions}},
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

			_, got, err := observeBeforeUpdate(context.Background(), newClient(baseURL), newRepositoryPlan(t), tc.change, 1, observationAreas(tc.change), nil, delays.after, &updateInterval{updateStarted: true})

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

		_, got, err := observeBeforeUpdate(ctx, newClient(baseURL), newRepositoryPlan(t), change, 1, observationAreas(change), nil, interrupted, &updateInterval{updateStarted: true})

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

		_, got, err := observeBeforeUpdate(ctx, newClient(baseURL), newRepositoryPlan(t), change, 1, observationAreas(change), nil, interrupted, &updateInterval{updateStarted: true})

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

// TestObservationAreas は、更新直前と更新後の観測で読む設定の領域を、更新操作の宛先と基準の設定、期待状態の設定から導く規則を確かめる。
func TestObservationAreas(t *testing.T) {
	cases := []struct {
		name    string
		change  changeplan.LogicalChange
		want    []setting.Area
		notWant []setting.Area
	}{
		{
			// General と Secret scanning は同じ更新エンドポイントのため、読まないと比べる変更対象外の設定の観測値がない
			name:    "general change reads the general and secret scanning areas",
			change:  allowAutoMergeChange(),
			want:    []setting.Area{setting.AreaGeneral, setting.AreaSecretScanning},
			notWant: []setting.Area{setting.AllowedActions.Area()},
		},
		{
			name:    "bundled merge method change reads the general and secret scanning areas",
			change:  mergeMethodSwapChange(t),
			want:    []setting.Area{setting.AreaGeneral, setting.AreaSecretScanning},
			notWant: []setting.Area{setting.AllowedActions.Area()},
		},
		{
			name:    "workflow permissions change does not read the secret scanning area",
			change:  defaultWorkflowPermissionsChange(),
			want:    []setting.Area{setting.AreaWorkflowPermissions},
			notWant: []setting.Area{setting.AreaSecretScanning},
		},
		{
			name:   "change to selected reads the actions permissions area",
			change: toSelectedChange(),
			want:   []setting.Area{setting.AllowedActions.Area()},
		},
		{
			name:    "environment change reads the environments area",
			change:  creationChange(t, "production"),
			want:    []setting.Area{setting.AreaEnvironments},
			notWant: []setting.Area{setting.AllowedActions.Area(), setting.AreaSecretScanning},
		},
		{
			// 計画時に存在した Environment の変数は、Environment の存在と ID も基準にする
			name:    "environment variable change reads the environment variables and environments areas",
			change:  environmentVariableValueChange(t, "staging", withVariables(observedEnvironment(7, "staging", setting.DeploymentBranchPolicyAll), github.Variable{Name: "REGION", Value: "a"}), "REGION", "a", "b"),
			want:    []setting.Area{setting.AreaEnvironmentVariables, setting.AreaEnvironments},
			notWant: []setting.Area{setting.AreaRepositoryVariables},
		},
		{
			// 属する先の Environment の存在は、Environment Variables の領域の観測が同じ観測で確かめる
			name:    "variable creation in a created environment reads the environment variables area",
			change:  createdEnvironmentVariableChange(t, "production", "REGION", "a"),
			want:    []setting.Area{setting.AreaEnvironmentVariables},
			notWant: []setting.Area{setting.AreaRepositoryVariables},
		},
		{
			// どの変更予定にも生じない組。基準の設定の領域を読まないと、観測結果にその値がないことを確かめる
			name:   "precondition basis in another area is also read",
			change: singleUpdate(setting.AllowAutoMerge, setting.BoolValue(true), basesOf(setting.DefaultWorkflowPermissions, setting.PermissionLevelValue(setting.PermissionRead))),
			want:   []setting.Area{setting.AllowAutoMerge.Area(), setting.DefaultWorkflowPermissions.Area()},
		},
		{
			// どの変更予定にも生じない組。期待値と比べる設定の領域を読まないと、観測結果にその値がないことを確かめる
			name: "desired setting in another area is also read",
			change: func() changeplan.LogicalChange {
				change := allowAutoMergeChange()
				change.DesiredState = append(change.DesiredState, configuration.ManagedSetting{Unit: setting.Unit{Key: setting.DefaultWorkflowPermissions}, Desired: setting.PermissionLevelValue(setting.PermissionRead)})
				return change
			}(),
			want: []setting.Area{setting.AllowAutoMerge.Area(), setting.DefaultWorkflowPermissions.Area()},
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

//trace:checks INV-004 FR-012
func TestObservationResources(t *testing.T) {
	production, staging := mustEnvironmentName(t, "production"), mustEnvironmentName(t, "staging")
	stagingPolicy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}
	protect, strict := mustRulesetName(t, "protect"), mustRulesetName(t, "strict")
	cases := []struct {
		name   string
		change changeplan.LogicalChange
		want   github.NamedResources
	}{
		{
			name:   "repository setting change names no environment or ruleset",
			change: allowAutoMergeChange(),
		},
		{
			name:   "creation with a mode and patterns names the destination once",
			change: creationWithPatternsChange(t, "production", selectedPolicy(branchPattern("main"))),
			want:   github.NamedResources{Environments: []setting.EnvironmentName{production}},
		},
		{
			name:   "variable creation in a created environment names the environment it belongs to",
			change: createdEnvironmentVariableChange(t, "production", "REGION", "a"),
			want:   github.NamedResources{Environments: []setting.EnvironmentName{production}},
		},
		{
			// 更新操作、関係する現在値、期待状態のどれもが同じ Ruleset を指すため、重複を除くことも確かめる
			name:   "ruleset creation names the ruleset once without an environment",
			change: rulesetCreationChange(t, "protect"),
			want:   github.NamedResources{Rulesets: []setting.RulesetName{protect}},
		},
		{
			// どの変更予定にも生じない組。期待状態の Ruleset を読まないと、更新後に値を比べられないことを確かめる
			name: "ruleset of a desired setting is also named",
			change: func() changeplan.LogicalChange {
				change := rulesetValueUpdateChange(t, "protect")
				change.DesiredState = append(change.DesiredState, configuration.ManagedSetting{
					Unit: setting.Unit{Key: setting.RulesetResource, Ruleset: strict}, Desired: setting.RulesetValue(protectRuleset(setting.RulesetEnforcementActive)),
				})
				return change
			}(),
			want: github.NamedResources{Rulesets: []setting.RulesetName{protect, strict}},
		},
		{
			// どの変更予定にも生じない組。更新操作の宛先の Environment を、期待状態に頼らずに読むことを確かめる
			name: "environment of an update destination is named without its precondition bases and desired state",
			change: func() changeplan.LogicalChange {
				change := creationChange(t, "production")
				change.Operations[0].PreconditionBases = nil
				change.DesiredState = nil
				return change
			}(),
			want: github.NamedResources{Environments: []setting.EnvironmentName{production}},
		},
		{
			// どの変更予定にも生じない組。関係する現在値の Environment を読まないと、変更前提で個別パターンを比べられないことを確かめる
			name:   "environment of a precondition basis is also named",
			change: singleUpdate(setting.AllowAutoMerge, setting.BoolValue(true), []changeplan.PreconditionBasis{{Unit: stagingPolicy, AnticipatedValue: changeplan.AnticipateValue(modeValue(setting.DeploymentBranchPolicyAll))}}),
			want:   github.NamedResources{Environments: []setting.EnvironmentName{staging}},
		},
		{
			// どの変更予定にも生じない組。期待状態の Environment を読まないと、更新後に個別パターンを比べられないことを確かめる
			name: "environment of a desired setting is also named",
			change: func() changeplan.LogicalChange {
				change := allowAutoMergeChange()
				change.DesiredState = append(change.DesiredState, configuration.ManagedSetting{Unit: stagingPolicy, Desired: modeValue(setting.DeploymentBranchPolicyAll)})
				return change
			}(),
			want: github.NamedResources{Environments: []setting.EnvironmentName{staging}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := observationResources(tc.change); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("observationResources() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

//trace:checks INV-001 INV-004 INV-005 FR-010 FR-012 FR-013
func TestExecuteChange(t *testing.T) {
	disconnect := response{disconnect: true}
	serverError := response{status: http.StatusInternalServerError}
	redirect := response{status: http.StatusTemporaryRedirect, location: "https://example.com/"}

	noContent := response{status: http.StatusNoContent}
	// allDelays は、変更の実行の設計が定める、期待状態と一致しないときに観測し直す前に待つ時間。
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
			wantDesired: desiredStateObservationFailed,
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
			wantOutside: []unexpectedOutsideChange{{Setting: string(setting.DeleteBranchOnMerge), Before: "false", After: "true"}},
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
			wantDesired: desiredStateObservationFailed,
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
			wantDesired: desiredStateObservationFailed,
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
			wantOutside: []unexpectedOutsideChange{{Setting: string(setting.DeleteBranchOnMerge), Before: "false", After: "true"}},
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
			wantDesired: desiredStateObservationFailed,
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
			wantOutside: []unexpectedOutsideChange{{Setting: string(setting.CanApprovePullRequestReviews), Before: "false", After: "true"}},
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
		if got.confirmation != desiredStateObservationFailed {
			t.Errorf("confirmation = %v, want %v", got.confirmation, desiredStateObservationFailed)
		}
		if !errors.Is(got.cause, context.Canceled) {
			t.Errorf("cause = %v, want it to wrap %v", got.cause, context.Canceled)
		}
	})
}

// TestExecuteChange_ActionsPermissions は、Actions の実行許可の領域の設定を一つの更新操作で変える論理的な設定変更について、
// 観測と更新の要求の送り先と、観測不能を伴う変更結果を確かめる。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-012 FR-013
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
			wantOutside:  []unexpectedOutsideChange{{Setting: string(setting.SHAPinningRequired), Before: "false", After: "true"}},
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
			// 更新後は Actions 許可ポリシーと sha_pinning_required が観測不能で、保持の確認結果を根拠に保全されるとみなす
			name:   "disabling actions sends only enabled and completes with the policy unavailable after the update",
			change: enabledStateChange(false),
			responses: []response{
				okRepository(1, false, false), okAllowedActions("all", true), noContent,
				okRepository(1, false, false), okActionsDisabled(),
			},
			wantRequests: []string{getRepository, getActionsPermissions, putActionsPermissions, getRepository, getActionsPermissions},
			wantBodies:   []map[string]any{nil, nil, {"enabled": false}, nil, nil},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			// 更新直前は sha_pinning_required が観測不能で、更新後の true と比べない
			name:   "enabling actions sends only enabled and completes with the policy restored as selected",
			change: enabledStateChange(true),
			responses: []response{
				okRepository(1, false, false), okActionsDisabled(), noContent,
				okRepository(1, false, false), okAllowedActions("selected", true), okSelectedActions(false, true, "a/*"),
			},
			wantRequests: []string{getRepository, getActionsPermissions, putActionsPermissions, getRepository, getActionsPermissions, getSelectedActions},
			wantBodies:   []map[string]any{nil, nil, {"enabled": true}, nil, nil, nil},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
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
		{
			name:   "sha_pinning_required change sends enabled true and sha_pinning_required only and completes",
			change: shaPinningRequiredChange(),
			responses: []response{
				okRepository(1, false, false), okAllowedActions("all", false), noContent,
				okRepository(1, false, false), okAllowedActions("all", true),
			},
			wantRequests: []string{getRepository, getActionsPermissions, putActionsPermissions, getRepository, getActionsPermissions},
			wantBodies:   []map[string]any{nil, nil, {"enabled": true, "sha_pinning_required": true}, nil, nil},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			name:   "changed allowed_actions after the sha_pinning_required update fails even if the value matches",
			change: shaPinningRequiredChange(),
			responses: []response{
				okRepository(1, false, false), okAllowedActions("all", false), noContent,
				okRepository(1, false, false), okAllowedActions("local_only", true),
			},
			wantRequests: []string{getRepository, getActionsPermissions, putActionsPermissions, getRepository, getActionsPermissions},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantOutside: []unexpectedOutsideChange{{
				Setting: string(setting.AllowedActions), Before: "all", After: "local_only",
			}},
			wantResult: failed,
		},
		{
			// 計画時に観測した sha_pinning_required が観測不能になったことを、Actions の有効状態より優先して示す
			name:         "sha_pinning_required change with actions disabled since approval reports sha_pinning_required changed",
			change:       shaPinningRequiredChange(),
			responses:    []response{okRepository(1, false, false), okActionsDisabled()},
			wantRequests: []string{getRepository, getActionsPermissions},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  setting.SHAPinningRequired,
			wantResult:   notAttempted,
		},
		{
			// 更新要求が enabled: true を送るため、無効のまま送ると利用者が宣言していない有効化を行う
			name:         "sha_pinning_required update after enabling with actions still disabled sends no update request",
			change:       shaPinningRequiredAfterEnablingChange(),
			responses:    []response{okRepository(1, false, false), okActionsDisabled()},
			wantRequests: []string{getRepository, getActionsPermissions},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  setting.ActionsEnabled,
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
			if got.operation(0).changedSetting != (setting.Unit{Key: tc.wantChanged}) {
				t.Errorf("changedSetting = %q, want %q", got.operation(0).changedSetting.Key, tc.wantChanged)
			}
			if got.operation(0).update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.operation(0).update, tc.wantUpdate)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if got.desiredUnavailable.reason != tc.wantReason {
				t.Errorf("desiredUnavailable.reason = %v, want %v", got.desiredUnavailable.reason, tc.wantReason)
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

// TestExecuteChange_ActionsAccess は、access_level の論理的な設定変更について、観測と更新の要求の送り先と、
// リポジトリが公開に変わって access_level が観測不能になったときの変更結果を確かめる。
//
//trace:checks INV-001 INV-004 FR-010 FR-011 FR-012 FR-013
func TestExecuteChange_ActionsAccess(t *testing.T) {
	const (
		getRepository    = "GET /repos/owner/repo"
		getActionsAccess = "GET /repos/owner/repo/actions/permissions/access"
		putActionsAccess = "PUT /repos/owner/repo/actions/permissions/access"
	)
	noContent := response{status: http.StatusNoContent}
	allDelays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	cases := []struct {
		name         string
		responses    []response
		wantRequests []string
		wantBodies   []map[string]any
		wantDelays   []time.Duration
		wantPrecond  preconditionCheck
		wantChanged  setting.Key
		wantDesired  desiredStateConfirmation
		wantReason   github.UnavailableReason
		wantResult   changeResult
	}{
		{
			name:         "private repository sends only access_level and completes",
			responses:    []response{okPrivateRepository(1), okActionsAccess("none"), noContent, okPrivateRepository(1), okActionsAccess("user")},
			wantRequests: []string{getRepository, getActionsAccess, putActionsAccess, getRepository, getActionsAccess},
			wantBodies:   []map[string]any{nil, nil, {"access_level": "user"}, nil, nil},
			wantPrecond:  preconditionHolds,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			// 観測値だった access_level が観測不能になったことは、関係する現在値の変化に当たる
			name:         "repository made public since approval sends no update request",
			responses:    []response{okRepository(1, false, false)},
			wantRequests: []string{getRepository},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  setting.ActionsAccessLevel,
			wantResult:   notAttempted,
		},
		{
			name: "repository made public after the update is a failed confirmation with the public reason",
			responses: []response{
				okPrivateRepository(1), okActionsAccess("none"), noContent,
				okRepository(1, false, false), okRepository(1, false, false), okRepository(1, false, false),
				okRepository(1, false, false), okRepository(1, false, false), okRepository(1, false, false),
			},
			wantRequests: []string{getRepository, getActionsAccess, putActionsAccess, getRepository, getRepository, getRepository, getRepository, getRepository, getRepository},
			wantDelays:   allDelays,
			wantPrecond:  preconditionHolds,
			wantDesired:  desiredStateUnavailable,
			wantReason:   github.UnavailablePublicRepository,
			wantResult:   failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)
			delays := &delayRecorder{}

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), accessLevelChange(), delays.after, &updateInterval{})

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
			if got.operation(0).changedSetting != (setting.Unit{Key: tc.wantChanged}) {
				t.Errorf("changedSetting = %q, want %q", got.operation(0).changedSetting.Key, tc.wantChanged)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if got.desiredUnavailable.reason != tc.wantReason {
				t.Errorf("desiredUnavailable.reason = %v, want %v", got.desiredUnavailable.reason, tc.wantReason)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
		})
	}
}

// TestExecuteChange_ForkPullRequestApproval は、承認ポリシーの論理的な設定変更について、観測と更新の要求の送り先と本文、
// 更新の直後の観測が更新前の値を返したときの観測し直し、リポジトリが非公開に変わったときの変更結果を確かめる。
//
//trace:checks INV-001 INV-004 FR-010 FR-011 FR-012
func TestExecuteChange_ForkPullRequestApproval(t *testing.T) {
	const (
		getRepository              = "GET /repos/owner/repo"
		getForkPullRequestApproval = "GET /repos/owner/repo/actions/permissions/fork-pr-contributor-approval"
		putForkPullRequestApproval = "PUT /repos/owner/repo/actions/permissions/fork-pr-contributor-approval"
	)
	public := okRepository(1, false, false)
	cases := []struct {
		name         string
		responses    []response
		wantRequests []string
		wantBodies   []map[string]any
		wantDelays   []time.Duration
		wantPrecond  preconditionCheck
		wantChanged  setting.Key
		wantResult   changeResult
	}{
		{
			// GitHub は更新の直後の取得で更新前の値を返すことがある (GitHub Actions 権限 API 仕様の承認ポリシーの更新)
			name: "first read after the update still returns the previous policy and the next read completes",
			responses: []response{
				public, okForkPullRequestApproval("first_time_contributors"), {status: http.StatusNoContent},
				public, okForkPullRequestApproval("first_time_contributors"), public, okForkPullRequestApproval("all_external_contributors"),
			},
			wantRequests: []string{
				getRepository, getForkPullRequestApproval, putForkPullRequestApproval,
				getRepository, getForkPullRequestApproval, getRepository, getForkPullRequestApproval,
			},
			wantBodies:  []map[string]any{nil, nil, {"approval_policy": "all_external_contributors"}, nil, nil, nil, nil},
			wantDelays:  []time.Duration{time.Second},
			wantPrecond: preconditionHolds,
			wantResult:  completed,
		},
		{
			// 観測値だった承認ポリシーが観測不能になったことは、関係する現在値の変化に当たる
			name:         "repository made private since approval sends no update request",
			responses:    []response{okPrivateRepository(1)},
			wantRequests: []string{getRepository},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  setting.ForkPullRequestApproval,
			wantResult:   notAttempted,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)
			delays := &delayRecorder{}

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), forkPullRequestApprovalChange(), delays.after, &updateInterval{})

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
			if got.operation(0).changedSetting != (setting.Unit{Key: tc.wantChanged}) {
				t.Errorf("changedSetting = %q, want %q", got.operation(0).changedSetting.Key, tc.wantChanged)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
		})
	}
}

// TestExecuteChange_RetentionDays は、保持日数の論理的な設定変更の完了と、
// 承認のあとに上限が下がって GitHub が拒否したときに、再送せず失敗とすることを確かめる。
//
//trace:checks INV-004 INV-005 FR-010 FR-011 FR-013
func TestExecuteChange_RetentionDays(t *testing.T) {
	const (
		getRepository = "GET /repos/owner/repo"
		getRetention  = "GET /repos/owner/repo/actions/permissions/artifact-and-log-retention"
		putRetention  = "PUT /repos/owner/repo/actions/permissions/artifact-and-log-retention"
	)
	private := okPrivateRepository(1)
	cases := []struct {
		name         string
		responses    []response
		wantRequests []string
		wantUpdate   github.UpdateOutcome
		wantResult   changeResult
	}{
		{
			name:         "days within the maximum is updated and completes",
			responses:    []response{private, okRetention(90, 400), {status: http.StatusNoContent}, private, okRetention(120, 400)},
			wantRequests: []string{getRepository, getRetention, putRetention, getRepository, getRetention},
			wantUpdate:   github.UpdateSucceeded,
			wantResult:   completed,
		},
		{
			name:         "maximum lowered below the desired days since approval is rejected by GitHub and fails without resending",
			responses:    []response{private, okRetention(90, 90), {status: http.StatusUnprocessableEntity}},
			wantRequests: []string{getRepository, getRetention, putRetention},
			wantUpdate:   github.UpdateRejected,
			wantResult:   failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), retentionDaysChange(), (&delayRecorder{}).after, &updateInterval{})

			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			if got.operation(0).update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.operation(0).update, tc.wantUpdate)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
		})
	}
}

// TestExecuteChange_OIDCSubject は、subject のカスタマイズの論理的な設定変更について、
// 3項目のうち更新要求が送る項目と、省略した項目の承認時との違いを変更前提が見ないこと、key の並びの違いは見ることを確かめる。
//
//trace:checks INV-004 FR-010 FR-011 FR-012
func TestExecuteChange_OIDCSubject(t *testing.T) {
	const (
		getRepository = "GET /repos/owner/repo"
		getSubject    = "GET /repos/owner/repo/actions/oidc/customization/sub"
		putSubject    = "PUT /repos/owner/repo/actions/oidc/customization/sub"
	)
	private := okPrivateRepository(1)
	approved := okOIDCSubject(githubtest.OIDCSubjectBody{UseDefault: true})
	declared := okOIDCSubject(githubtest.OIDCSubjectBody{IncludeClaimKeys: []string{"repo", "context"}})
	created := response{status: http.StatusCreated, body: `{}`}
	cases := []struct {
		name         string
		change       changeplan.LogicalChange
		responses    []response
		wantRequests []string
		wantBodies   []map[string]any
		wantPrecond  preconditionCheck
		wantDesired  desiredStateConfirmation
		wantResult   changeResult
	}{
		{
			name:         "update sends use_default and the keys only and completes",
			change:       oidcSubjectChange(),
			responses:    []response{private, approved, created, private, declared},
			wantRequests: []string{getRepository, getSubject, putSubject, getRepository, getSubject},
			wantBodies:   []map[string]any{nil, nil, {"use_default": false, "include_claim_keys": []any{"repo", "context"}}, nil, nil},
			wantPrecond:  preconditionHolds,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			name:   "omitted item differing from the approved read still updates and completes",
			change: oidcSubjectChange(),
			responses: []response{
				private, okOIDCSubject(githubtest.OIDCSubjectBody{UseDefault: true, UseImmutableSubject: true}), created,
				private, okOIDCSubject(githubtest.OIDCSubjectBody{IncludeClaimKeys: []string{"repo", "context"}, UseImmutableSubject: true}),
			},
			wantRequests: []string{getRepository, getSubject, putSubject, getRepository, getSubject},
			wantPrecond:  preconditionHolds,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), tc.change, (&delayRecorder{}).after, &updateInterval{})

			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			if tc.wantBodies != nil && !reflect.DeepEqual(server.Bodies(), tc.wantBodies) {
				t.Errorf("request bodies = %v, want %v", server.Bodies(), tc.wantBodies)
			}
			if got.operation(0).precondition != tc.wantPrecond {
				t.Errorf("precondition = %v, want %v", got.operation(0).precondition, tc.wantPrecond)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
		})
	}
}

// TestExecuteChange_PrivateForkPullRequestWorkflows は、実行条件の論理的な設定変更について、4項目を一つの更新要求で送ることと、
// 更新直前と更新後に4項目の組の全体を比べること、更新の直後の観測が更新前の値を返したときの観測し直し、
// リポジトリが公開に変わったときの変更結果を確かめる。
//
//trace:checks INV-001 INV-004 FR-010 FR-011 FR-012
func TestExecuteChange_PrivateForkPullRequestWorkflows(t *testing.T) {
	const (
		getRepository = "GET /repos/owner/repo"
		getConditions = "GET /repos/owner/repo/actions/permissions/fork-pr-workflows-private-repos"
		putConditions = "PUT /repos/owner/repo/actions/permissions/fork-pr-workflows-private-repos"
	)
	private := okPrivateRepository(1)
	approved := okPrivateForkPullRequestWorkflows(githubtest.PrivateForkPullRequestWorkflowsBody{})
	declared := okPrivateForkPullRequestWorkflows(githubtest.PrivateForkPullRequestWorkflowsBody{RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true})
	// 宣言が変えない send_write_tokens_to_workflows だけが期待値と異なる
	writeTokensToo := okPrivateForkPullRequestWorkflows(githubtest.PrivateForkPullRequestWorkflowsBody{RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true, SendWriteTokensToWorkflows: true})
	allDelays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
	cases := []struct {
		name         string
		responses    []response
		wantRequests []string
		wantBodies   []map[string]any
		wantDelays   []time.Duration
		wantPrecond  preconditionCheck
		wantChanged  setting.Key
		wantDesired  desiredStateConfirmation
		wantResult   changeResult
	}{
		{
			// GitHub は更新の直後の取得で更新前の値を返すことがある (GitHub Actions 権限 API 仕様の実行条件の更新)
			name: "first read after the update still returns the previous conditions and the next read completes",
			responses: []response{
				private, approved, noContent(),
				private, approved, private, declared,
			},
			wantRequests: []string{
				getRepository, getConditions, putConditions,
				getRepository, getConditions, getRepository, getConditions,
			},
			wantBodies: []map[string]any{nil, nil, {
				"run_workflows_from_fork_pull_requests": true, "send_write_tokens_to_workflows": false,
				"send_secrets_and_variables": true, "require_approval_for_fork_pr_workflows": false,
			}, nil, nil, nil, nil},
			wantDelays:  []time.Duration{time.Second},
			wantPrecond: preconditionHolds,
			wantDesired: desiredStateMatches,
			wantResult:  completed,
		},
		{
			// 4項目の組が一つの設定の値のため、宣言が変えない項目の変化も関係する現在値の変化に当たる
			name: "item outside the declared change changed since approval sends no update request",
			responses: []response{
				private, okPrivateForkPullRequestWorkflows(githubtest.PrivateForkPullRequestWorkflowsBody{RequireApprovalForForkPullRequestWorkflows: true}),
			},
			wantRequests: []string{getRepository, getConditions},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  setting.PrivateForkPullRequestWorkflows,
			wantResult:   notAttempted,
		},
		{
			// 観測値だった実行条件が観測不能になったことは、関係する現在値の変化に当たる。承認ポリシーの取得は送らない
			name:         "repository made public since approval sends no update request",
			responses:    []response{okRepository(1, false, false)},
			wantRequests: []string{getRepository},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  setting.PrivateForkPullRequestWorkflows,
			wantResult:   notAttempted,
		},
		{
			name: "item outside the declared change differing after the update is a failed confirmation",
			responses: []response{
				private, approved, noContent(),
				private, writeTokensToo, private, writeTokensToo, private, writeTokensToo,
				private, writeTokensToo, private, writeTokensToo, private, writeTokensToo,
			},
			wantRequests: []string{
				getRepository, getConditions, putConditions,
				getRepository, getConditions, getRepository, getConditions, getRepository, getConditions,
				getRepository, getConditions, getRepository, getConditions, getRepository, getConditions,
			},
			wantDelays:  allDelays,
			wantPrecond: preconditionHolds,
			wantDesired: desiredStateMismatch,
			wantResult:  failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)
			delays := &delayRecorder{}

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), privateForkPullRequestWorkflowsChange(), delays.after, &updateInterval{})

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
			if got.operation(0).changedSetting != (setting.Unit{Key: tc.wantChanged}) {
				t.Errorf("changedSetting = %q, want %q", got.operation(0).changedSetting.Key, tc.wantChanged)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
		})
	}
}

// TestExecuteChange_TwoUpdates は、二つの更新操作からなる論理的な設定変更について、
// 更新操作ごとの直前の観測と変更前提、先行する更新操作が成功したときだけ後続へ進むこと、更新後の確認のつなぎ目を確かめる。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-012 FR-013
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
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), {precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.AllowedActions}}},
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
			wantOutside:    []unexpectedOutsideChange{{Setting: string(setting.SHAPinningRequired), Before: "false", After: "true"}},
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
			wantOutside:    []unexpectedOutsideChange{{Setting: string(setting.SHAPinningRequired), Before: "false", After: "true"}},
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
//trace:checks INV-004 INV-005 FR-010 FR-012 FR-013
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
//trace:checks INV-004 INV-005 FR-010 FR-013
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

// TestSendUpdate は、更新操作ごとに送る更新要求の宛先と本文を確かめる。
// Environment には、作成は宣言の綴りへ、存在する Environment には更新直前の観測の綴りへ、deployment_branch_policy だけを送る。
// Environment Variable は、属する Environment の更新直前の観測の綴りへ送り、Repository Variables の更新エンドポイントへは送らない。
//
//trace:checks INV-001 INV-004
func TestSendUpdate(t *testing.T) {
	staging := observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyAll)
	mainBranch, mainTag := branchPattern("main"), tagPattern("main")
	// 方式の PUT、POST v* の順
	toSelectedPatterns := deploymentPatternsChange(t, "staging", staging, selectedPolicy(tagPattern("v*")))
	// DELETE tag main だけ。計画時の ID は 10 と 11
	removeMainTag := deploymentPatternsChange(t, "staging", selectedEnvironment(7, "Staging", []setting.DeploymentPattern{mainBranch, mainTag}, 10, 11), selectedPolicy(mainBranch))
	// 宣言の production と綴りを変え、観測の綴りを宛先にすることを確かめる
	productionRegion := withVariables(observedEnvironment(7, "Production", setting.DeploymentBranchPolicyAll), github.Variable{Name: "REGION", Value: "us-east-1"})
	cases := []struct {
		name      string
		operation changeplan.UpdateOperation
		observed  github.ObservationResult
		response  response
		want      githubtest.Request
	}{
		{
			name:      "repository setting is sent by its update request",
			operation: allowAutoMergeChange().Operations[0],
			response:  okRepository(1, true, false),
			want:      githubtest.Request{Method: http.MethodPatch, Path: "/repos/owner/repo", Body: map[string]any{"allow_auto_merge": true}},
		},
		{
			name:      "merge methods are sent together in one request",
			operation: mergeMethodSwapChange(t).Operations[0],
			response:  okRepositoryBody(githubtest.RepositoryBody{ID: 1, AllowSquashMerge: true}),
			want: githubtest.Request{Method: http.MethodPatch, Path: "/repos/owner/repo", Body: map[string]any{
				"allow_squash_merge": true, "allow_merge_commit": false,
			}},
		},
		{
			name:      "creation without a mode is sent to the declared name without a body",
			operation: creationChange(t, "production").Operations[0],
			observed:  github.ObservationResult{Environments: environments(staging)},
			response:  okEnvironment(githubtest.EnvironmentBody{ID: 8, Name: "production"}),
			want:      githubtest.Request{Method: http.MethodPut, Path: "/repos/owner/repo/environments/production"},
		},
		{
			name:      "creation with a mode is sent with only the mode",
			operation: creationWithModeChange(t, "production", setting.DeploymentBranchPolicyProtectedBranches).Operations[0],
			observed:  github.ObservationResult{Environments: environments()},
			response:  okEnvironment(githubtest.EnvironmentBody{ID: 8, Name: "production", DeploymentBranchPolicy: protectedBranchesBody()}),
			want: githubtest.Request{Method: http.MethodPut, Path: "/repos/owner/repo/environments/production", Body: map[string]any{
				"deployment_branch_policy": map[string]any{"protected_branches": true, "custom_branch_policies": false},
			}},
		},
		{
			name:      "mode change is sent to the observed spelling with only the mode",
			operation: modeChange(t, "staging", staging, setting.DeploymentBranchPolicyAll).Operations[0],
			observed:  github.ObservationResult{Environments: environments(staging)},
			response:  okEnvironment(githubtest.EnvironmentBody{ID: 7, Name: "Staging"}),
			want:      githubtest.Request{Method: http.MethodPut, Path: "/repos/owner/repo/environments/Staging", Body: map[string]any{"deployment_branch_policy": nil}},
		},
		{
			name:      "pattern addition is sent to the observed spelling with only the type and the name",
			operation: toSelectedPatterns.Operations[1],
			observed:  github.ObservationResult{Environments: environments(selectedEnvironment(7, "Staging", nil))},
			response:  okDeploymentPattern(githubtest.DeploymentPatternBody{ID: 30, Name: "v*", Type: "tag"}),
			want: githubtest.Request{Method: http.MethodPost, Path: "/repos/owner/repo/environments/Staging/deployment-branch-policies", Body: map[string]any{
				"name": "v*", "type": "tag",
			}},
		},
		{
			// 同じ名前で種類の違う個別パターンを並べ、種類と名前の組で ID を引くことを確かめる
			name:      "pattern removal is sent to the ID of the pattern in the read before the update",
			operation: removeMainTag.Operations[0],
			observed:  github.ObservationResult{Environments: environments(selectedEnvironment(7, "Staging", []setting.DeploymentPattern{mainBranch, mainTag}, 20, 21))},
			response:  noContent(),
			want:      githubtest.Request{Method: http.MethodDelete, Path: "/repos/owner/repo/environments/Staging/deployment-branch-policies/21"},
		},
		{
			// 値は引用符で囲んだ表示の表記ではなく、宣言した文字列のまま送る
			name:      "variable creation is sent with the declared name and value",
			operation: variableCreationChange(t, "GO_VERSION", "1.27\n").Operations[0],
			observed:  github.ObservationResult{RepositoryVariables: variables()},
			response:  createdVariable(),
			want:      githubtest.Request{Method: http.MethodPost, Path: "/repos/owner/repo/actions/variables", Body: map[string]any{"name": "GO_VERSION", "value": "1.27\n"}},
		},
		{
			name:      "variable value change is sent to the declared name with only the value",
			operation: variableValueChange(t, "GO_VERSION", "1.26", "1.27\n").Operations[0],
			observed:  github.ObservationResult{RepositoryVariables: variables(github.Variable{Name: "go_version", Value: "1.26"})},
			response:  noContent(),
			want:      githubtest.Request{Method: http.MethodPatch, Path: "/repos/owner/repo/actions/variables/GO_VERSION", Body: map[string]any{"value": "1.27\n"}},
		},
		{
			name:      "environment variable creation is sent to the observed spelling of the environment with the declared name and value",
			operation: createdEnvironmentVariableChange(t, "production", "REGION", "ap-northeast-1\n").Operations[0],
			observed:  github.ObservationResult{Environments: environments(withVariables(observedEnvironment(8, "Production", setting.DeploymentBranchPolicyAll)))},
			response:  createdVariable(),
			want: githubtest.Request{Method: http.MethodPost, Path: "/repos/owner/repo/environments/Production/variables", Body: map[string]any{
				"name": "REGION", "value": "ap-northeast-1\n",
			}},
		},
		{
			name:      "environment variable value change is sent to the observed spelling of the environment with only the value",
			operation: environmentVariableValueChange(t, "production", productionRegion, "REGION", "us-east-1", "ap-northeast-1\n").Operations[0],
			observed:  github.ObservationResult{Environments: environments(productionRegion)},
			response:  noContent(),
			want:      githubtest.Request{Method: http.MethodPatch, Path: "/repos/owner/repo/environments/Production/variables/REGION", Body: map[string]any{"value": "ap-northeast-1\n"}},
		},
		{
			name:      "ruleset creation is sent with the declared name and the whole declared value",
			operation: rulesetCreationChange(t, "Protect").Operations[0],
			observed:  github.ObservationResult{},
			response:  createdRuleset(protectRulesetBody(42, "Protect", setting.RulesetEnforcementActive)),
			want: githubtest.Request{Method: http.MethodPost, Path: "/repos/owner/repo/rulesets", Body: map[string]any{
				"name": "Protect", "target": "branch", "enforcement": "active",
				"conditions":    map[string]any{"ref_name": map[string]any{"include": []any{"~DEFAULT_BRANCH"}, "exclude": []any{}}},
				"bypass_actors": []any{map[string]any{"actor_type": "RepositoryRole", "actor_id": float64(5), "bypass_mode": "always"}},
				"rules":         []any{map[string]any{"type": "deletion"}},
			}},
		},
		{
			// 宛先は更新直前の観測から引かず、承認時に予定した ID にする。観測を空にし、観測から引く実装を失敗させる
			name:      "ruleset value update is sent to the approved ID with the whole declared value and without the name",
			operation: rulesetValueUpdateChange(t, "protect").Operations[0],
			observed:  github.ObservationResult{},
			response:  okRuleset(protectRulesetBody(42, "protect", setting.RulesetEnforcementActive)),
			want: githubtest.Request{Method: http.MethodPut, Path: "/repos/owner/repo/rulesets/42", Body: map[string]any{
				"target": "branch", "enforcement": "active",
				"conditions":    map[string]any{"ref_name": map[string]any{"include": []any{"~DEFAULT_BRANCH"}, "exclude": []any{}}},
				"bypass_actors": []any{map[string]any{"actor_type": "RepositoryRole", "actor_id": float64(5), "bypass_mode": "always"}},
				"rules":         []any{map[string]any{"type": "deletion"}},
			}},
		},
		{
			name:      "variable deletion is sent to the declared name without a body",
			operation: variableDeletionChange(t, "GO_VERSION", "1.26").Operations[0],
			observed:  github.ObservationResult{RepositoryVariables: variables(github.Variable{Name: "GO_VERSION", Value: "1.26"})},
			response:  noContent(),
			want:      githubtest.Request{Method: http.MethodDelete, Path: "/repos/owner/repo/actions/variables/GO_VERSION"},
		},
		{
			name:      "environment variable deletion is sent to the observed spelling of the environment without a body",
			operation: environmentVariableDeletionChange(t, "production", productionRegion, "REGION", "us-east-1").Operations[0],
			observed:  github.ObservationResult{Environments: environments(productionRegion)},
			response:  noContent(),
			want:      githubtest.Request{Method: http.MethodDelete, Path: "/repos/owner/repo/environments/Production/variables/REGION"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.response)

			got, err := sendUpdate(context.Background(), newClient(baseURL), mustReference(t, "owner/repo"), tc.operation, tc.observed)

			if got != github.UpdateSucceeded || err != nil {
				t.Errorf("sendUpdate() = %v, %v, want succeeded without an error", got, err)
			}
			requests := server.Requests()
			if len(requests) != 1 {
				t.Fatalf("requests = %v, want one", server.RequestLines())
			}
			requests[0].Authorization = ""
			if !reflect.DeepEqual(requests[0], tc.want) {
				t.Errorf("request = %+v, want %+v", requests[0], tc.want)
			}
		})
	}

	// 方式を伴う作成は方式の更新と同じ要求を送るため、利用者が承認した作成として失敗を示すかで送り分けを確かめる
	t.Run("rejected creation with a mode is reported as a creation", func(t *testing.T) {
		baseURL, _ := newSequenceServer(t, response{status: http.StatusUnprocessableEntity})
		operation := creationWithModeChange(t, "production", setting.DeploymentBranchPolicyProtectedBranches).Operations[0]

		_, err := sendUpdate(context.Background(), newClient(baseURL), mustReference(t, "owner/repo"), operation, github.ObservationResult{Environments: environments()})

		if err == nil || !strings.HasPrefix(err.Error(), `create environment with deployment branch policy "production" `) {
			t.Errorf("sendUpdate() error = %v, want it to name the creation", err)
		}
	})

	// 変更前提が成立した観測には取り除く個別パターンが必ずあるため、ない形は不変条件の破れになる
	panicCases := []struct {
		name      string
		operation changeplan.UpdateOperation
		observed  github.ObservationResult
	}{
		{
			name:      "pattern removal without the pattern in the read before the update panics",
			operation: removeMainTag.Operations[0],
			observed:  github.ObservationResult{Environments: environments(selectedEnvironment(7, "Staging", []setting.DeploymentPattern{mainBranch}, 20))},
		},
		{
			// apply は担当者の集合の更新要求をまだ持たず、承認の前に止めるため、届いたら要求を送らずに止める
			name: "required reviewers update panics without a request",
			operation: changeplan.UpdateOperation{
				Kind: changeplan.RequiredReviewersUpdate, Unit: setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: mustEnvironmentName(t, "staging")},
				Value: setting.ReviewerSetValue(setting.ReviewerSetOf(nil)),
			},
			observed: github.ObservationResult{Environments: environments(selectedEnvironment(7, "Staging", nil))},
		},
		{
			name: "undefined kind of an update operation panics",
			operation: func() changeplan.UpdateOperation {
				o := toSelectedPatterns.Operations[1]
				o.Kind = changeplan.RulesetValueUpdate + 1
				return o
			}(),
			observed: github.ObservationResult{Environments: environments(selectedEnvironment(7, "Staging", nil))},
		},
	}
	for _, tc := range panicCases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t)
			defer func() {
				if recover() == nil {
					t.Error("sendUpdate() did not panic")
				}
				if len(server.RequestLines()) != 0 {
					t.Errorf("requests = %v, want none", server.RequestLines())
				}
			}()
			_, _ = sendUpdate(context.Background(), newClient(baseURL), mustReference(t, "owner/repo"), tc.operation, tc.observed)
		})
	}
}

// TestEnvironmentDestination は、Environment の更新要求の宛先の綴りを確かめる。
//
//trace:checks INV-001 INV-004
func TestEnvironmentDestination(t *testing.T) {
	cases := []struct {
		name     string
		declared string
		existing []github.Environment
		want     string
	}{
		{name: "existing environment is addressed by the observed spelling", declared: "production", existing: []github.Environment{observedEnvironment(7, "Production", setting.DeploymentBranchPolicyAll)}, want: "Production"},
		{name: "absent environment is addressed by the declared spelling", declared: "Production", existing: []github.Environment{observedEnvironment(7, "staging", setting.DeploymentBranchPolicyAll)}, want: "Production"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{Environments: environments(tc.existing...)}
			if got := environmentDestination(mustEnvironmentName(t, tc.declared), observed); got != tc.want {
				t.Errorf("environmentDestination() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestExecuteChange_Environments は、Environment の作成と方式の変更について、更新直前の観測、更新要求、更新後の確認のつなぎ目を確かめる。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-011 FR-012 FR-013
func TestExecuteChange_Environments(t *testing.T) {
	const (
		getRepository    = "GET /repos/owner/repo"
		getEnvironments  = "GET /repos/owner/repo/environments"
		putProduction    = "PUT /repos/owner/repo/environments/production"
		putStagingTarget = "PUT /repos/owner/repo/environments/staging"
	)
	waitTimer := 30
	production := githubtest.EnvironmentBody{ID: 8, Name: "production", DeploymentBranchPolicy: protectedBranchesBody()}
	stagingAll := githubtest.EnvironmentBody{ID: 7, Name: "staging", WaitTimer: &waitTimer}
	stagingProtected := githubtest.EnvironmentBody{ID: 7, Name: "staging", DeploymentBranchPolicy: protectedBranchesBody(), WaitTimer: &waitTimer}
	creation := creationWithModeChange(t, "production", setting.DeploymentBranchPolicyProtectedBranches)
	approvedStaging := observedEnvironment(7, "staging", setting.DeploymentBranchPolicyAll)
	approvedStaging.Protection.WaitTimer = &waitTimer
	stagingMode := modeChange(t, "staging", approvedStaging, setting.DeploymentBranchPolicyProtectedBranches)
	cases := []struct {
		name         string
		change       changeplan.LogicalChange
		responses    []response
		wantRequests []string
		wantDelays   []time.Duration
		wantPrecond  preconditionCheck
		wantChanged  setting.Unit
		wantUpdate   github.UpdateOutcome
		wantDesired  desiredStateConfirmation
		wantOutside  []unexpectedOutsideChange
		wantResult   changeResult
	}{
		{
			name:   "creation absent before the update is sent once and completes",
			change: creation,
			responses: []response{
				okRepository(1, false, false), okEnvironments(stagingAll), okEnvironment(production),
				okRepository(1, false, false), okEnvironments(production, stagingAll),
			},
			wantRequests: []string{getRepository, getEnvironments, putProduction, getRepository, getEnvironments},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			// 一覧の読み落としで作成と計画しても、更新直前の観測で存在を確かめ、既存の Environment を上書きしない
			name:         "creation with the same name existing before the update sends no update request",
			change:       creation,
			responses:    []response{okRepository(1, false, false), okEnvironments(githubtest.EnvironmentBody{ID: 9, Name: "Production"})},
			wantRequests: []string{getRepository, getEnvironments},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "production")},
			wantResult:   notAttempted,
		},
		{
			name:   "creation answered with 502 is not sent again and is read once for diagnosis",
			change: creation,
			responses: []response{
				okRepository(1, false, false), okEnvironments(), {status: http.StatusBadGateway},
				okRepository(1, false, false), okEnvironments(production),
			},
			wantRequests: []string{getRepository, getEnvironments, putProduction, getRepository, getEnvironments},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateUnknown,
			wantDesired:  desiredStateMatches,
			wantResult:   failed,
		},
		{
			name:   "creation not yet reflected after the update is read again and completes",
			change: creation,
			responses: []response{
				okRepository(1, false, false), okEnvironments(), okEnvironment(production),
				okRepository(1, false, false), okEnvironments(), okRepository(1, false, false), okEnvironments(production),
			},
			wantRequests: []string{getRepository, getEnvironments, putProduction, getRepository, getEnvironments, getRepository, getEnvironments},
			wantDelays:   []time.Duration{time.Second},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			name:   "mode change with wait_timer changed after the update fails even if the mode matches",
			change: stagingMode,
			responses: []response{
				okRepository(1, false, false), okEnvironments(stagingAll), okEnvironment(stagingProtected),
				okRepository(1, false, false), okEnvironments(githubtest.EnvironmentBody{ID: 7, Name: "staging", DeploymentBranchPolicy: protectedBranchesBody()}),
			},
			wantRequests: []string{getRepository, getEnvironments, putStagingTarget, getRepository, getEnvironments},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantOutside:  []unexpectedOutsideChange{{Setting: "environments.staging.wait_timer", Before: "30", After: "none"}},
			wantResult:   failed,
		},
		{
			name:         "mode change of an environment recreated with another ID sends no update request",
			change:       stagingMode,
			responses:    []response{okRepository(1, false, false), okEnvironments(githubtest.EnvironmentBody{ID: 9, Name: "staging", WaitTimer: &waitTimer})},
			wantRequests: []string{getRepository, getEnvironments},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "staging")},
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
			if !slices.Equal(delays.delays, tc.wantDelays) {
				t.Errorf("delays = %v, want %v", delays.delays, tc.wantDelays)
			}
			if o := got.operation(0); o.precondition != tc.wantPrecond || o.changedSetting != tc.wantChanged {
				t.Errorf("precondition = %v, %+v, want %v, %+v", o.precondition, o.changedSetting, tc.wantPrecond, tc.wantChanged)
			}
			if tc.wantPrecond == preconditionHolds && got.operation(0).update != tc.wantUpdate {
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
		})
	}
}

// TestExecuteChange_RepositoryVariables は、変数の作成、値の変更、削除を、一覧の読み直しで確かめて一度だけ送るつなぎ目を確かめる。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-011 FR-012 FR-013
func TestExecuteChange_RepositoryVariables(t *testing.T) {
	const (
		getRepository  = "GET /repos/owner/repo"
		getVariables   = "GET /repos/owner/repo/actions/variables"
		postVariable   = "POST /repos/owner/repo/actions/variables"
		patchVariable  = "PATCH /repos/owner/repo/actions/variables/GO_VERSION"
		deleteVariable = "DELETE /repos/owner/repo/actions/variables/LEGACY"
	)
	declared := githubtest.VariableBody{Name: "GO_VERSION", Value: "1.27"}
	other := githubtest.VariableBody{Name: "OTHER", Value: "a"}
	legacy := githubtest.VariableBody{Name: "LEGACY", Value: "old"}
	creation := variableCreationChange(t, "GO_VERSION", "1.27")
	deletion := variableDeletionChange(t, "LEGACY", "old")
	cases := []struct {
		name         string
		change       changeplan.LogicalChange
		responses    []response
		wantRequests []string
		wantPrecond  preconditionCheck
		wantUpdate   github.UpdateOutcome
		wantDesired  desiredStateConfirmation
		wantResult   changeResult
	}{
		{
			name:   "creation absent before the update is sent once and completes",
			change: creation,
			responses: []response{
				okRepository(1, false, false), okVariables(other), createdVariable(),
				okRepository(1, false, false), okVariables(other, declared),
			},
			wantRequests: []string{getRepository, getVariables, postVariable, getRepository, getVariables},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			// GitHub は同じ名前の作成を 409 で拒否するが、拒否に頼らず更新直前の観測で止める
			name:         "creation with the same name existing before the update sends no update request",
			change:       creation,
			responses:    []response{okRepository(1, false, false), okVariables(declared)},
			wantRequests: []string{getRepository, getVariables},
			wantPrecond:  preconditionCurrentValueChanged,
			wantResult:   notAttempted,
		},
		{
			name:   "creation rejected with 409 fails without reading after the update",
			change: creation,
			responses: []response{
				okRepository(1, false, false), okVariables(), {status: http.StatusConflict, body: `{"message":"Already exists - Variable already exists"}`},
			},
			wantRequests: []string{getRepository, getVariables, postVariable},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateRejected,
			wantResult:   failed,
		},
		{
			name:   "creation answered with 502 is not sent again and is read once for diagnosis",
			change: creation,
			responses: []response{
				okRepository(1, false, false), okVariables(), {status: http.StatusBadGateway},
				okRepository(1, false, false), okVariables(declared),
			},
			wantRequests: []string{getRepository, getVariables, postVariable, getRepository, getVariables},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateUnknown,
			wantDesired:  desiredStateMatches,
			wantResult:   failed,
		},
		{
			name:   "value change is sent once and completes",
			change: variableValueChange(t, "GO_VERSION", "1.26", "1.27"),
			responses: []response{
				okRepository(1, false, false), okVariables(githubtest.VariableBody{Name: "GO_VERSION", Value: "1.26"}), noContent(),
				okRepository(1, false, false), okVariables(declared),
			},
			wantRequests: []string{getRepository, getVariables, patchVariable, getRepository, getVariables},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			// 宣言していない OTHER は一覧にあっても宛先にならない
			name:   "deletion of the planned value is sent once only to the declared name and completes when the list lacks it",
			change: deletion,
			responses: []response{
				okRepository(1, false, false), okVariables(legacy, other), noContent(),
				okRepository(1, false, false), okVariables(other),
			},
			wantRequests: []string{getRepository, getVariables, deleteVariable, getRepository, getVariables},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
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
			if got.operation(0).precondition != tc.wantPrecond {
				t.Errorf("precondition = %v, want %v", got.operation(0).precondition, tc.wantPrecond)
			}
			if tc.wantPrecond == preconditionHolds && got.operation(0).update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.operation(0).update, tc.wantUpdate)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
		})
	}
}

// TestExecuteChange_EnvironmentVariables は、Environment Variable の作成と削除を、Environment の一覧と変数の一覧の読み直しで確かめて一度だけ送るつなぎ目と、
// 作成する Environment の中で存在しないことを宣言した変数を、変数への要求を送らずに作成のあとの一覧で確かめるつなぎ目を確かめる。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-011 FR-012 FR-013
func TestExecuteChange_EnvironmentVariables(t *testing.T) {
	const (
		getRepository   = "GET /repos/owner/repo"
		getEnvironments = "GET /repos/owner/repo/environments"
		getVariables    = "GET /repos/owner/repo/environments/production/variables"
		postVariable    = "POST /repos/owner/repo/environments/production/variables"
		deleteVariable  = "DELETE /repos/owner/repo/environments/production/variables/REGION"
		putProduction   = "PUT /repos/owner/repo/environments/production"
	)
	production := githubtest.EnvironmentBody{ID: 8, Name: "production"}
	staging := githubtest.EnvironmentBody{ID: 7, Name: "staging"}
	creation := createdEnvironmentVariableChange(t, "production", "REGION", "ap-northeast-1")
	other := githubtest.VariableBody{Name: "OTHER", Value: "a"}
	approved := withVariables(observedEnvironment(8, "production", setting.DeploymentBranchPolicyAll), github.Variable{Name: "REGION", Value: "us"})
	cases := []struct {
		name         string
		change       changeplan.LogicalChange
		responses    []response
		wantRequests []string
		wantPrecond  preconditionCheck
		wantChanged  setting.Unit
		wantUpdate   github.UpdateOutcome
		wantDesired  desiredStateConfirmation
		wantResult   changeResult
	}{
		{
			name:   "creation in an existing environment without the variable is sent once and completes",
			change: creation,
			responses: []response{
				okRepository(1, false, false), okEnvironments(production), okVariables(), createdVariable(),
				okRepository(1, false, false), okEnvironments(production), okVariables(githubtest.VariableBody{Name: "REGION", Value: "ap-northeast-1"}),
			},
			wantRequests: []string{getRepository, getEnvironments, getVariables, postVariable, getRepository, getEnvironments, getVariables},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			name:         "creation in an environment absent before the update sends no update request",
			change:       creation,
			responses:    []response{okRepository(1, false, false), okEnvironments()},
			wantRequests: []string{getRepository, getEnvironments},
			wantPrecond:  preconditionCurrentValueChanged,
			wantChanged:  creation.Operations[0].Unit,
			wantResult:   notAttempted,
		},
		{
			name:   "deletion in an existing environment is sent once only to the declared name and completes when the list lacks it",
			change: environmentVariableDeletionChange(t, "production", approved, "REGION", "us"),
			responses: []response{
				okRepository(1, false, false), okEnvironments(production), okVariables(githubtest.VariableBody{Name: "REGION", Value: "us"}, other), noContent(),
				okRepository(1, false, false), okEnvironments(production), okVariables(other),
			},
			wantRequests: []string{getRepository, getEnvironments, getVariables, deleteVariable, getRepository, getEnvironments, getVariables},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
		{
			// 作られたばかりの Environment に消す変数はないため、変数への要求を送らず、作成のあとの一覧で存在しないことを確かめる。
			// 一覧にある宣言していない staging の変数は、更新の前にも後にも読まない
			name:   "creation with a declared absence sends only the creation and completes when the list of the created environment lacks the variable",
			change: environmentCreationWithAbsentVariableChange(t, "production", "REGION"),
			responses: []response{
				okRepository(1, false, false), okEnvironments(staging), okEnvironment(production),
				okRepository(1, false, false), okEnvironments(staging, production), okVariables(),
			},
			wantRequests: []string{getRepository, getEnvironments, putProduction, getRepository, getEnvironments, getVariables},
			wantPrecond:  preconditionHolds,
			wantUpdate:   github.UpdateSucceeded,
			wantDesired:  desiredStateMatches,
			wantResult:   completed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, tc.responses...)

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), tc.change, (&delayRecorder{}).after, &updateInterval{})

			// Repository Variables の一覧と更新エンドポイントを宛先にしないことも、要求の並びの一致で確かめる
			if !slices.Equal(server.RequestLines(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), tc.wantRequests)
			}
			if o := got.operation(0); o.precondition != tc.wantPrecond || o.changedSetting != tc.wantChanged {
				t.Errorf("precondition = %v, %+v, want %v, %+v", o.precondition, o.changedSetting, tc.wantPrecond, tc.wantChanged)
			}
			if tc.wantPrecond == preconditionHolds && got.operation(0).update != tc.wantUpdate {
				t.Errorf("update = %v, want %v", got.operation(0).update, tc.wantUpdate)
			}
			if got.confirmation != tc.wantDesired {
				t.Errorf("confirmation = %v, want %v", got.confirmation, tc.wantDesired)
			}
			if got.result() != tc.wantResult {
				t.Errorf("result() = %v, want %v", got.result(), tc.wantResult)
			}
		})
	}
}

// TestExecuteChange_DeploymentPatterns は、個別パターンを一つずつ取り除き、加える論理的な設定変更について、
// 更新操作ごとの間隔と更新直前の観測、宛先の ID、止まり方、更新後の確認のつなぎ目を確かめる。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-011 FR-012 FR-013
func TestExecuteChange_DeploymentPatterns(t *testing.T) {
	const (
		getRepository   = "GET /repos/owner/repo"
		getEnvironments = "GET /repos/owner/repo/environments"
		getPatterns     = "GET /repos/owner/repo/environments/production/deployment-branch-policies"
		putProduction   = "PUT /repos/owner/repo/environments/production"
		postPattern     = "POST /repos/owner/repo/environments/production/deployment-branch-policies"
	)
	deletePattern := func(id string) string {
		return "DELETE /repos/owner/repo/environments/production/deployment-branch-policies/" + id
	}
	readSelected := []string{getRepository, getEnvironments, getPatterns}
	readNotSelected := []string{getRepository, getEnvironments}
	waitTimer, changedWaitTimer := 30, 10
	selectedOf := func(id int64, wait *int) githubtest.EnvironmentBody {
		return githubtest.EnvironmentBody{ID: id, Name: "production", DeploymentBranchPolicy: selectedBody(), WaitTimer: wait}
	}
	// observeSelected は、ID id で方式 selected の production を、個別パターン patterns とともに読む観測の応答。
	observeSelected := func(id int64, wait *int, patterns ...githubtest.DeploymentPatternBody) []response {
		return []response{okRepository(1, false, false), okEnvironments(selectedOf(id, wait)), okDeploymentPatterns(patterns...)}
	}
	pattern := func(id int64, p setting.DeploymentPattern) githubtest.DeploymentPatternBody {
		return githubtest.DeploymentPatternBody{ID: id, Name: p.Name, Type: string(p.Type)}
	}
	main, feature, release, version := branchPattern("main"), branchPattern("feature/*"), branchPattern("release/*"), tagPattern("v*")
	existence := setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "production")}
	policy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: mustEnvironmentName(t, "production")}
	approvedAll := observedEnvironment(7, "production", setting.DeploymentBranchPolicyAll)
	// 計画時の ID は main が 10、feature/* が 11
	approvedMainFeature := selectedEnvironment(7, "production", []setting.DeploymentPattern{main, feature}, 10, 11)
	succeeded := operationRecord{precondition: preconditionHolds, update: github.UpdateSucceeded}

	cases := []struct {
		name           string
		change         changeplan.LogicalChange
		responses      [][]response
		wantRequests   [][]string
		wantDelays     []time.Duration
		wantOperations []operationRecord
		wantDesired    desiredStateConfirmation
		wantOutside    []unexpectedOutsideChange
		wantResult     changeResult
	}{
		{
			name:   "all to selected with two patterns sends the mode and each pattern after a read and the interval and completes",
			change: deploymentPatternsChange(t, "production", approvedAll, selectedPolicy(main, version)),
			responses: [][]response{
				{okRepository(1, false, false), okEnvironments(githubtest.EnvironmentBody{ID: 7, Name: "production"})},
				{okEnvironment(selectedOf(7, nil))},
				observeSelected(7, nil), {okDeploymentPattern(pattern(10, main))},
				observeSelected(7, nil, pattern(10, main)), {okDeploymentPattern(pattern(11, version))},
				observeSelected(7, nil, pattern(10, main), pattern(11, version)),
			},
			wantRequests:   [][]string{readNotSelected, {putProduction}, readSelected, {postPattern}, readSelected, {postPattern}, readSelected},
			wantDelays:     []time.Duration{time.Second, time.Second},
			wantOperations: []operationRecord{succeeded, succeeded, succeeded},
			wantDesired:    desiredStateMatches,
			wantResult:     completed,
		},
		{
			name:   "replacement removes the pattern by the ID in the read before the update rather than the ID at planning and completes",
			change: deploymentPatternsChange(t, "production", approvedMainFeature, selectedPolicy(main, release)),
			responses: [][]response{
				observeSelected(7, nil, pattern(10, main), pattern(20, feature)), {noContent()},
				observeSelected(7, nil, pattern(10, main)), {okDeploymentPattern(pattern(30, release))},
				observeSelected(7, nil, pattern(10, main), pattern(30, release)),
			},
			wantRequests:   [][]string{readSelected, {deletePattern("20")}, readSelected, {postPattern}, readSelected},
			wantDelays:     []time.Duration{time.Second},
			wantOperations: []operationRecord{succeeded, succeeded},
			wantDesired:    desiredStateMatches,
			wantResult:     completed,
		},
		{
			name:   "undeclared selected environment in the list gets no patterns request before or after the update",
			change: deploymentPatternsChange(t, "production", approvedMainFeature, selectedPolicy(main)),
			responses: [][]response{
				{okRepository(1, false, false), okEnvironments(githubtest.EnvironmentBody{ID: 9, Name: "undeclared", DeploymentBranchPolicy: selectedBody()}, selectedOf(7, nil)), okDeploymentPatterns(pattern(10, main), pattern(20, feature))},
				{noContent()},
				{okRepository(1, false, false), okEnvironments(githubtest.EnvironmentBody{ID: 9, Name: "undeclared", DeploymentBranchPolicy: selectedBody()}, selectedOf(7, nil)), okDeploymentPatterns(pattern(10, main))},
			},
			wantRequests:   [][]string{readSelected, {deletePattern("20")}, readSelected},
			wantOperations: []operationRecord{succeeded},
			wantDesired:    desiredStateMatches,
			wantResult:     completed,
		},
		{
			name: "first removal rejected with 404 sends neither the rest nor a read after the update",
			change: deploymentPatternsChange(t, "production", selectedEnvironment(7, "production", []setting.DeploymentPattern{feature, main, release}, 11, 10, 12),
				selectedPolicy(main, version)),
			responses:      [][]response{observeSelected(7, nil, pattern(11, feature), pattern(10, main), pattern(12, release)), {{status: http.StatusNotFound}}},
			wantRequests:   [][]string{readSelected, {deletePattern("11")}},
			wantOperations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateRejected}},
			wantResult:     failed,
		},
		{
			name:   "addition answered with 303 after a removal is not sent again, starts no later addition, and is read once for diagnosis",
			change: deploymentPatternsChange(t, "production", approvedMainFeature, selectedPolicy(main, release, version)),
			responses: [][]response{
				observeSelected(7, nil, pattern(10, main), pattern(11, feature)), {noContent()},
				observeSelected(7, nil, pattern(10, main)),
				{{status: http.StatusSeeOther, location: "/repos/owner/repo/environments/production/deployment-branch-policies"}},
				observeSelected(7, nil, pattern(10, main), pattern(12, release)),
			},
			wantRequests:   [][]string{readSelected, {deletePattern("11")}, readSelected, {postPattern}, readSelected},
			wantDelays:     []time.Duration{time.Second},
			wantOperations: []operationRecord{succeeded, {precondition: preconditionHolds, update: github.UpdateUnknown}},
			wantDesired:    desiredStateMismatch,
			wantResult:     failed,
		},
		{
			name:   "pattern added by another party after a removal stops the addition without reading again",
			change: deploymentPatternsChange(t, "production", approvedMainFeature, selectedPolicy(main, release)),
			responses: [][]response{
				observeSelected(7, nil, pattern(10, main), pattern(11, feature)), {noContent()},
				observeSelected(7, nil, pattern(10, main), pattern(12, branchPattern("other/*"))),
				observeSelected(7, nil, pattern(10, main), pattern(12, branchPattern("other/*"))),
			},
			wantRequests:   [][]string{readSelected, {deletePattern("11")}, readSelected, readSelected},
			wantDelays:     []time.Duration{time.Second},
			wantOperations: []operationRecord{succeeded, {precondition: preconditionCurrentValueChanged, changedSetting: policy}},
			wantDesired:    desiredStateMismatch,
			wantResult:     failed,
		},
		{
			name:   "removal not yet reflected before the addition is read again and completes",
			change: deploymentPatternsChange(t, "production", approvedMainFeature, selectedPolicy(main, release)),
			responses: [][]response{
				observeSelected(7, nil, pattern(10, main), pattern(11, feature)), {noContent()},
				observeSelected(7, nil, pattern(10, main), pattern(11, feature)),
				observeSelected(7, nil, pattern(10, main)), {okDeploymentPattern(pattern(12, release))},
				observeSelected(7, nil, pattern(10, main), pattern(12, release)),
			},
			wantRequests:   [][]string{readSelected, {deletePattern("11")}, readSelected, readSelected, {postPattern}, readSelected},
			wantDelays:     []time.Duration{time.Second, time.Second},
			wantOperations: []operationRecord{succeeded, succeeded},
			wantDesired:    desiredStateMatches,
			wantResult:     completed,
		},
		{
			name:           "pattern added since approval before the first update sends no update request",
			change:         deploymentPatternsChange(t, "production", approvedMainFeature, selectedPolicy(main, release)),
			responses:      [][]response{observeSelected(7, nil, pattern(10, main), pattern(11, feature), pattern(12, branchPattern("other/*")))},
			wantRequests:   [][]string{readSelected},
			wantOperations: []operationRecord{{precondition: preconditionCurrentValueChanged, changedSetting: policy}},
			wantResult:     notAttempted,
		},
		{
			name:   "created environment recreated before the second addition stops it as a change of the environment",
			change: creationWithPatternsChange(t, "production", selectedPolicy(main, version)),
			responses: [][]response{
				{okRepository(1, false, false), okEnvironments()}, {okEnvironment(selectedOf(8, nil))},
				observeSelected(8, nil), {okDeploymentPattern(pattern(10, main))},
				observeSelected(9, nil, pattern(20, main)),
				observeSelected(9, nil, pattern(20, main)),
			},
			wantRequests:   [][]string{readNotSelected, {putProduction}, readSelected, {postPattern}, readSelected, readSelected},
			wantDelays:     []time.Duration{time.Second, time.Second},
			wantOperations: []operationRecord{succeeded, succeeded, {precondition: preconditionCurrentValueChanged, changedSetting: existence}},
			wantDesired:    desiredStateMismatch,
			wantResult:     failed,
		},
		{
			// wait_timer を PUT と POST のあいだで変え、POST の直前とは同じ値にして、PUT の直前と比べることを確かめる
			name:   "wait_timer changed after the mode update is one unexpected change compared with the read before it",
			change: deploymentPatternsChange(t, "production", approvedAll, selectedPolicy(main)),
			responses: [][]response{
				{okRepository(1, false, false), okEnvironments(githubtest.EnvironmentBody{ID: 7, Name: "production", WaitTimer: &waitTimer})},
				{okEnvironment(selectedOf(7, &waitTimer))},
				observeSelected(7, &changedWaitTimer), {okDeploymentPattern(pattern(10, main))},
				observeSelected(7, &changedWaitTimer, pattern(10, main)),
			},
			wantRequests:   [][]string{readNotSelected, {putProduction}, readSelected, {postPattern}, readSelected},
			wantDelays:     []time.Duration{time.Second},
			wantOperations: []operationRecord{succeeded, succeeded},
			wantDesired:    desiredStateMatches,
			wantOutside:    []unexpectedOutsideChange{{Setting: "environments.production.wait_timer", Before: "30", After: "10"}},
			wantResult:     failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, server := newSequenceServer(t, slices.Concat(tc.responses...)...)
			delays := &delayRecorder{}

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), tc.change, delays.after, &updateInterval{})

			if want := slices.Concat(tc.wantRequests...); !slices.Equal(server.RequestLines(), want) {
				t.Errorf("requests = %v, want %v", server.RequestLines(), want)
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
		})
	}
}
