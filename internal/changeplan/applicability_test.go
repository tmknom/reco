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
	cases := []struct {
		name string
		in   SettingPlan
		want Applicability
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
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := judgeApplicability(tc.in); got != tc.want {
				t.Errorf("judgeApplicability() = %v, want %v", got, tc.want)
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
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("judgeApplicability() did not panic")
				}
			}()
			_ = judgeApplicability(tc.in)
		})
	}
}
