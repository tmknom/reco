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
		{name: "absent repository variable is a creation based on its absence", unit: repositoryUnit, anticipated: setting.AbsentVariable(), wantKind: RepositoryVariableCreation},
		{name: "different repository variable value is a value update based on the planned value", unit: repositoryUnit, anticipated: setting.VariableValue("1.26"), wantKind: RepositoryVariableValueUpdate},
		{name: "empty value is an existing variable and a value update", unit: repositoryUnit, anticipated: setting.VariableValue(""), wantKind: RepositoryVariableValueUpdate},
		{name: "absent environment variable is an environment variable creation", unit: environmentUnit, anticipated: setting.AbsentVariable(), wantKind: EnvironmentVariableCreation},
		{name: "different environment variable value is an environment variable value update", unit: environmentUnit, anticipated: setting.VariableValue("1.26"), wantKind: EnvironmentVariableValueUpdate},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := SettingPlan{Unit: tc.unit, Desired: desired, Result: Mismatch, Applicability: Applicable}
			want := UpdateOperation{
				Kind: tc.wantKind, Unit: tc.unit, Value: desired,
				RelevantSettings: []RelevantSetting{{Unit: tc.unit, AnticipatedValue: github.Observed(tc.anticipated)}},
			}
			if got := variableOperationOf(s, github.Observed(tc.anticipated)); !reflect.DeepEqual(got, want) {
				t.Errorf("variableOperationOf() = %+v, want %+v", got, want)
			}
		})
	}

	panics := []struct {
		name        string
		unit        setting.Unit
		anticipated github.SettingObservation[setting.Value]
	}{
		{name: "not comparable anticipated value panics", unit: repositoryUnit, anticipated: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)},
		{name: "unit other than a variable panics", unit: setting.Unit{Key: setting.AllowAutoMerge}, anticipated: github.Observed(setting.AbsentVariable())},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("variableOperationOf() did not panic")
				}
			}()
			variableOperationOf(SettingPlan{Unit: tc.unit, Desired: desired, Result: Mismatch, Applicability: Applicable}, tc.anticipated)
		})
	}
}
