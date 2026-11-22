package changeplan

import (
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-006
func TestJudgeApplicability(t *testing.T) {
	notComparable := func(key setting.Key, reason github.UnavailableReason) SettingPlan {
		return SettingPlan{Unit: setting.Unit{Key: key}, Observed: github.Unavailable[setting.Value](reason), Result: NotComparable}
	}
	shaPinningDisabled := notComparable(setting.SHAPinningRequired, github.UnavailableActionsDisabled)
	generalMismatch := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch}
	enablesActions := SettingPlan{Unit: setting.Unit{Key: setting.ActionsEnabled}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch}
	accessLevelPublic := notComparable(setting.ActionsAccessLevel, github.UnavailablePublicRepository)
	production, staging := mustEnvironmentName(t, "production"), mustEnvironmentName(t, "staging")
	creation := func(name setting.EnvironmentName) SettingPlan {
		return SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: name}, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch}
	}
	environmentPolicy := func(name setting.EnvironmentName, reason github.UnavailableReason) SettingPlan {
		return SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: name}, Observed: github.Unavailable[setting.Value](reason),
			Desired: setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}), Result: NotComparable,
		}
	}
	environmentVariable := func(name setting.EnvironmentName, reason github.UnavailableReason) SettingPlan {
		return SettingPlan{
			Unit: environmentVariableUnit(name, mustVariableName(t, "REGION")), Observed: github.Unavailable[setting.Value](reason),
			Desired: setting.VariableValue("eu"), Result: NotComparable,
		}
	}
	undeterminedEnvironment := SettingPlan{
		Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentNameUndetermined), Desired: setting.Present(), Result: NotComparable,
	}
	cases := []struct {
		name string
		in   SettingPlan
		// settings は、in と同じリポジトリの計画に並ぶ管理設定の計画。in を含む。
		settings   []SettingPlan
		want       Applicability
		wantReason NotApplicableReason
	}{
		{
			name: "match is not judged",
			in:   SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match},
			want: ApplicabilityNotJudged,
		},
		{
			name: "mismatch is applicable",
			in:   SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
			want: Applicable,
		},
		{
			name: "allowed_actions not comparable because actions is disabled is applicable",
			in:   notComparable(setting.AllowedActions, github.UnavailableActionsDisabled),
			want: Applicable,
		},
		{
			name: "selected_actions not comparable because the policy is not selected is applicable",
			in:   notComparable(setting.SelectedActions, github.UnavailableNotSelected),
			want: Applicable,
		},
		{
			name: "selected_actions not comparable because actions is disabled is applicable",
			in:   notComparable(setting.SelectedActions, github.UnavailableActionsDisabled),
			want: Applicable,
		},
		{
			name: "sha_pinning_required mismatch is applicable without a declaration enabling actions",
			in:   SettingPlan{Unit: setting.Unit{Key: setting.SHAPinningRequired}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
			want: Applicable,
		},
		{
			name:       "sha_pinning_required not comparable because actions is disabled is not applicable when actions stays disabled",
			in:         shaPinningDisabled,
			settings:   []SettingPlan{generalMismatch, shaPinningDisabled},
			want:       NotApplicable,
			wantReason: NotApplicableActionsRemainDisabled,
		},
		{
			name:     "sha_pinning_required not comparable because actions is disabled is applicable when the same plan enables actions",
			in:       shaPinningDisabled,
			settings: []SettingPlan{enablesActions, shaPinningDisabled},
			want:     Applicable,
		},
		{
			name:       "access_level not comparable because the repository is public is not applicable",
			in:         accessLevelPublic,
			want:       NotApplicable,
			wantReason: NotApplicablePublicRepository,
		},
		{
			name:       "access_level not comparable because the repository is public stays not applicable when the same plan enables actions",
			in:         accessLevelPublic,
			settings:   []SettingPlan{enablesActions, accessLevelPublic},
			want:       NotApplicable,
			wantReason: NotApplicablePublicRepository,
		},
		{
			name:       "environment whose existence cannot be determined is not applicable",
			in:         undeterminedEnvironment,
			want:       NotApplicable,
			wantReason: NotApplicableEnvironmentNameUndetermined,
		},
		{
			name:       "mode of an environment whose existence cannot be determined is not applicable",
			in:         environmentPolicy(production, github.UnavailableEnvironmentNameUndetermined),
			settings:   []SettingPlan{undeterminedEnvironment, environmentPolicy(production, github.UnavailableEnvironmentNameUndetermined)},
			want:       NotApplicable,
			wantReason: NotApplicableEnvironmentNameUndetermined,
		},
		{
			name:       "secret_scanning not comparable because the repository is private is not applicable",
			in:         notComparable(setting.SecretScanningEnabled, github.UnavailablePrivateRepository),
			want:       NotApplicable,
			wantReason: NotApplicablePrivateRepository,
		},
		{
			name:       "push protection not comparable because the repository is private is not applicable",
			in:         notComparable(setting.RepositoryPushProtection, github.UnavailablePrivateRepository),
			want:       NotApplicable,
			wantReason: NotApplicablePrivateRepository,
		},
		{
			name:       "private_vulnerability_reporting not comparable because the repository is private is not applicable",
			in:         notComparable(setting.PrivateVulnerabilityReportingEnabled, github.UnavailablePrivateRepository),
			want:       NotApplicable,
			wantReason: NotApplicablePrivateRepository,
		},
		{
			name:     "mode of an absent environment created in the same plan is applicable",
			in:       environmentPolicy(production, github.UnavailableEnvironmentAbsent),
			settings: []SettingPlan{creation(production), environmentPolicy(production, github.UnavailableEnvironmentAbsent)},
			want:     Applicable,
		},
		{
			name:     "variable of an absent environment created in the same plan is applicable",
			in:       environmentVariable(production, github.UnavailableEnvironmentAbsent),
			settings: []SettingPlan{creation(production), environmentVariable(production, github.UnavailableEnvironmentAbsent)},
			want:     Applicable,
		},
		{
			name:       "variable of an environment whose existence cannot be determined is not applicable",
			in:         environmentVariable(production, github.UnavailableEnvironmentNameUndetermined),
			settings:   []SettingPlan{undeterminedEnvironment, environmentVariable(production, github.UnavailableEnvironmentNameUndetermined)},
			want:       NotApplicable,
			wantReason: NotApplicableEnvironmentNameUndetermined,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := tc.settings
			if settings == nil {
				settings = []SettingPlan{tc.in}
			}
			got, reason := judgeApplicability(tc.in, settings, github.ObservationResult{})
			if got != tc.want || reason != tc.wantReason {
				t.Errorf("judgeApplicability() = %v, %v, want %v, %v", got, reason, tc.want, tc.wantReason)
			}
		})
	}

	// 遷移表にない比較不能を適用可能として黙って通さない
	panics := []struct {
		name     string
		in       SettingPlan
		settings []SettingPlan
	}{
		{name: "allowed_actions not comparable for a reason other than disabled actions panics", in: notComparable(setting.AllowedActions, github.UnavailableNotSelected)},
		{name: "not comparable setting without a fetch condition panics", in: notComparable(setting.AllowAutoMerge, github.UnavailableActionsDisabled)},
		{name: "sha_pinning_required not comparable for a reason other than disabled actions panics", in: notComparable(setting.SHAPinningRequired, github.UnavailableNotSelected)},
		{name: "access_level not comparable for a reason other than a public repository panics", in: notComparable(setting.ActionsAccessLevel, github.UnavailableActionsDisabled)},
		{name: "setting other than access_level not comparable because the repository is public panics", in: notComparable(setting.AllowAutoMerge, github.UnavailablePublicRepository)},
		{
			name:     "mode of an absent environment without its creation in the plan panics",
			in:       environmentPolicy(production, github.UnavailableEnvironmentAbsent),
			settings: []SettingPlan{creation(staging), environmentPolicy(production, github.UnavailableEnvironmentAbsent)},
		},
		{
			name:     "variable of an absent environment without its creation in the plan panics",
			in:       environmentVariable(production, github.UnavailableEnvironmentAbsent),
			settings: []SettingPlan{creation(staging), environmentVariable(production, github.UnavailableEnvironmentAbsent)},
		},
		{name: "repository setting not comparable because an environment is absent panics", in: notComparable(setting.AllowAutoMerge, github.UnavailableEnvironmentAbsent)},
		{name: "setting other than those available only on public repositories not comparable because the repository is private panics", in: notComparable(setting.AllowAutoMerge, github.UnavailablePrivateRepository)},
		{name: "secret scanning not comparable for a reason other than a private repository panics", in: notComparable(setting.SecretScanningEnabled, github.UnavailablePublicRepository)},
		{name: "private_vulnerability_reporting not comparable for a reason other than a private repository panics", in: notComparable(setting.PrivateVulnerabilityReportingEnabled, github.UnavailablePublicRepository)},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			settings := tc.settings
			if settings == nil {
				settings = []SettingPlan{tc.in}
			}
			defer func() {
				if recover() == nil {
					t.Error("judgeApplicability() did not panic")
				}
			}()
			_, _ = judgeApplicability(tc.in, settings, github.ObservationResult{})
		})
	}
}

