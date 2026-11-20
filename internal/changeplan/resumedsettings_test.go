package changeplan

import (
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-007
func TestResumedSettings(t *testing.T) {
	enabled := func(observed, desired bool, result ReconciliationResult) SettingPlan {
		return SettingPlan{Key: setting.ActionsEnabled, Observed: github.Observed(setting.BoolValue(observed)), Desired: setting.BoolValue(desired), Result: result}
	}
	disabled := github.Unavailable[setting.Value](github.UnavailableActionsDisabled)
	shaPinningRequired := SettingPlan{Key: setting.SHAPinningRequired, Observed: disabled, Desired: setting.BoolValue(true), Result: NotComparable}
	localOnly := setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly)
	enablingPolicy := SettingPlan{Key: setting.AllowedActions, Observed: disabled, Desired: localOnly, Result: NotComparable}
	policyMismatch := SettingPlan{
		Key: setting.AllowedActions, Observed: github.Observed(setting.AllowedActionsPolicyValue(setting.AllowedActionsAll)), Desired: localOnly, Result: Mismatch,
	}
	cases := []struct {
		name     string
		plan     SettingPlan
		settings []SettingPlan
		want     []setting.Key
	}{
		{
			name: "reenabling actions alone resumes with the policy, the criteria, and sha_pinning_required",
			plan: enabled(false, true, Mismatch), settings: []SettingPlan{enabled(false, true, Mismatch)},
			want: []setting.Key{setting.AllowedActions, setting.SelectedActions, setting.SHAPinningRequired},
		},
		{
			name: "reenabling actions with sha_pinning_required declared does not list it",
			plan: enabled(false, true, Mismatch), settings: []SettingPlan{enabled(false, true, Mismatch), shaPinningRequired},
			want: []setting.Key{setting.AllowedActions, setting.SelectedActions},
		},
		{
			name: "policy enabling actions resumes with sha_pinning_required only",
			plan: enablingPolicy, settings: []SettingPlan{enablingPolicy},
			want: []setting.Key{setting.SHAPinningRequired},
		},
		{
			name: "policy enabling actions with sha_pinning_required declared has no resumed settings",
			plan: enablingPolicy, settings: []SettingPlan{enablingPolicy, shaPinningRequired},
		},
		{name: "policy change on enabled actions has no resumed settings", plan: policyMismatch, settings: []SettingPlan{policyMismatch}},
		{name: "disabling actions has no resumed settings", plan: enabled(true, false, Mismatch), settings: []SettingPlan{enabled(true, false, Mismatch)}},
		{name: "enabled already true has no resumed settings", plan: enabled(true, true, Match), settings: []SettingPlan{enabled(true, true, Match)}},
		{
			name: "another setting from false to true has no resumed settings",
			plan: SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resumedSettings(tc.plan, tc.settings); !slices.Equal(got, tc.want) {
				t.Errorf("resumedSettings() = %v, want %v", got, tc.want)
			}
		})
	}
}
