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
		identity RulesetIdentity
		wantKind UpdateOperationKind
	}{
		{name: "absent ruleset is a creation based on its absence without an ID", observed: setting.Absent(), wantKind: RulesetCreation},
		{name: "different value is a value update based on the planned value and ID", observed: setting.RulesetValue(disabled), identity: ObservedRulesetIdentity(42), wantKind: RulesetValueUpdate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := SettingPlan{Unit: unit, Observed: github.Observed(tc.observed), Desired: desired, RulesetIdentity: tc.identity, Result: Mismatch, Applicability: Applicable()}
			want := LogicalChange{
				DesiredState: []DesiredSetting{{Unit: unit, Value: desired}},
				Operations: []UpdateOperation{{
					Kind: tc.wantKind, Unit: unit, Value: desired,
					RelevantSettings: []RelevantSetting{{Unit: unit, AnticipatedValue: github.Observed(tc.observed), AnticipatedRulesetIdentity: tc.identity}},
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

// TestUpdateOperation_AnticipatedRulesetID は、更新操作の宛先の ID を、変更前提の基準のうち更新操作の管理の単位のものから返し、基準がなければ panic することを確かめる。
//
//trace:checks INV-004
func TestUpdateOperation_AnticipatedRulesetID(t *testing.T) {
	unit := rulesetUnit(t, "protect")
	other := rulesetUnit(t, "release")
	operation := UpdateOperation{
		Kind: RulesetValueUpdate, Unit: unit,
		RelevantSettings: []RelevantSetting{
			{Unit: other, AnticipatedRulesetIdentity: ObservedRulesetIdentity(7)},
			{Unit: unit, AnticipatedRulesetIdentity: ObservedRulesetIdentity(42)},
		},
	}
	if got := operation.AnticipatedRulesetID(); got != 42 {
		t.Errorf("AnticipatedRulesetID() = %v, want 42", got)
	}

	panics := []struct {
		name      string
		operation UpdateOperation
	}{
		{name: "relevant setting of another ruleset panics", operation: UpdateOperation{Kind: RulesetValueUpdate, Unit: unit, RelevantSettings: []RelevantSetting{{Unit: other, AnticipatedRulesetIdentity: ObservedRulesetIdentity(7)}}}},
		{name: "relevant setting without an observed ID panics", operation: UpdateOperation{Kind: RulesetValueUpdate, Unit: unit, RelevantSettings: []RelevantSetting{{Unit: unit}}}},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("AnticipatedRulesetID() did not panic")
				}
			}()
			tc.operation.AnticipatedRulesetID()
		})
	}
}
