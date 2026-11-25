package changeplan

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestVariableOperationOf は、承認時に予定した変数の値から、変数の更新操作の種類と変更前提の基準を決めることを確かめる。
// 種類の組は、Repository Variable と Environment Variable で宛先が違うため、管理の単位の形で選ぶ。
//
//trace:checks INV-001 INV-004 FR-010 FR-011
func TestVariableOperationOf(t *testing.T) {
	repositoryUnit := variableUnit(t, "GO_VERSION")
	environmentUnit := environmentVariableUnit(mustEnvironmentName(t, "production"), mustVariableName(t, "GO_VERSION"))
	desired := setting.VariableValue("1.27")
	cases := []struct {
		name        string
		unit        setting.Unit
		anticipated setting.Value
		wantKind    UpdateOperationKind
	}{
		{name: "absent repository variable is a creation based on its absence", unit: repositoryUnit, anticipated: setting.Absent(), wantKind: RepositoryVariableCreation},
		{name: "different repository variable value is a value update based on the planned value", unit: repositoryUnit, anticipated: setting.VariableValue("1.26"), wantKind: RepositoryVariableValueUpdate},
		{name: "empty value is an existing variable and a value update", unit: repositoryUnit, anticipated: setting.VariableValue(""), wantKind: RepositoryVariableValueUpdate},
		{name: "absent environment variable is an environment variable creation", unit: environmentUnit, anticipated: setting.Absent(), wantKind: EnvironmentVariableCreation},
		{name: "different environment variable value is an environment variable value update", unit: environmentUnit, anticipated: setting.VariableValue("1.26"), wantKind: EnvironmentVariableValueUpdate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := SettingPlan{Unit: tc.unit, Desired: desired, Result: Mismatch, Applicability: Applicable()}
			want := UpdateOperation{
				Kind: tc.wantKind, Unit: tc.unit, Value: desired,
				PreconditionBases: []PreconditionBasis{{Unit: tc.unit, AnticipatedValue: AnticipateValue(tc.anticipated)}},
			}
			if got := variableOperationOf(s, AnticipateValue(tc.anticipated)); !reflect.DeepEqual(got, want) {
				t.Errorf("variableOperationOf() = %+v, want %+v", got, want)
			}
		})
	}

	// 削除は期待値で選び、送る値を持たない。基準は計画時の値である
	deletions := []struct {
		name        string
		unit        setting.Unit
		anticipated setting.Value
		wantKind    UpdateOperationKind
	}{
		{name: "declared absence of an existing repository variable is a deletion based on the planned value", unit: repositoryUnit, anticipated: setting.VariableValue("1.26"), wantKind: RepositoryVariableDeletion},
		{name: "declared absence of an empty value is a deletion", unit: repositoryUnit, anticipated: setting.VariableValue(""), wantKind: RepositoryVariableDeletion},
		{name: "declared absence of an existing environment variable is an environment variable deletion", unit: environmentUnit, anticipated: setting.VariableValue("1.26"), wantKind: EnvironmentVariableDeletion},
	}
	for _, tc := range deletions {
		t.Run(tc.name, func(t *testing.T) {
			s := SettingPlan{Unit: tc.unit, Desired: setting.Absent(), Result: Mismatch, Applicability: Applicable()}
			want := UpdateOperation{
				Kind: tc.wantKind, Unit: tc.unit,
				PreconditionBases: []PreconditionBasis{{Unit: tc.unit, AnticipatedValue: AnticipateValue(tc.anticipated)}},
			}
			if got := variableOperationOf(s, AnticipateValue(tc.anticipated)); !reflect.DeepEqual(got, want) {
				t.Errorf("variableOperationOf() = %+v, want %+v", got, want)
			}
		})
	}

	panics := []struct {
		name        string
		unit        setting.Unit
		desired     setting.Value
		anticipated github.SettingObservation[setting.Value]
	}{
		{name: "not comparable anticipated value panics", unit: repositoryUnit, desired: desired, anticipated: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)},
		{name: "not comparable anticipated value of a declared absence panics", unit: repositoryUnit, desired: setting.Absent(), anticipated: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent)},
		{name: "declared absence of an absent variable panics", unit: repositoryUnit, desired: setting.Absent(), anticipated: github.Observed(setting.Absent())},
		{name: "unit other than a variable panics", unit: setting.Unit{Key: setting.AllowAutoMerge}, desired: desired, anticipated: github.Observed(setting.Absent())},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("variableOperationOf() did not panic")
				}
			}()
			variableOperationOf(SettingPlan{Unit: tc.unit, Desired: tc.desired, Result: Mismatch, Applicability: Applicable()}, AnticipateUnchanged(tc.anticipated))
		})
	}
}
