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
		return newSettingPlan(setting.Unit{Key: setting.AllowedActions}, observed.Setting(setting.Unit{Key: setting.AllowedActions}), policyValue(desired))
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
			plan:     newSettingPlan(setting.Unit{Key: setting.ActionsEnabled}, selected.Setting(setting.Unit{Key: setting.ActionsEnabled}), setting.BoolValue(false)),
			observed: selected,
			want:     nil,
		},
		{
			name:     "criteria change has no side effect",
			plan:     newSettingPlan(setting.Unit{Key: setting.SelectedActions}, selected.Setting(setting.Unit{Key: setting.SelectedActions}), setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{})),
			observed: selected,
			want:     nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, unaffected := sideEffects(tc.plan, tc.observed, []SettingPlan{tc.plan})
			if !reflect.DeepEqual(got, tc.want) || unaffected != nil {
				t.Errorf("sideEffects() = %+v, %+v, want %+v, nil", got, unaffected, tc.want)
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
		wantUnaffected  []SettingExpectedUnaffected
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
			name:           "disabling alerts keeps the undeclared disabled security updates as a setting expected unaffected",
			observed:       observation(true, setting.DependabotSecurityUpdatesDisabled),
			want:           []SideEffect{graphDisabled},
			wantUnaffected: []SettingExpectedUnaffected{{Key: setting.DependabotSecurityUpdates, Observed: updatesValue(setting.DependabotSecurityUpdatesDisabled)}},
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
			plan := newSettingPlan(setting.Unit{Key: setting.DependabotAlerts}, tc.observed.Setting(setting.Unit{Key: setting.DependabotAlerts}), setting.BoolValue(tc.desired))
			settings := []SettingPlan{plan}
			if tc.declaresUpdates {
				settings = append(settings, newSettingPlan(setting.Unit{Key: setting.DependabotSecurityUpdates}, tc.observed.Setting(setting.Unit{Key: setting.DependabotSecurityUpdates}), setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesDisabled)))
			}
			got, unaffected := sideEffects(plan, tc.observed, settings)
			if !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(unaffected, tc.wantUnaffected) {
				t.Errorf("sideEffects() = %+v, %+v, want %+v, %+v", got, unaffected, tc.want, tc.wantUnaffected)
			}
		})
	}
}

//trace:checks FR-007 FR-010 INV-001
func TestSideEffects_SecretScanning(t *testing.T) {
	public := func(enabled, pushProtection bool) github.ObservationResult {
		return github.ObservationResult{SecretScanning: &github.SecretScanning{Enabled: github.Observed(enabled), PushProtection: github.Observed(pushProtection)}}
	}
	private := github.ObservationResult{SecretScanning: &github.SecretScanning{
		Enabled:        github.Unavailable[bool](github.UnavailablePrivateRepository),
		PushProtection: github.Unavailable[bool](github.UnavailablePrivateRepository),
	}}
	cases := []struct {
		name     string
		observed github.ObservationResult
		desired  bool
		// declaresPushProtection は、同じリポジトリで secret_scanning_push_protection を false と宣言するか。
		declaresPushProtection bool
		want                   []SideEffect
		wantUnaffected         []SettingExpectedUnaffected
	}{
		{
			name:     "disabling secret scanning disables the undeclared enabled push protection",
			observed: public(true, true),
			want:     []SideEffect{{Kind: SideEffectPushProtectionDisabled, Observed: github.Observed(setting.BoolValue(true))}},
		},
		{
			name:           "disabling secret scanning keeps the undeclared disabled push protection as a setting expected unaffected",
			observed:       public(true, false),
			wantUnaffected: []SettingExpectedUnaffected{{Key: setting.RepositoryPushProtection, Observed: github.Observed(setting.BoolValue(false))}},
		},
		{
			name:                   "disabling secret scanning leaves declared push protection to the plan",
			observed:               public(true, true),
			declaresPushProtection: true,
		},
		{
			name:     "enabling secret scanning has neither",
			observed: public(false, false),
			desired:  true,
		},
		{
			name:     "not comparable secret scanning on a private repository has neither because it is not applicable",
			observed: private,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := newSettingPlan(setting.Unit{Key: setting.SecretScanningEnabled}, tc.observed.Setting(setting.Unit{Key: setting.SecretScanningEnabled}), setting.BoolValue(tc.desired))
			settings := []SettingPlan{plan}
			if tc.declaresPushProtection {
				settings = append(settings, newSettingPlan(setting.Unit{Key: setting.RepositoryPushProtection}, tc.observed.Setting(setting.Unit{Key: setting.RepositoryPushProtection}), setting.BoolValue(false)))
			}
			got, unaffected := sideEffects(plan, tc.observed, settings)
			if !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(unaffected, tc.wantUnaffected) {
				t.Errorf("sideEffects() = %+v, %+v, want %+v, %+v", got, unaffected, tc.want, tc.wantUnaffected)
			}
		})
	}
}

