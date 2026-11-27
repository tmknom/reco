package changeplan

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestRulesetChangeOf は、Ruleset の変更予定を、計画時の観測から種類を選んだ更新操作一つの論理的な設定変更にし、
// 計画時の観測を変更前提の基準にすることと、作成と値の変更は宣言の値を送り、削除は値を送らないことを確かめる。
//
//trace:checks INV-001 INV-004 FR-010 FR-011
func TestRulesetChangeOf(t *testing.T) {
	unit := rulesetUnit(t, "protect")
	desired := setting.RulesetValue(protectRuleset())
	disabled := protectRuleset()
	disabled.Enforcement = setting.RulesetEnforcementDisabled
	cases := []struct {
		name     string
		observed setting.Value
		identity RulesetIdentity
		desired  setting.Value
		wantKind UpdateOperationKind
		wantSent setting.Value
	}{
		{name: "absent ruleset is a creation based on its absence without an ID", observed: setting.Absent(), desired: desired, wantKind: RulesetCreation, wantSent: desired},
		{name: "different value is a value update based on the planned value and ID", observed: setting.RulesetValue(disabled), identity: ObservedRulesetIdentity(42), desired: desired, wantKind: RulesetValueUpdate, wantSent: desired},
		{name: "declared absence of an existing ruleset is a deletion based on the planned value and ID sending no value", observed: setting.RulesetValue(disabled), identity: ObservedRulesetIdentity(42), desired: setting.Absent(), wantKind: RulesetDeletion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := SettingPlan{Unit: unit, Observed: github.Observed(tc.observed), Desired: tc.desired, RulesetIdentity: tc.identity, Result: Mismatch, Applicability: Applicable()}
			want := LogicalChange{
				DesiredState: []configuration.ManagedSetting{{Unit: unit, Desired: tc.desired}},
				Operations: []UpdateOperation{{
					Kind: tc.wantKind, Unit: unit, Value: tc.wantSent,
					PreconditionBases: []PreconditionBasis{{Unit: unit, AnticipatedValue: AnticipateValue(tc.observed), AnticipatedRulesetIdentity: tc.identity}},
				}},
			}
			if got := rulesetChangeOf(s); !reflect.DeepEqual(got, want) {
				t.Errorf("rulesetChangeOf() = %+v, want %+v", got, want)
			}
		})
	}

	// 不一致でない計画を、作成とも値の変更とも削除とも取り違えない
	panics := []struct {
		name string
		plan SettingPlan
	}{
		{name: "matched ruleset panics", plan: SettingPlan{Unit: unit, Observed: github.Observed(desired), Desired: desired, Result: Match}},
		{name: "absent ruleset declared absent panics", plan: SettingPlan{Unit: unit, Observed: github.Observed(setting.Absent()), Desired: setting.Absent(), Result: Mismatch}},
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
		PreconditionBases: []PreconditionBasis{
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
		{name: "precondition basis of another ruleset panics", operation: UpdateOperation{Kind: RulesetValueUpdate, Unit: unit, PreconditionBases: []PreconditionBasis{{Unit: other, AnticipatedRulesetIdentity: ObservedRulesetIdentity(7)}}}},
		{name: "precondition basis without an observed ID panics", operation: UpdateOperation{Kind: RulesetValueUpdate, Unit: unit, PreconditionBases: []PreconditionBasis{{Unit: unit}}}},
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
