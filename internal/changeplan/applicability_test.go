package changeplan

import (
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-006
func TestJudgeApplicability(t *testing.T) {
	notComparable := func(key setting.Key, reason github.UnavailableReason) SettingPlan {
		return SettingPlan{Key: key, Observed: github.Unavailable[setting.Value](reason), Result: NotComparable}
	}
	shaPinningDisabled := notComparable(setting.SHAPinningRequired, github.UnavailableActionsDisabled)
	generalMismatch := SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch}
	enablesActions := SettingPlan{Key: setting.ActionsEnabled, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch}
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
			in:   SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match},
			want: ApplicabilityNotJudged,
		},
		{
			name: "mismatch is applicable",
			in:   SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
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
			in:   SettingPlan{Key: setting.SHAPinningRequired, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := tc.settings
			if settings == nil {
				settings = []SettingPlan{tc.in}
			}
			got, reason := judgeApplicability(tc.in, settings)
			if got != tc.want || reason != tc.wantReason {
				t.Errorf("judgeApplicability() = %v, %v, want %v, %v", got, reason, tc.want, tc.wantReason)
			}
		})
	}

	// 遷移表にない比較不能を適用可能として黙って通さない
	panics := []struct {
		name string
		in   SettingPlan
	}{
		{name: "allowed_actions not comparable for a reason other than disabled actions panics", in: notComparable(setting.AllowedActions, github.UnavailableNotSelected)},
		{name: "not comparable setting without a fetch condition panics", in: notComparable(setting.AllowAutoMerge, github.UnavailableActionsDisabled)},
		{name: "sha_pinning_required not comparable for a reason other than disabled actions panics", in: notComparable(setting.SHAPinningRequired, github.UnavailableNotSelected)},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("judgeApplicability() did not panic")
				}
			}()
			_, _ = judgeApplicability(tc.in, []SettingPlan{tc.in})
		})
	}
}

func TestExpectsActionsEnabled(t *testing.T) {
	plan := func(key setting.Key, desired setting.Value, result ReconciliationResult) SettingPlan {
		return SettingPlan{Key: key, Desired: desired, Result: result}
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
