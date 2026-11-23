package changeplan

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestRulesetChangeOf は、Ruleset の変更予定を、計画時の観測から種類を選んだ更新操作一つの論理的な設定変更にし、
// 計画時の観測を変更前提の基準に、宣言の値を送る値にすることを確かめる。
//
//trace:checks INV-004 FR-011
func TestRulesetChangeOf(t *testing.T) {
	unit := rulesetUnit(t, "protect")
	desired := setting.RulesetValue(protectRuleset())
	disabled := protectRuleset()
	disabled.Enforcement = setting.RulesetEnforcementDisabled
	cases := []struct {
		name     string
		observed setting.Value
		wantKind UpdateOperationKind
	}{
		{name: "absent ruleset is a creation based on its absence", observed: setting.Absent(), wantKind: RulesetCreation},
		{name: "different value is a value update based on the planned value", observed: setting.RulesetValue(disabled), wantKind: RulesetValueUpdate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := SettingPlan{Unit: unit, Observed: github.Observed(tc.observed), Desired: desired, Result: Mismatch, Applicability: Applicable()}
			want := LogicalChange{
				DesiredState: []DesiredSetting{{Unit: unit, Value: desired}},
				Operations: []UpdateOperation{{
					Kind: tc.wantKind, Unit: unit, Value: desired,
					RelevantSettings: []RelevantSetting{{Unit: unit, AnticipatedValue: github.Observed(tc.observed)}},
				}},
			}
			if got := rulesetChangeOf(s); !reflect.DeepEqual(got, want) {
				t.Errorf("rulesetChangeOf() = %+v, want %+v", got, want)
			}
		})
	}

	// 不一致でない計画を、作成とも値の変更とも取り違えない
	panics := []struct {
		name string
		plan SettingPlan
	}{
		{name: "matched ruleset panics", plan: SettingPlan{Unit: unit, Observed: github.Observed(desired), Desired: desired, Result: Match}},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("rulesetChangeOf() did not panic")
				}
			}()
			rulesetChangeOf(tc.plan)
		})
	}
}