//trace:checks FR-006 INV-001
func TestJudgeApplicability_DependabotSecurityUpdates(t *testing.T) {
	updatesPlan := func(observed setting.DependabotSecurityUpdatesState, desired bool) SettingPlan {
		return newSettingPlan(setting.Unit{Key: setting.DependabotSecurityUpdates}, github.Observed(setting.DependabotSecurityUpdatesValue(observed)), setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesStateOf(desired)))
	}
	alertsPlan := func(observed, desired bool) SettingPlan {
		return newSettingPlan(setting.Unit{Key: setting.DependabotAlerts}, github.Observed(setting.BoolValue(observed)), setting.BoolValue(desired))
	}
	observation := func(alerts bool, updates setting.DependabotSecurityUpdatesState) github.ObservationResult {
		return github.ObservationResult{Dependabot: &github.Dependabot{Alerts: alerts, SecurityUpdates: updates}}
	}
	cases := []struct {
		name       string
		in         SettingPlan
		alerts     []SettingPlan
		observed   github.ObservationResult
		want       Applicability
		wantReason NotApplicableReason
	}{
		{
			name:     "enabling with alerts enabled and undeclared is applicable",
			in:       updatesPlan(setting.DependabotSecurityUpdatesDisabled, true),
			observed: observation(true, setting.DependabotSecurityUpdatesDisabled),
			want:     Applicable,
		},
		{
			name:     "disabling with alerts enabled and to be disabled is applicable",
			in:       updatesPlan(setting.DependabotSecurityUpdatesEnabled, false),
			alerts:   []SettingPlan{alertsPlan(true, false)},
			observed: observation(true, setting.DependabotSecurityUpdatesEnabled),
			want:     Applicable,
		},
		{
			name:     "enabling with alerts disabled and declared true is applicable",
			in:       updatesPlan(setting.DependabotSecurityUpdatesDisabled, true),
			alerts:   []SettingPlan{alertsPlan(false, true)},
			observed: observation(false, setting.DependabotSecurityUpdatesDisabled),
			want:     Applicable,
		},
		{
			name:       "enabling with alerts disabled and undeclared is not applicable because alerts remain disabled",
			in:         updatesPlan(setting.DependabotSecurityUpdatesDisabled, true),
			observed:   observation(false, setting.DependabotSecurityUpdatesDisabled),
			want:       NotApplicable,
			wantReason: NotApplicableDependabotAlertsRemainDisabled,
		},
		{
			name:       "disabling with alerts disabled and undeclared is not applicable because GitHub rejects it too",
			in:         updatesPlan(setting.DependabotSecurityUpdatesEnabled, false),
			observed:   observation(false, setting.DependabotSecurityUpdatesEnabled),
			want:       NotApplicable,
			wantReason: NotApplicableDependabotAlertsRemainDisabled,
		},
		{
			name:       "disabling with alerts disabled and declared false is not applicable",
			in:         updatesPlan(setting.DependabotSecurityUpdatesEnabled, false),
			alerts:     []SettingPlan{alertsPlan(false, false)},
			observed:   observation(false, setting.DependabotSecurityUpdatesEnabled),
			want:       NotApplicable,
			wantReason: NotApplicableDependabotAlertsRemainDisabled,
		},
		{
			name:     "alerts mismatch is applicable",
			in:       alertsPlan(true, false),
			observed: observation(true, setting.DependabotSecurityUpdatesEnabled),
			want:     Applicable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := slices.Concat(tc.alerts, []SettingPlan{tc.in})
			got, reason := judgeApplicability(tc.in, settings, tc.observed)
			if got != tc.want || reason != tc.wantReason {
				t.Errorf("judgeApplicability() = %v, %v, want %v, %v", got, reason, tc.want, tc.wantReason)
			}
		})
	}
}

