package changeplan

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-007 INV-001
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
		return newSettingPlan(setting.AllowedActions, observed.Setting(setting.Unit{Key: setting.AllowedActions}), policyValue(desired))
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
			want:     []SideEffect{{Kind: SideEffectSelectedActionsDiscarded, Observed: github.Observed(setting.SelectedActionsCriteriaValue(criteria))}},
		},
		{
			name:     "selected to local_only discards the observed criteria",
			plan:     policyPlan(selected, setting.AllowedActionsLocalOnly),
			observed: selected,
			want:     []SideEffect{{Kind: SideEffectSelectedActionsDiscarded, Observed: github.Observed(setting.SelectedActionsCriteriaValue(criteria))}},
		},
		{
			name:     "disabled to all may discard the criteria that cannot be observed",
			plan:     policyPlan(disabled, setting.AllowedActionsAll),
			observed: disabled,
			want:     []SideEffect{{Kind: SideEffectSelectedActionsDiscarded, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)}},
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
			name:     "disabling actions with selected policy has no side effect because GitHub keeps the criteria",
			plan:     newSettingPlan(setting.ActionsEnabled, selected.Setting(setting.Unit{Key: setting.ActionsEnabled}), setting.BoolValue(false)),
			observed: selected,
			want:     nil,
		},
		{
			name:     "criteria change has no side effect",
			plan:     newSettingPlan(setting.SelectedActions, selected.Setting(setting.Unit{Key: setting.SelectedActions}), setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{})),
			observed: selected,
			want:     nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, without := sideEffects(tc.plan, tc.observed, []SettingPlan{tc.plan})
			if !reflect.DeepEqual(got, tc.want) || without != nil {
				t.Errorf("sideEffects() = %+v, %+v, want %+v, nil", got, without, tc.want)
			}
		})
	}
}

//trace:checks FR-007 FR-010 INV-001
func TestSideEffects_DependabotAlerts(t *testing.T) {
	observation := func(alerts bool, updates setting.DependabotSecurityUpdatesState) github.ObservationResult {
		return github.ObservationResult{Dependabot: &github.Dependabot{Alerts: alerts, SecurityUpdates: updates}}
	}
	updatesValue := func(state setting.DependabotSecurityUpdatesState) github.SettingObservation[setting.Value] {
		return github.Observed(setting.DependabotSecurityUpdatesValue(state))
	}
	graphDisabled := SideEffect{Kind: SideEffectDependencyGraphDisabled}
	cases := []struct {
		name     string
		observed github.ObservationResult
		desired  bool
		// declaresUpdates は、同じリポジトリで dependabot_security_updates を false と宣言するか。
		declaresUpdates bool
		want            []SideEffect
		wantWithout     []SettingWithoutSideEffect
	}{
		{
			name:     "enabling alerts enables the dependency graph",
			observed: observation(false, setting.DependabotSecurityUpdatesDisabled),
			desired:  true,
			want:     []SideEffect{{Kind: SideEffectDependencyGraphEnabled}},
		},
		{
			name:     "disabling alerts disables the dependency graph and the undeclared enabled security updates",
			observed: observation(true, setting.DependabotSecurityUpdatesEnabled),
			want:     []SideEffect{graphDisabled, {Kind: SideEffectSecurityUpdatesDisabled, Observed: updatesValue(setting.DependabotSecurityUpdatesEnabled)}},
		},
		{
			name:        "disabling alerts keeps the undeclared disabled security updates as a setting without side effect",
			observed:    observation(true, setting.DependabotSecurityUpdatesDisabled),
			want:        []SideEffect{graphDisabled},
			wantWithout: []SettingWithoutSideEffect{{Key: setting.DependabotSecurityUpdates, Observed: updatesValue(setting.DependabotSecurityUpdatesDisabled)}},
		},
		{
			name:            "disabling alerts leaves declared security updates to the plan",
			observed:        observation(true, setting.DependabotSecurityUpdatesEnabled),
			declaresUpdates: true,
			want:            []SideEffect{graphDisabled},
		},
		{
			name:     "matched enabled alerts have neither",
			observed: observation(true, setting.DependabotSecurityUpdatesEnabled),
			desired:  true,
		},
		{
			name:     "matched disabled alerts have neither because nothing is updated",
			observed: observation(false, setting.DependabotSecurityUpdatesDisabled),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := newSettingPlan(setting.DependabotAlerts, tc.observed.Setting(setting.Unit{Key: setting.DependabotAlerts}), setting.BoolValue(tc.desired))
			settings := []SettingPlan{plan}
			if tc.declaresUpdates {
				settings = append(settings, newSettingPlan(setting.DependabotSecurityUpdates, tc.observed.Setting(setting.Unit{Key: setting.DependabotSecurityUpdates}), setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesDisabled)))
			}
			got, without := sideEffects(plan, tc.observed, settings)
			if !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(without, tc.wantWithout) {
				t.Errorf("sideEffects() = %+v, %+v, want %+v, %+v", got, without, tc.want, tc.wantWithout)
			}
		})
	}
}

func TestSideEffectKind_ObservedSetting(t *testing.T) {
	cases := []struct {
		kind   SideEffectKind
		want   setting.Key
		wantOK bool
	}{
		{kind: SideEffectSelectedActionsDiscarded, want: setting.SelectedActions, wantOK: true},
		{kind: SideEffectSecurityUpdatesDisabled, want: setting.DependabotSecurityUpdates, wantOK: true},
		{kind: SideEffectDependencyGraphEnabled, wantOK: false},
		{kind: SideEffectDependencyGraphDisabled, wantOK: false},
	}
	for _, tc := range cases {
		if got, ok := tc.kind.ObservedSetting(); got != tc.want || ok != tc.wantOK {
			t.Errorf("%d.ObservedSetting() = %q, %t, want %q, %t", tc.kind, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestSideEffectKind_ObservedSettingOfUnknownKindPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("ObservedSetting() did not panic for an unknown kind")
		}
	}()
	SideEffectKind(0).ObservedSetting()
}