//trace:checks INV-003
func TestSideEffects_RequiredReviewers(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	unit := setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: production}
	octocat := setting.Reviewer{Type: setting.ReviewerUser, ID: 583231}
	protection := func(preventSelfReview bool, reviewers ...setting.Reviewer) *setting.RequiredReviewers {
		return &setting.RequiredReviewers{PreventSelfReview: preventSelfReview, Reviewers: reviewers}
	}
	observation := func(rule *setting.RequiredReviewers) github.ObservationResult {
		return github.ObservationResult{Environments: &github.Environments{Existing: []github.Environment{{Name: "production", Protection: setting.EnvironmentProtection{RequiredReviewers: rule}}}}}
	}
	absent := github.ObservationResult{Environments: &github.Environments{}}
	none := setting.ReviewerSetValue(setting.ReviewerSetOf(nil))
	one := setting.ReviewerSetValue(setting.ReviewerSetOf([]setting.Reviewer{octocat}))
	preventSelfReviewDiscarded := []SideEffect{{Kind: SideEffectPreventSelfReviewDiscarded, Observed: github.Observed(setting.BoolValue(true))}}
	cases := []struct {
		name     string
		observed github.ObservationResult
		desired  setting.Value
		want     []SideEffect
	}{
		{name: "removing every reviewer discards prevent_self_review that is true", observed: observation(protection(true, octocat)), desired: none, want: preventSelfReviewDiscarded},
		{name: "removing every reviewer has no side effect when prevent_self_review is false", observed: observation(protection(false, octocat)), desired: none},
		{name: "keeping a reviewer has no side effect when prevent_self_review is true", observed: observation(protection(true, octocat)), desired: one},
		{name: "emptying a protection rule without reviewers discards prevent_self_review that is true", observed: observation(protection(true)), desired: none, want: preventSelfReviewDiscarded},
		{name: "environment without a protection rule matches no reviewers and has no side effect", observed: observation(nil), desired: none},
		{name: "absent environment has no side effect", observed: absent, desired: none},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := newSettingPlan(unit, tc.observed.Setting(unit), tc.desired)
			got, unaffected := sideEffects(plan, tc.observed, []SettingPlan{plan})
			if !reflect.DeepEqual(got, tc.want) || unaffected != nil {
				t.Errorf("sideEffects() = %+v, %+v, want %+v, nil", got, unaffected, tc.want)
			}
		})
	}
}

func TestSideEffects_DependentWithoutObservationPanics(t *testing.T) {
	observed := github.ObservationResult{SecretScanning: &github.SecretScanning{
		Enabled: github.Observed(true), PushProtection: github.Unavailable[bool](github.UnavailablePrivateRepository),
	}}
	plan := newSettingPlan(setting.Unit{Key: setting.SecretScanningEnabled}, observed.Setting(setting.Unit{Key: setting.SecretScanningEnabled}), setting.BoolValue(false))
	defer func() {
		if recover() == nil {
			t.Error("sideEffects() did not panic for the unobserved dependent setting")
		}
	}()
	sideEffects(plan, observed, []SettingPlan{plan})
}

func TestSideEffectKind_ObservedSetting(t *testing.T) {
	cases := []struct {
		kind   SideEffectKind
		want   setting.Key
		wantOK bool
	}{
		{kind: SideEffectSelectedActionsDiscarded, want: setting.SelectedActions, wantOK: true},
		{kind: SideEffectSecurityUpdatesDisabled, want: setting.DependabotSecurityUpdates, wantOK: true},
		{kind: SideEffectPushProtectionDisabled, want: setting.RepositoryPushProtection, wantOK: true},
		{kind: SideEffectPreventSelfReviewDiscarded, wantOK: false},
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

// TestDependentDisabledSideEffect は、どの設定の依存関係の依存する設定にも、その設定が無効になる副作用の種類があり、依存する設定でない設定を不変条件の破れとすることを確かめる。
// 設定の依存関係を足して副作用の種類を足し忘れたことを、計画を作る前に検出するため、設定の依存関係の一覧を走査する。
func TestDependentDisabledSideEffect(t *testing.T) {
	for _, d := range setting.Dependencies() {
		t.Run(string(d.Dependent), func(t *testing.T) {
			if key, ok := dependentDisabledSideEffect(d.Dependent).ObservedSetting(); !ok || key != d.Dependent {
				t.Errorf("dependentDisabledSideEffect(%s).ObservedSetting() = %s, %t, want %s, true", d.Dependent, key, ok, d.Dependent)
			}
		})
	}

	t.Run("setting that is not a dependent panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("dependentDisabledSideEffect() did not panic")
			}
		}()
		dependentDisabledSideEffect(setting.AllowAutoMerge)
	})
}