//trace:checks FR-006 INV-001
func TestJudgeApplicability_RepositoryPushProtection(t *testing.T) {
	boolPlan := func(key setting.Key, observed, desired bool) SettingPlan {
		return newSettingPlan(setting.Unit{Key: key}, github.Observed(setting.BoolValue(observed)), setting.BoolValue(desired))
	}
	observation := func(enabled, pushProtection bool) github.ObservationResult {
		return github.ObservationResult{SecretScanning: &github.SecretScanning{Enabled: github.Observed(enabled), PushProtection: github.Observed(pushProtection)}}
	}
	cases := []struct {
		name           string
		in             SettingPlan
		secretScanning []SettingPlan
		observed       github.ObservationResult
		want           Applicability
		wantReason     NotApplicableReason
	}{
		{
			name:     "enabling with secret scanning enabled and undeclared is applicable",
			in:       boolPlan(setting.RepositoryPushProtection, false, true),
			observed: observation(true, false),
			want:     Applicable,
		},
		{
			name:           "enabling with secret scanning disabled and declared true is applicable",
			in:             boolPlan(setting.RepositoryPushProtection, false, true),
			secretScanning: []SettingPlan{boolPlan(setting.SecretScanningEnabled, false, true)},
			observed:       observation(false, false),
			want:           Applicable,
		},
		{
			name:       "enabling with secret scanning disabled and undeclared is not applicable because GitHub does not apply it",
			in:         boolPlan(setting.RepositoryPushProtection, false, true),
			observed:   observation(false, false),
			want:       NotApplicable,
			wantReason: NotApplicableSecretScanningRemainsDisabled,
		},
		{
			name:     "disabling with secret scanning disabled and undeclared is applicable because only enabling is not applied",
			in:       boolPlan(setting.RepositoryPushProtection, true, false),
			observed: observation(false, true),
			want:     Applicable,
		},
		{
			name:     "secret scanning mismatch is applicable",
			in:       boolPlan(setting.SecretScanningEnabled, true, false),
			observed: observation(true, true),
			want:     Applicable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := slices.Concat(tc.secretScanning, []SettingPlan{tc.in})
			got, reason := judgeApplicability(tc.in, settings, tc.observed)
			if got != tc.want || reason != tc.wantReason {
				t.Errorf("judgeApplicability() = %v, %v, want %v, %v", got, reason, tc.want, tc.wantReason)
			}
		})
	}
}

