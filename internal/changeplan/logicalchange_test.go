package changeplan

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks INV-001
//trace:checks INV-002
//trace:checks FR-008
//trace:checks FR-011
func TestPlan_LogicalChanges(t *testing.T) {
	match := SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match}
	mismatch := SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
	newPlan := func(settings ...SettingPlan) Plan {
		return Plan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}
	relevant := func(key setting.Key, atApproval github.SettingObservation[setting.Value]) RelevantSetting {
		return RelevantSetting{Key: key, AtApproval: atApproval}
	}

	t.Run("all matches have no logical change", func(t *testing.T) {
		got, err := newPlan(match).LogicalChanges()
		if err != nil || len(got) != 0 {
			t.Errorf("LogicalChanges() = %+v, error = %v, want none without error", got, err)
		}
	})

	t.Run("applicable mismatch becomes a logical change relevant to its own observed value", func(t *testing.T) {
		got, err := newPlan(match, mismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{
			Key: setting.AllowAutoMerge, Desired: setting.BoolValue(true),
			RelevantSettings: []RelevantSetting{relevant(setting.AllowAutoMerge, github.Observed(setting.BoolValue(false)))},
		}}
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
			{
				Key: setting.DeleteBranchOnMerge, Desired: setting.BoolValue(true),
				RelevantSettings: []RelevantSetting{relevant(setting.DeleteBranchOnMerge, github.Observed(setting.BoolValue(false)))},
			},
			{
				Key: setting.AllowAutoMerge, Desired: setting.BoolValue(true),
				RelevantSettings: []RelevantSetting{relevant(setting.AllowAutoMerge, github.Observed(setting.BoolValue(false)))},
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	criteria := setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"octo-org/*"}})
	disabled := github.Unavailable[setting.Value](github.UnavailableActionsDisabled)
	policy := func(p setting.AllowedActionsPolicy) setting.Value { return setting.AllowedActionsPolicyValue(p) }
	policyChange := func(observed, desired setting.AllowedActionsPolicy, effects ...SideEffect) SettingPlan {
		return SettingPlan{
			Key: setting.AllowedActions, Observed: github.Observed(policy(observed)), Desired: policy(desired),
			Result: Mismatch, Applicability: Applicable, SideEffects: effects,
		}
	}

	t.Run("applicable mismatch without side effects becomes a logical change", func(t *testing.T) {
		got, err := newPlan(policyChange(setting.AllowedActionsAll, setting.AllowedActionsLocalOnly)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{
			Key: setting.AllowedActions, Desired: policy(setting.AllowedActionsLocalOnly),
			RelevantSettings: []RelevantSetting{relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsAll)))},
		}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("mismatch with a side effect is also relevant to the lost value shown before approval", func(t *testing.T) {
		lost := SideEffect{Key: setting.SelectedActions, Observed: github.Observed(criteria)}
		got, err := newPlan(policyChange(setting.AllowedActionsSelected, setting.AllowedActionsAll, lost)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{
			Key: setting.AllowedActions, Desired: policy(setting.AllowedActionsAll),
			RelevantSettings: []RelevantSetting{
				relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsSelected))),
				relevant(setting.SelectedActions, github.Observed(criteria)),
			},
		}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("policy not comparable because actions is disabled is relevant to the unavailable values", func(t *testing.T) {
		enable := SettingPlan{
			Key: setting.AllowedActions, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly), Result: NotComparable, Applicability: Applicable,
			SideEffects: []SideEffect{{Key: setting.SelectedActions, Observed: disabled}},
		}
		got, err := newPlan(enable).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{
			Key: setting.AllowedActions, Desired: policy(setting.AllowedActionsLocalOnly),
			RelevantSettings: []RelevantSetting{relevant(setting.AllowedActions, disabled), relevant(setting.SelectedActions, disabled)},
		}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	notComparableCriteria := func(reason github.UnavailableReason) SettingPlan {
		return SettingPlan{
			Key: setting.SelectedActions, Observed: github.Unavailable[setting.Value](reason),
			Desired: criteria, Result: NotComparable, Applicability: Applicable,
		}
	}
	errorCases := []struct {
		name    string
		setting SettingPlan
	}{
		// 遷移先が selected の変更は、ポリシーと許可条件のどちらも単独で拒否されることを確かめる
		{
			name:    "policy mismatch to selected is an error without the criteria",
			setting: policyChange(setting.AllowedActionsAll, setting.AllowedActionsSelected),
		},
		{
			name:    "policy enabling actions with selected is an error without the criteria",
			setting: SettingPlan{Key: setting.AllowedActions, Observed: disabled, Desired: policy(setting.AllowedActionsSelected), Result: NotComparable, Applicability: Applicable},
		},
		{
			name:    "criteria not comparable because the policy is not selected is an error",
			setting: notComparableCriteria(github.UnavailableNotSelected),
		},
		{
			name:    "not-applicable mismatch is an error",
			setting: SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: NotApplicable},
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
