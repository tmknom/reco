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
	operation := allowAutoMergeChange().Operations[0]
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
			if got, changed := checkPrecondition(1, operation, tc.observed); got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %q, want %v, %q", got, changed, tc.want, tc.wantChanged)
			}
		})
	}

	// 関係する現在値の設定が複数並ぶ変更と、承認時に予定した値が観測不能の変更。値の比べ方は TestSameObservation が確かめる
	actionsCases := []struct {
		name   string
		change changeplan.LogicalChange
		// index は、確かめる更新操作の change.Operations の添字。
		index       int
		observed    *github.ActionsPermissions
		want        preconditionCheck
		wantChanged setting.Key
	}{
		{
			// 2つ目がすべて上書きする3項目は基準に並ばないため、GitHub が戻した値でも成立する
			name:     "second update to selected holds with the policy selected and the criteria GitHub set back",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}}),
			want:     preconditionHolds,
		},
		{
			name:        "second update to selected with the policy not selected is a current value change of allowed_actions",
			change:      toSelectedChange(),
			index:       1,
			observed:    actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:        preconditionCurrentValueChanged,
			wantChanged: setting.AllowedActions,
		},
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
			if got, changed := checkPrecondition(1, tc.change.Operations[tc.index], observed); got != tc.want || changed != tc.wantChanged {
				t.Errorf("checkPrecondition() = %v, %q, want %v, %q", got, changed, tc.want, tc.wantChanged)
			}
		})
	}

	t.Run("update operation without relevant settings panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("checkPrecondition() did not panic")
			}
		}()
		empty := changeplan.UpdateOperation{Key: setting.AllowAutoMerge, Desired: setting.BoolValue(true)}
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
			if got, reason := confirmDesiredState(1, change, tc.observed); got != tc.want || reason != 0 {
				t.Errorf("confirmDesiredState() = %v, %v, want %v without a reason", got, reason, tc.want)
			}
		})
	}

	selectedCases := []struct {
		name       string
		change     changeplan.LogicalChange
		observed   *github.ActionsPermissions
		want       desiredStateConfirmation
		wantReason github.UnavailableReason
	}{
		{
			name:     "criteria differing from the desired value only in the order of patterns match",
			change:   selectedActionsChange(),
			observed: actionsSelected(setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{"b/*", "a/*"}}),
			want:     desiredStateMatches,
		},
		{
			name:       "unavailable criteria are unavailable rather than a mismatch",
			change:     selectedActionsChange(),
			observed:   actionsDisabled(),
			want:       desiredStateUnavailable,
			wantReason: github.UnavailableActionsDisabled,
		},
		{
			name:     "policy and criteria both desired, with criteria differing only in the order of patterns, match",
			change:   toSelectedChange(),
			observed: actionsSelected(setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{"b/*", "a/*"}}),
			want:     desiredStateMatches,
		},
		{
			name:     "selected policy with the criteria GitHub set back is a mismatch of the second desired value",
			change:   toSelectedChange(),
			observed: actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}}),
			want:     desiredStateMismatch,
		},
		{
			// ドメインモデルの期待状態の確認結果。観測できた値に期待値と異なるものがあれば、観測不能が残っても不一致になる
			name:     "policy other than selected is a mismatch even though the criteria are unavailable",
			change:   toSelectedChange(),
			observed: actionsEnabled(setting.AllowedActionsAll, false),
			want:     desiredStateMismatch,
		},
		{
			// 計画が作らない並び。観測不能の設定が先に並んでも、観測できた値の不一致が優先されることを確かめる
			name: "mismatch after an unavailable desired value in the order of updates is still a mismatch",
			change: changeplan.LogicalChange{Operations: []changeplan.UpdateOperation{
				toSelectedChange().Operations[1], toSelectedChange().Operations[0],
			}},
			observed: actionsEnabled(setting.AllowedActionsAll, false),
			want:     desiredStateMismatch,
		},
		{
			name:       "actions disabled is unavailable for both desired values with the reason",
			change:     toSelectedChange(),
			observed:   actionsDisabled(),
			want:       desiredStateUnavailable,
			wantReason: github.UnavailableActionsDisabled,
		},
	}
	for _, tc := range selectedCases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, ActionsPermissions: tc.observed}
			if got, reason := confirmDesiredState(1, tc.change, observed); got != tc.want || reason != tc.wantReason {
				t.Errorf("confirmDesiredState() = %v, %v, want %v, %v", got, reason, tc.want, tc.wantReason)
			}
		})
	}
}