func TestExpectsActionsEnabled(t *testing.T) {
	plan := func(key setting.Key, desired setting.Value, result ReconciliationResult) SettingPlan {
		return SettingPlan{Unit: setting.Unit{Key: key}, Desired: desired, Result: result}
	}
	cases := []struct {
		name     string
		settings []SettingPlan
		want     bool
	}{
		{name: "general settings alone do not expect actions enabled", settings: []SettingPlan{plan(setting.AllowAutoMerge, setting.BoolValue(true), Mismatch)}, want: false},
		{name: "matched allowed_actions expects actions enabled", settings: []SettingPlan{plan(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsAll), Match)}, want: true},
		{name: "enabled declared true expects actions enabled", settings: []SettingPlan{plan(setting.ActionsEnabled, setting.BoolValue(true), Match)}, want: true},
		{name: "enabled declared false does not expect actions enabled", settings: []SettingPlan{plan(setting.ActionsEnabled, setting.BoolValue(false), Mismatch)}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := expectsActionsEnabled(tc.settings); got != tc.want {
				t.Errorf("expectsActionsEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-006 INV-001
func TestJudgeApplicability_MergeMethods(t *testing.T) {
	disableSquash := newSettingPlan(setting.Unit{Key: setting.AllowSquashMerge}, github.Observed(setting.BoolValue(true)), setting.BoolValue(false))
	cases := []struct {
		name       string
		observed   github.ObservationResult
		want       Applicability
		wantReason NotApplicableReason
	}{
		{
			name:     "disabling with another merge method left enabled is applicable",
			observed: github.ObservationResult{General: github.General{AllowSquashMerge: true, AllowRebaseMerge: true}},
			want:     Applicable,
		},
		{
			name:       "disabling the last enabled merge method is not applicable",
			observed:   github.ObservationResult{General: github.General{AllowSquashMerge: true}},
			want:       NotApplicable,
			wantReason: NotApplicableNoMergeMethodEnabled,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := judgeApplicability(disableSquash, []SettingPlan{disableSquash}, tc.observed)
			if got != tc.want || reason != tc.wantReason {
				t.Errorf("judgeApplicability() = %v, %v, want %v, %v", got, reason, tc.want, tc.wantReason)
			}
		})
	}
}
