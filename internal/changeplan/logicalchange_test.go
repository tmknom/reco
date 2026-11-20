package changeplan

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

func TestLogicalChange_Keys(t *testing.T) {
	change := LogicalChange{Operations: []UpdateOperation{{Key: setting.AllowedActions}, {Key: setting.SelectedActions}}}
	if got, want := change.Keys(), []setting.Key{setting.AllowedActions, setting.SelectedActions}; !reflect.DeepEqual(got, want) {
		t.Errorf("Keys() = %v, want %v", got, want)
	}
}

// TestLogicalChange_SettingsToChange は、変更対象外の設定から除く、論理的な設定変更が変える設定の範囲を確かめる。
//
//trace:checks INV-001
//trace:checks FR-012
func TestLogicalChange_SettingsToChange(t *testing.T) {
	cases := []struct {
		name string
		keys []setting.Key
		want []setting.Key
	}{
		{
			name: "policy change also changes the enabled state",
			keys: []setting.Key{setting.AllowedActions},
			want: []setting.Key{setting.AllowedActions, setting.ActionsEnabled},
		},
		{
			name: "enabled state change changes only the enabled state",
			keys: []setting.Key{setting.ActionsEnabled},
			want: []setting.Key{setting.ActionsEnabled},
		},
		{
			name: "criteria change does not change the enabled state",
			keys: []setting.Key{setting.SelectedActions},
			want: []setting.Key{setting.SelectedActions},
		},
		{
			name: "sha_pinning_required change does not change the enabled state",
			keys: []setting.Key{setting.SHAPinningRequired},
			want: []setting.Key{setting.SHAPinningRequired},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var change LogicalChange
			for _, key := range tc.keys {
				change.Operations = append(change.Operations, UpdateOperation{Key: key})
			}
			if got := change.SettingsToChange(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("SettingsToChange() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks INV-001
//trace:checks INV-002
//trace:checks INV-004
//trace:checks FR-008
//trace:checks FR-011
func TestRepositoryPlan_LogicalChanges(t *testing.T) {
	match := SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match}
	mismatch := SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
	newRepositoryPlan := func(settings ...SettingPlan) RepositoryPlan {
		return RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}
	relevant := func(key setting.Key, atApproval github.SettingObservation[setting.Value]) RelevantSetting {
		return RelevantSetting{Key: key, ValueAtApproval: atApproval}
	}

	t.Run("all matches have no logical change", func(t *testing.T) {
		got, err := newRepositoryPlan(match).LogicalChanges()
		if err != nil || len(got) != 0 {
			t.Errorf("LogicalChanges() = %+v, error = %v, want none without error", got, err)
		}
	})

	t.Run("applicable mismatch becomes a logical change relevant to its own observed value", func(t *testing.T) {
		got, err := newRepositoryPlan(match, mismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{Operations: []UpdateOperation{{
			Key: setting.AllowAutoMerge, Desired: setting.BoolValue(true),
			RelevantSettings: []RelevantSetting{relevant(setting.AllowAutoMerge, github.Observed(setting.BoolValue(false)))},
		}}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("applicable mismatches of both settings become logical changes in the plan order", func(t *testing.T) {
		deleteBranch := SettingPlan{Key: setting.DeleteBranchOnMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
		got, err := newRepositoryPlan(deleteBranch, mismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			{Operations: []UpdateOperation{{
				Key: setting.DeleteBranchOnMerge, Desired: setting.BoolValue(true),
				RelevantSettings: []RelevantSetting{relevant(setting.DeleteBranchOnMerge, github.Observed(setting.BoolValue(false)))},
			}}},
			{Operations: []UpdateOperation{{
				Key: setting.AllowAutoMerge, Desired: setting.BoolValue(true),
				RelevantSettings: []RelevantSetting{relevant(setting.AllowAutoMerge, github.Observed(setting.BoolValue(false)))},
			}}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	enabling := SettingPlan{Key: setting.ActionsEnabled, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}

	t.Run("applicable change of the enabled state becomes a logical change relevant only to its own observed value", func(t *testing.T) {
		got, err := newRepositoryPlan(enabling).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{Operations: []UpdateOperation{{
			Key: setting.ActionsEnabled, Desired: setting.BoolValue(true),
			RelevantSettings: []RelevantSetting{relevant(setting.ActionsEnabled, github.Observed(setting.BoolValue(false)))},
		}}}}
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
		got, err := newRepositoryPlan(policyChange(setting.AllowedActionsAll, setting.AllowedActionsLocalOnly)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{Operations: []UpdateOperation{{
			Key: setting.AllowedActions, Desired: policy(setting.AllowedActionsLocalOnly),
			RelevantSettings: []RelevantSetting{relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsAll)))},
		}}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("mismatch with a side effect is also relevant to the lost value shown before approval", func(t *testing.T) {
		lost := SideEffect{Key: setting.SelectedActions, Observed: github.Observed(criteria)}
		got, err := newRepositoryPlan(policyChange(setting.AllowedActionsSelected, setting.AllowedActionsAll, lost)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{Operations: []UpdateOperation{{
			Key: setting.AllowedActions, Desired: policy(setting.AllowedActionsAll),
			RelevantSettings: []RelevantSetting{
				relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsSelected))),
				relevant(setting.SelectedActions, github.Observed(criteria)),
			},
		}}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("policy not comparable because actions is disabled is relevant to the unavailable values", func(t *testing.T) {
		enable := SettingPlan{
			Key: setting.AllowedActions, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly), Result: NotComparable, Applicability: Applicable,
			SideEffects: []SideEffect{{Key: setting.SelectedActions, Observed: disabled}},
		}
		got, err := newRepositoryPlan(enable).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{Operations: []UpdateOperation{{
			Key: setting.AllowedActions, Desired: policy(setting.AllowedActionsLocalOnly),
			RelevantSettings: []RelevantSetting{relevant(setting.AllowedActions, disabled), relevant(setting.SelectedActions, disabled)},
		}}}}
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
	// toSelected は、ポリシーを selected にする更新操作のあとに続く、選択方式の Actions 許可条件の更新操作。
	// 基準は1つ目による予定どおりの変化を反映したポリシーだけで、2つ目がすべて上書きする3項目は並ばない。
	toSelected := UpdateOperation{
		Key: setting.SelectedActions, Desired: criteria,
		RelevantSettings: []RelevantSetting{relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsSelected)))},
	}
	workflow := SettingPlan{
		Key: setting.DefaultWorkflowPermissions, Observed: github.Observed(setting.PermissionLevelValue(setting.PermissionWrite)),
		Desired: setting.PermissionLevelValue(setting.PermissionRead), Result: Mismatch, Applicability: Applicable,
	}

	t.Run("policy to selected and the criteria not comparable become one logical change of two updates at the policy position", func(t *testing.T) {
		got, err := newRepositoryPlan(mismatch, policyChange(setting.AllowedActionsAll, setting.AllowedActionsSelected), notComparableCriteria(github.UnavailableNotSelected), workflow).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			{Operations: []UpdateOperation{{Key: setting.AllowAutoMerge, Desired: setting.BoolValue(true), RelevantSettings: []RelevantSetting{relevant(setting.AllowAutoMerge, github.Observed(setting.BoolValue(false)))}}}},
			{Operations: []UpdateOperation{
				{
					Key: setting.AllowedActions, Desired: policy(setting.AllowedActionsSelected),
					RelevantSettings: []RelevantSetting{relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsAll)))},
				},
				toSelected,
			}},
			{Operations: []UpdateOperation{{Key: workflow.Key, Desired: workflow.Desired, RelevantSettings: []RelevantSetting{relevant(workflow.Key, workflow.Observed)}}}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("policy enabling actions with selected and the criteria not comparable become one logical change of two updates", func(t *testing.T) {
		enableSelected := SettingPlan{Key: setting.AllowedActions, Observed: disabled, Desired: policy(setting.AllowedActionsSelected), Result: NotComparable, Applicability: Applicable}
		got, err := newRepositoryPlan(enableSelected, notComparableCriteria(github.UnavailableActionsDisabled)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{Operations: []UpdateOperation{
			{Key: setting.AllowedActions, Desired: policy(setting.AllowedActionsSelected), RelevantSettings: []RelevantSetting{relevant(setting.AllowedActions, disabled)}},
			toSelected,
		}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	// 設定宣言は selected と3項目を組で求め、有効状態とポリシーを同時に管理設定にせず、
	// 許可条件はポリシーが selected でなければ観測不能になるため、Generate からは到達しない組
	panicCases := []struct {
		name     string
		settings []SettingPlan
	}{
		{
			name:     "policy to selected without the criteria panics",
			settings: []SettingPlan{policyChange(setting.AllowedActionsAll, setting.AllowedActionsSelected)},
		},
		{
			name:     "criteria not comparable without the policy to selected panics",
			settings: []SettingPlan{notComparableCriteria(github.UnavailableNotSelected)},
		},
		{
			name: "policy already selected with the criteria not comparable panics",
			settings: []SettingPlan{
				{Key: setting.AllowedActions, Observed: github.Observed(policy(setting.AllowedActionsSelected)), Desired: policy(setting.AllowedActionsSelected), Result: Match},
				notComparableCriteria(github.UnavailableNotSelected),
			},
		},
		{
			name: "policy to selected with the criteria mismatch panics",
			settings: []SettingPlan{
				policyChange(setting.AllowedActionsAll, setting.AllowedActionsSelected),
				{Key: setting.SelectedActions, Observed: github.Observed(criteria), Desired: criteria, Result: Mismatch, Applicability: Applicable},
			},
		},
		{
			name:     "enabled state with a policy panics",
			settings: []SettingPlan{enabling, {Key: setting.AllowedActions, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly), Result: NotComparable, Applicability: Applicable}},
		},
	}
	for _, tc := range panicCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("LogicalChanges() did not panic")
				}
			}()
			_, _ = newRepositoryPlan(tc.settings...).LogicalChanges()
		})
	}

	t.Run("not-applicable criteria paired with an applicable policy to selected is an error", func(t *testing.T) {
		// 対の1つ目が適用可能でも、2つ目の適用可能性も確かめる
		notApplicable := notComparableCriteria(github.UnavailableNotSelected)
		notApplicable.Applicability = NotApplicable
		if got, err := newRepositoryPlan(mismatch, policyChange(setting.AllowedActionsAll, setting.AllowedActionsSelected), notApplicable).LogicalChanges(); err == nil {
			t.Errorf("LogicalChanges() = %+v, error = nil, want an error", got)
		}
	})

	errorCases := []struct {
		name    string
		setting SettingPlan
	}{
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
			if got, err := newRepositoryPlan(mismatch, tc.setting).LogicalChanges(); err == nil {
				t.Errorf("LogicalChanges() = %+v, error = nil, want an error", got)
			}
		})
	}

	shaPinningMismatch := SettingPlan{
		Key: setting.SHAPinningRequired, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable,
	}
	actionsEnabled := relevant(setting.ActionsEnabled, github.Observed(setting.BoolValue(true)))
	shaPinningOperation := UpdateOperation{
		Key: setting.SHAPinningRequired, Desired: setting.BoolValue(true),
		RelevantSettings: []RelevantSetting{relevant(setting.SHAPinningRequired, github.Observed(setting.BoolValue(false))), actionsEnabled},
	}

	t.Run("sha_pinning_required mismatch is relevant to its observed value and then to actions being enabled", func(t *testing.T) {
		got, err := newRepositoryPlan(shaPinningMismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		if want := []LogicalChange{{Operations: []UpdateOperation{shaPinningOperation}}}; !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("sha_pinning_required not comparable after enabling is a separate logical change relevant only to actions being enabled", func(t *testing.T) {
		shaPinningNotComparable := SettingPlan{Key: setting.SHAPinningRequired, Observed: disabled, Desired: setting.BoolValue(true), Result: NotComparable, Applicability: Applicable}
		got, err := newRepositoryPlan(enabling, shaPinningNotComparable).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			{Operations: []UpdateOperation{{
				Key: setting.ActionsEnabled, Desired: setting.BoolValue(true),
				RelevantSettings: []RelevantSetting{relevant(setting.ActionsEnabled, github.Observed(setting.BoolValue(false)))},
			}}},
			{Operations: []UpdateOperation{{Key: setting.SHAPinningRequired, Desired: setting.BoolValue(true), RelevantSettings: []RelevantSetting{actionsEnabled}}}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("policy change and sha_pinning_required mismatch are separate logical changes in the plan order", func(t *testing.T) {
		got, err := newRepositoryPlan(policyChange(setting.AllowedActionsAll, setting.AllowedActionsLocalOnly), shaPinningMismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		if len(got) != 2 || !slices.Equal(got[0].Keys(), []setting.Key{setting.AllowedActions}) || !reflect.DeepEqual(got[1], LogicalChange{Operations: []UpdateOperation{shaPinningOperation}}) {
			t.Errorf("LogicalChanges() = %+v, want the policy change and then %+v", got, shaPinningOperation)
		}
	})
}
