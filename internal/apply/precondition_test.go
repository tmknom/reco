package apply

import (
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestMatchesValueAtApproval は、承認時に予定した値と更新直前の設定の観測結果が同じかの規則を確かめる。
//
//trace:checks INV-001
//trace:checks INV-004
//trace:checks FR-010
func TestMatchesValueAtApproval(t *testing.T) {
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
			if got := matchesValueAtApproval(tc.atApproval, tc.current); got != tc.want {
				t.Errorf("matchesValueAtApproval() = %v, want %v", got, tc.want)
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

	// 関係する現在値の設定が複数並ぶ変更と、承認時に予定した値が観測不能の変更。値の比べ方は TestMatchesValueAtApproval が確かめる
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
