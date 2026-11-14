package changeplan

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-007
//trace:checks INV-001
func TestSideEffects(t *testing.T) {
	criteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: []string{"example/*"}}
	policyValue := setting.AllowedActionsPolicyValue
	notSelected := func(policy setting.AllowedActionsPolicy) github.ObservationResult {
		return github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{
			Enabled: true, AllowedActions: github.Observed(policy), SelectedActions: github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableNotSelected),
		}}
	}
	selected := github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{
		Enabled: true, AllowedActions: github.Observed(setting.AllowedActionsSelected), SelectedActions: github.Observed(criteria),
	}}
	disabled := github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{
		AllowedActions:  github.Unavailable[setting.AllowedActionsPolicy](github.UnavailableActionsDisabled),
		SelectedActions: github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableActionsDisabled),
	}}
	// policyPlan は、observed を計画時に観測して期待値を desired とした Actions 許可ポリシーの計画を作る。
	policyPlan := func(observed github.ObservationResult, desired setting.AllowedActionsPolicy) SettingPlan {
		return newSettingPlan(setting.AllowedActions, observed.Value(setting.AllowedActions), policyValue(desired))
	}
	cases := []struct {
		name     string
		plan     SettingPlan
		observed github.ObservationResult
		want     []SideEffect
	}{
		{
			name:     "selected to all discards the observed criteria",
			plan:     policyPlan(selected, setting.AllowedActionsAll),
			observed: selected,
			want:     []SideEffect{{Key: setting.SelectedActions, Observed: github.Observed(setting.SelectedActionsCriteriaValue(criteria))}},
		},
		{
			name:     "selected to local_only discards the observed criteria",
			plan:     policyPlan(selected, setting.AllowedActionsLocalOnly),
			observed: selected,
			want:     []SideEffect{{Key: setting.SelectedActions, Observed: github.Observed(setting.SelectedActionsCriteriaValue(criteria))}},
		},
		{
			name:     "disabled to all may discard the criteria that cannot be observed",
			plan:     policyPlan(disabled, setting.AllowedActionsAll),
			observed: disabled,
			want:     []SideEffect{{Key: setting.SelectedActions, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)}},
		},
		{
			name:     "all to local_only has no side effect because no criteria exist",
			plan:     policyPlan(notSelected(setting.AllowedActionsAll), setting.AllowedActionsLocalOnly),
			observed: notSelected(setting.AllowedActionsAll),
			want:     nil,
		},
		{
			name:     "disabled to selected has no side effect because the declared criteria replace them",
			plan:     policyPlan(disabled, setting.AllowedActionsSelected),
			observed: disabled,
			want:     nil,
		},
		{
			name:     "criteria change has no side effect",
			plan:     newSettingPlan(setting.SelectedActions, selected.Value(setting.SelectedActions), setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{})),
			observed: selected,
			want:     nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sideEffects(tc.plan, tc.observed); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("sideEffects() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