// TestPrecedingUpdateNotReflected は、後続の更新操作の直前の観測が、先行する更新操作の直前の状態のままかの規則を確かめる。
// 真のときだけ反映の遅れの可能性として観測し直すため、偽の側に倒れる形を確かめる。
//
//trace:checks INV-004
//trace:checks INV-005
func TestPrecedingUpdateNotReflected(t *testing.T) {
	cases := []struct {
		name     string
		change   changeplan.LogicalChange
		index    int
		observed *github.ActionsPermissions
		want     bool
	}{
		{
			name:     "first update has no preceding update even if the observation is the value before it",
			change:   toSelectedChange(),
			index:    0,
			observed: actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:     false,
		},
		{
			name:     "policy still local_only as before the first update is not reflected",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsEnabled(setting.AllowedActionsLocalOnly, false),
			want:     true,
		},
		{
			name:     "actions still disabled as before the first update is not reflected",
			change:   enableSelectedChange(),
			index:    1,
			observed: actionsDisabled(),
			want:     true,
		},
		{
			name:     "policy selected is reflected",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsSelected(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{}}),
			want:     false,
		},
		{
			name:     "policy neither the value before the first update nor selected is not a delay",
			change:   toSelectedChange(),
			index:    1,
			observed: actionsEnabled(setting.AllowedActionsAll, false),
			want:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observed := github.ObservationResult{RepositoryID: 1, ActionsPermissions: tc.observed}
			if got := precedingUpdateNotReflected(1, tc.change.Operations[:tc.index], observed); got != tc.want {
				t.Errorf("precedingUpdateNotReflected() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestObserveBeforeUpdate は、2つ目の更新操作の直前の観測で、1つ目の反映の遅れを待って観測し直す規則を確かめる。
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
	selected := []response{okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(true, false, `[]`)}
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
		name         string
		change       changeplan.LogicalChange
		responses    []response
		wantRequests []string
		wantDelays   []time.Duration
		want         operationRecord
		wantErr      bool
	}{
		{
			name:         "policy already selected holds without waiting",
			change:       toSelectedChange(),
			responses:    selected,
			wantRequests: readSelected,
			want:         operationRecord{precondition: preconditionHolds},
		},
		{
			name:         "policy still before the first update and then selected holds after waiting once",
			change:       toSelectedChange(),
			responses:    append(slices.Clone(localOnly), selected...),
			wantRequests: append(slices.Clone(readNotSelected), readSelected...),
			wantDelays:   allDelays[:1],
			want:         operationRecord{precondition: preconditionHolds},
		},
		{
			name:         "policy still before the first update on all six reads is a current value change of allowed_actions",
			change:       toSelectedChange(),
			responses:    repeat(6, localOnly),
			wantRequests: repeatRequests(6, readNotSelected),
			wantDelays:   allDelays,
			want:         operationRecord{precondition: preconditionCurrentValueChanged, changedSetting: setting.AllowedActions},
		},
		{
			name:         "policy neither before the first update nor selected is a current value change without waiting",
			change:       toSelectedChange(),
			responses:    []response{okRepository(1, false, false), okAllowedActions("all", false)},
			wantRequests: readNotSelected,
			want:         operationRecord{precondition: preconditionCurrentValueChanged, changedSetting: setting.AllowedActions},
		},
		{
			name:         "failed read is unconfirmed without waiting",
			change:       toSelectedChange(),
			responses:    []response{okRepository(1, false, false), {status: http.StatusInternalServerError}},
			wantRequests: readNotSelected,
			want:         operationRecord{precondition: preconditionUnconfirmed},
			wantErr:      true,
		},
		{
			name:         "different ID is an identity mismatch without waiting even if the policy is before the first update",
			change:       toSelectedChange(),
			responses:    []response{okRepository(2, false, false), okAllowedActions("local_only", false)},
			wantRequests: readNotSelected,
			want:         operationRecord{precondition: preconditionIdentityMismatch},
		},
		{
			name:         "actions still disabled as before the first update and then selected holds after waiting once",
			change:       enableSelectedChange(),
			responses:    append([]response{okRepository(1, false, false), okActionsDisabled()}, selected...),
			wantRequests: append(slices.Clone(readNotSelected), readSelected...),
			wantDelays:   allDelays[:1],
			want:         operationRecord{precondition: preconditionHolds},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, rec := newSequenceServer(t, tc.responses...)
			delays := &delayRecorder{}

			_, got, err := observeBeforeUpdate(context.Background(), newClient(baseURL), newRepositoryPlan(t), tc.change, 1, observationAreas(tc.change), delays.after)

			if !slices.Equal(rec.Requests(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", rec.Requests(), tc.wantRequests)
			}
			if !slices.Equal(delays.delays, tc.wantDelays) {
				t.Errorf("delays = %v, want %v", delays.delays, tc.wantDelays)
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
		baseURL, rec := newSequenceServer(t, append(slices.Clone(localOnly), selected...)...)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		interrupted := func(time.Duration) <-chan time.Time {
			cancel()
			return make(chan time.Time)
		}
		change := toSelectedChange()

		_, got, err := observeBeforeUpdate(ctx, newClient(baseURL), newRepositoryPlan(t), change, 1, observationAreas(change), interrupted)

		if !slices.Equal(rec.Requests(), readNotSelected) {
			t.Errorf("requests = %v, want %v", rec.Requests(), readNotSelected)
		}
		if got != (operationRecord{precondition: preconditionUnconfirmed}) {
			t.Errorf("observeBeforeUpdate() = %+v, want unconfirmed", got)
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("observeBeforeUpdate() error = %v, want it to wrap %v", err, context.Canceled)
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
			if got := outsideChangesAt(tc.endpoint, tc.change.Keys(), tc.before, tc.after); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("outsideChangesAt() = %v, want %v", got, tc.want)
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
			p := newRepositoryPlan(t)
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
		baseURL, rec := newSequenceServer(t, okRepository(1, false, false), okRepository(1, true, false), okRepository(1, false, false), okRepository(1, true, false))
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		interrupted := func(time.Duration) <-chan time.Time {
			cancel()
			return make(chan time.Time)
		}

		got := executeChange(ctx, newClient(baseURL), newRepositoryPlan(t), allowAutoMergeChange(), interrupted)

		if want := []string{"GET", "PATCH", "GET"}; !slices.Equal(rec.Methods(), want) {
			t.Errorf("requests = %v, want %v", rec.Methods(), want)
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

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), tc.change, delays.after)

			if !slices.Equal(rec.Requests(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", rec.Requests(), tc.wantRequests)
			}
			if tc.wantBodies != nil && !reflect.DeepEqual(rec.Bodies(), tc.wantBodies) {
				t.Errorf("request bodies = %v, want %v", rec.Bodies(), tc.wantBodies)
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
	setBack := okSelectedActions(true, false, `[]`)
	beforeSecond := []response{okRepository(1, false, false), okAllowedActions("selected", false), setBack}
	// 宣言した値と許可パターンの順序だけが異なる
	afterBoth := []response{okRepository(1, false, false), okAllowedActions("selected", false), okSelectedActions(false, true, `["b/*", "a/*"]`)}
	readNotSelected := []string{getRepository, getActionsPermissions}
	readSelected := []string{getRepository, getActionsPermissions, getSelectedActions}
	join := func(parts ...[]response) []response { return slices.Concat(parts...) }
	requests := func(parts ...[]string) []string { return slices.Concat(parts...) }
	firstBody := map[string]any{"enabled": true, "allowed_actions": "selected"}
	secondBody := map[string]any{"github_owned_allowed": false, "verified_allowed": true, "patterns_allowed": []any{"a/*", "b/*"}}
	holds := func(update github.UpdateOutcome) operationRecord {
		return operationRecord{precondition: preconditionHolds, update: update}
	}
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
			name:   "policy other than selected before the second update fails without the second update and reads once after it",
			change: toSelectedChange(),
			responses: join(beforeFirst, []response{noContent, okRepository(1, false, false), okAllowedActions("all", false)},
				[]response{okRepository(1, false, false), okAllowedActions("all", false)}),
			wantRequests:   requests(readNotSelected, []string{putActionsPermissions}, readNotSelected, readNotSelected),
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), {precondition: preconditionCurrentValueChanged, changedSetting: setting.AllowedActions}},
			wantDesired:    desiredStateMismatch,
			wantResult:     failed,
		},
		{
			name:   "failed read before the second update fails without the second update and reads once after it",
			change: toSelectedChange(),
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
			name:   "rejected second update reads once after it and reports sha_pinning_required changed from before the first",
			change: toSelectedChange(),
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
			change:         toSelectedChange(),
			responses:      join(beforeFirst, []response{noContent}, beforeSecond, []response{redirect}, afterBoth),
			wantRequests:   requests(readNotSelected, []string{putActionsPermissions}, readSelected, []string{putSelectedActions}, readSelected),
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), holds(github.UpdateUnknown)},
			wantDesired:    desiredStateMatches,
			wantResult:     failed,
			wantCause:      github.ErrRedirect,
		},
		{
			name:   "sha_pinning_required changed from the read before the first update fails even if unchanged since the second",
			change: toSelectedChange(),
			responses: join(beforeFirst, []response{noContent},
				[]response{okRepository(1, false, false), okAllowedActions("selected", true), setBack}, []response{noContent},
				[]response{okRepository(1, false, false), okAllowedActions("selected", true), okSelectedActions(false, true, `["a/*", "b/*"]`)}),
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
			wantDelays:     []time.Duration{time.Second},
			wantOperations: []operationRecord{holds(github.UpdateSucceeded), holds(github.UpdateSucceeded)},
			wantDesired:    desiredStateMatches,
			wantResult:     completed,
		},
		{
			// 1つ目の直前は sha_pinning_required が観測不能で比べる観測値がなく、更新後に true でも予期しない変化にしない
			name:   "disabled to selected enables actions with the policy in one update and completes",
			change: enableSelectedChange(),
			responses: join([]response{okRepository(1, false, false), okActionsDisabled(), noContent}, beforeSecond, []response{noContent},
				[]response{okRepository(1, false, false), okAllowedActions("selected", true), okSelectedActions(false, true, `["a/*", "b/*"]`)}),
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
			baseURL, rec := newSequenceServer(t, tc.responses...)
			delays := &delayRecorder{}

			got := executeChange(context.Background(), newClient(baseURL), newRepositoryPlan(t), tc.change, delays.after)

			if !slices.Equal(rec.Requests(), tc.wantRequests) {
				t.Errorf("requests = %v, want %v", rec.Requests(), tc.wantRequests)
			}
			if tc.wantBodies != nil && !reflect.DeepEqual(rec.Bodies(), tc.wantBodies) {
				t.Errorf("request bodies = %v, want %v", rec.Bodies(), tc.wantBodies)
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
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			baseURL, rec := newSequenceServer(t, tc.responses...)
			changes := tc.changes

			got := executeChanges(context.Background(), newClient(baseURL), newRepositoryPlan(t), changes, elapsedAtOnce)

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
				if record.operation(0).precondition != tc.wantPrecond[i] {
					t.Errorf("records[%d].precondition = %v, want %v", i, record.operation(0).precondition, tc.wantPrecond[i])
				}
				if record.result() != tc.wantResults[i] {
					t.Errorf("records[%d].result() = %v, want %v", i, record.result(), tc.wantResults[i])
				}
			}
		})
	}
}
