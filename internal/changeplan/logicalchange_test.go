package changeplan

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks INV-002
//trace:checks FR-008
func TestPlan_LogicalChanges(t *testing.T) {
	match := SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match}
	mismatch := SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
	newPlan := func(settings ...SettingPlan) Plan {
		return Plan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}

	t.Run("all matches have no logical change", func(t *testing.T) {
		got, err := newPlan(match).LogicalChanges()
		if err != nil || len(got) != 0 {
			t.Errorf("LogicalChanges() = %+v, error = %v, want none without error", got, err)
		}
	})

	t.Run("applicable mismatch becomes a logical change with the observed and desired values", func(t *testing.T) {
		got, err := newPlan(match, mismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true)}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("applicable mismatches of both settings become logical changes in the plan order", func(t *testing.T) {
		deleteBranch := SettingPlan{Key: setting.DeleteBranchOnMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
		got, err := newPlan(deleteBranch, mismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			{Key: setting.DeleteBranchOnMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true)},
			{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true)},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	policyChange := func(observed, desired setting.AllowedActionsPolicy, effects ...SideEffect) SettingPlan {
		return SettingPlan{
			Key: setting.AllowedActions, Observed: github.Observed(setting.AllowedActionsPolicyValue(observed)), Desired: setting.AllowedActionsPolicyValue(desired),
			Result: Mismatch, Applicability: Applicable, SideEffects: effects,
		}
	}

	t.Run("applicable mismatch without side effects becomes a logical change", func(t *testing.T) {
		got, err := newPlan(policyChange(setting.AllowedActionsAll, setting.AllowedActionsLocalOnly)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{
			Key:      setting.AllowedActions,
			Observed: setting.AllowedActionsPolicyValue(setting.AllowedActionsAll), Desired: setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly),
		}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	// 遷移先が selected のポリシーの変更予定は、それ単独では副作用のない不一致だが、YAML 設定仕様の allowed_actions と
	// selected_actions の組み合わせの規則により同じ計画に必ず並ぶ、選択方式の Actions 許可条件の比較不能な変更予定で拒否される
	t.Run("policy change to selected with not-comparable criteria is an error", func(t *testing.T) {
		criteria := SettingPlan{
			Key: setting.SelectedActions, Observed: github.Unavailable[setting.Value](github.UnavailableNotSelected),
			Desired: setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true}), Result: NotComparable, Applicability: Applicable,
		}
		if got, err := newPlan(policyChange(setting.AllowedActionsAll, setting.AllowedActionsSelected), criteria).LogicalChanges(); err == nil {
			t.Errorf("LogicalChanges() = %+v, error = nil, want an error", got)
		}
	})

	lost := SideEffect{Key: setting.SelectedActions, Observed: github.Observed(setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true}))}
	errorCases := []struct {
		name    string
		setting SettingPlan
	}{
		{
			name:    "applicable mismatch with a side effect is an error because the lost value cannot be checked before the update",
			setting: policyChange(setting.AllowedActionsSelected, setting.AllowedActionsAll, lost),
		},
		{
			name:    "not-applicable mismatch is an error",
			setting: SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: NotApplicable},
		},
		{
			name:    "applicable not-comparable planned change is an error because it has no observed value",
			setting: SettingPlan{Key: setting.AllowedActions, Observed: github.Unavailable[setting.Value](github.UnavailableActionsDisabled), Desired: setting.AllowedActionsPolicyValue(setting.AllowedActionsAll), Result: NotComparable, Applicability: Applicable},
		},
		{
			name:    "mismatch without an applicability judgment is an error",
			setting: SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: ApplicabilityNotJudged},
		},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			// 適用可能な変更予定と並べ、ほかの変更予定があっても承認の対象にしないことを確かめる
			if got, err := newPlan(mismatch, tc.setting).LogicalChanges(); err == nil {
				t.Errorf("LogicalChanges() = %+v, error = nil, want an error", got)
			}
		})
	}
}
