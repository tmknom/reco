package changeplan

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

func TestLogicalChange_Units(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	existence := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	policy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}
	protected := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches})
	cases := []struct {
		name       string
		operations []UpdateOperation
		want       []setting.Unit
	}{
		{
			name:       "two updates return their units in the update order",
			operations: []UpdateOperation{{Unit: setting.Unit{Key: setting.AllowedActions}}, {Unit: setting.Unit{Key: setting.SelectedActions}}},
			want:       []setting.Unit{{Key: setting.AllowedActions}, {Key: setting.SelectedActions}},
		},
		{
			name:       "creation without a mode changes only the environment",
			operations: []UpdateOperation{{Unit: existence}},
			want:       []setting.Unit{existence},
		},
		{
			name:       "creation sending a mode changes the environment and then its mode",
			operations: []UpdateOperation{{Unit: existence, Desired: protected}},
			want:       []setting.Unit{existence, policy},
		},
		{
			name:       "mode change of an existing environment changes only the mode",
			operations: []UpdateOperation{{Unit: policy, Desired: protected}},
			want:       []setting.Unit{policy},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (LogicalChange{Operations: tc.operations}).Units(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Units() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestUpdateOperation_TargetsEnvironment(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	cases := []struct {
		name string
		unit setting.Unit
		want bool
	}{
		{name: "creation targets an environment", unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, want: true},
		{name: "mode change targets an environment", unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, want: true},
		{name: "repository setting does not target an environment", unit: setting.Unit{Key: setting.AllowAutoMerge}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (UpdateOperation{Unit: tc.unit}).TargetsEnvironment(); got != tc.want {
				t.Errorf("TargetsEnvironment() = %t, want %t", got, tc.want)
			}
		})
	}
}

// TestUpdateOperation_EnvironmentRequestShape は、Environment の更新操作が作成か、deployment_branch_policy を送るかの判定を確かめる。
// 方式を宣言していない作成だけが本文を送らない。
//
//trace:checks INV-001
func TestUpdateOperation_EnvironmentRequestShape(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	protected := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches})
	cases := []struct {
		name        string
		operation   UpdateOperation
		wantCreates bool
		wantSets    bool
	}{
		{
			name:        "creation without a mode creates and does not set the mode",
			operation:   UpdateOperation{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}},
			wantCreates: true, wantSets: false,
		},
		{
			name:        "creation with a mode creates and sets the mode",
			operation:   UpdateOperation{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Desired: protected},
			wantCreates: true, wantSets: true,
		},
		{
			name:        "mode change sets the mode without creating",
			operation:   UpdateOperation{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Desired: protected},
			wantCreates: false, wantSets: true,
		},
		{
			name:        "repository setting neither creates nor sets the mode",
			operation:   UpdateOperation{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Desired: setting.BoolValue(true)},
			wantCreates: false, wantSets: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.operation.CreatesEnvironment(); got != tc.wantCreates {
				t.Errorf("CreatesEnvironment() = %t, want %t", got, tc.wantCreates)
			}
			if got := tc.operation.SetsDeploymentBranchPolicy(); got != tc.wantSets {
				t.Errorf("SetsDeploymentBranchPolicy() = %t, want %t", got, tc.wantSets)
			}
		})
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
		want []setting.Unit
	}{
		{
			name: "policy change also changes the enabled state",
			keys: []setting.Key{setting.AllowedActions},
			want: []setting.Unit{{Key: setting.AllowedActions}, {Key: setting.ActionsEnabled}},
		},
		{
			name: "enabled state change changes only the enabled state",
			keys: []setting.Key{setting.ActionsEnabled},
			want: []setting.Unit{{Key: setting.ActionsEnabled}},
		},
		{
			name: "criteria change does not change the enabled state",
			keys: []setting.Key{setting.SelectedActions},
			want: []setting.Unit{{Key: setting.SelectedActions}},
		},
		{
			name: "sha_pinning_required change does not change the enabled state",
			keys: []setting.Key{setting.SHAPinningRequired},
			want: []setting.Unit{{Key: setting.SHAPinningRequired}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var change LogicalChange
			for _, key := range tc.keys {
				change.Operations = append(change.Operations, UpdateOperation{Unit: setting.Unit{Key: key}})
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
		return RelevantSetting{Unit: setting.Unit{Key: key}, ValueAtApproval: atApproval}
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
			Unit: setting.Unit{Key: setting.AllowAutoMerge}, Desired: setting.BoolValue(true),
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
				Unit: setting.Unit{Key: setting.DeleteBranchOnMerge}, Desired: setting.BoolValue(true),
				RelevantSettings: []RelevantSetting{relevant(setting.DeleteBranchOnMerge, github.Observed(setting.BoolValue(false)))},
			}}},
			{Operations: []UpdateOperation{{
				Unit: setting.Unit{Key: setting.AllowAutoMerge}, Desired: setting.BoolValue(true),
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
			Unit: setting.Unit{Key: setting.ActionsEnabled}, Desired: setting.BoolValue(true),
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
			Unit: setting.Unit{Key: setting.AllowedActions}, Desired: policy(setting.AllowedActionsLocalOnly),
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
			Unit: setting.Unit{Key: setting.AllowedActions}, Desired: policy(setting.AllowedActionsAll),
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
			Unit: setting.Unit{Key: setting.AllowedActions}, Desired: policy(setting.AllowedActionsLocalOnly),
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
		Unit: setting.Unit{Key: setting.SelectedActions}, Desired: criteria,
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
			{Operations: []UpdateOperation{{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Desired: setting.BoolValue(true), RelevantSettings: []RelevantSetting{relevant(setting.AllowAutoMerge, github.Observed(setting.BoolValue(false)))}}}},
			{Operations: []UpdateOperation{
				{
					Unit: setting.Unit{Key: setting.AllowedActions}, Desired: policy(setting.AllowedActionsSelected),
					RelevantSettings: []RelevantSetting{relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsAll)))},
				},
				toSelected,
			}},
			{Operations: []UpdateOperation{{Unit: setting.Unit{Key: workflow.Key}, Desired: workflow.Desired, RelevantSettings: []RelevantSetting{relevant(workflow.Key, workflow.Observed)}}}},
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
			{Unit: setting.Unit{Key: setting.AllowedActions}, Desired: policy(setting.AllowedActionsSelected), RelevantSettings: []RelevantSetting{relevant(setting.AllowedActions, disabled)}},
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
		Unit: setting.Unit{Key: setting.SHAPinningRequired}, Desired: setting.BoolValue(true),
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
				Unit: setting.Unit{Key: setting.ActionsEnabled}, Desired: setting.BoolValue(true),
				RelevantSettings: []RelevantSetting{relevant(setting.ActionsEnabled, github.Observed(setting.BoolValue(false)))},
			}}},
			{Operations: []UpdateOperation{{Unit: setting.Unit{Key: setting.SHAPinningRequired}, Desired: setting.BoolValue(true), RelevantSettings: []RelevantSetting{actionsEnabled}}}},
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
		if len(got) != 2 || !slices.Equal(got[0].Units(), []setting.Unit{{Key: setting.AllowedActions}}) || !reflect.DeepEqual(got[1], LogicalChange{Operations: []UpdateOperation{shaPinningOperation}}) {
			t.Errorf("LogicalChanges() = %+v, want the policy change and then %+v", got, shaPinningOperation)
		}
	})
}

// TestRepositoryPlan_LogicalChangesOfEnvironments は、Environment の変更予定から導く更新操作の形と、その変更前提の基準を確かめる。
//
//trace:checks INV-001
//trace:checks INV-002
//trace:checks INV-003
//trace:checks INV-004
//trace:checks FR-010
//trace:checks FR-011
func TestRepositoryPlan_LogicalChangesOfEnvironments(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	staging := mustEnvironmentName(t, "staging")
	mismatch := SettingPlan{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
	all := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll})
	protected := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches})
	absent := lookup(t, "production")
	existingStaging := lookup(t, "staging", github.Environment{ID: 7, Name: "Staging", DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}})
	creation := SettingPlan{Key: setting.EnvironmentResource, Environment: production, ObservedEnvironment: absent, Result: Mismatch, Applicability: Applicable}
	creationMode := SettingPlan{
		Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
		Desired: protected, Result: NotComparable, Applicability: Applicable,
	}
	existing := SettingPlan{Key: setting.EnvironmentResource, Environment: staging, ObservedEnvironment: existingStaging, Result: Match}
	modeChange := SettingPlan{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging, Observed: github.Observed(all), Desired: protected, Result: Mismatch, Applicability: Applicable}
	newRepositoryPlan := func(settings ...SettingPlan) RepositoryPlan {
		return RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}
	productionUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	stagingUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: staging}
	stagingPolicyUnit := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}
	allowAutoMerge := LogicalChange{Operations: []UpdateOperation{{
		Unit: setting.Unit{Key: setting.AllowAutoMerge}, Desired: setting.BoolValue(true),
		RelevantSettings: []RelevantSetting{{Unit: setting.Unit{Key: setting.AllowAutoMerge}, ValueAtApproval: github.Observed(setting.BoolValue(false))}},
	}}}
	// 作成の基準は存在しないことだけで、方式を宣言していても方式の基準は並ばない
	createProduction := func(desired setting.Value) LogicalChange {
		return LogicalChange{Operations: []UpdateOperation{{
			Unit: productionUnit, Desired: desired,
			RelevantSettings: []RelevantSetting{{Unit: productionUnit, EnvironmentAtApproval: absent}},
		}}}
	}
	// 方式の変更の基準は、Environment が存在し ID が同じこと、方式が計画時の観測値と同じことの順に並ぶ
	changeStagingMode := LogicalChange{Operations: []UpdateOperation{{
		Unit: stagingPolicyUnit, Desired: protected,
		RelevantSettings: []RelevantSetting{
			{Unit: stagingUnit, EnvironmentAtApproval: existingStaging},
			{Unit: stagingPolicyUnit, ValueAtApproval: github.Observed(all)},
		},
	}}}

	cases := []struct {
		name     string
		settings []SettingPlan
		want     []LogicalChange
	}{
		{
			name:     "creation without a mode is one update sending no mode and relevant only to the absence",
			settings: []SettingPlan{creation},
			want:     []LogicalChange{createProduction(setting.Value{})},
		},
		{
			name:     "creation with a mode is one logical change of one update sending the mode at the creation position",
			settings: []SettingPlan{creation, creationMode},
			want:     []LogicalChange{createProduction(protected)},
		},
		{
			name:     "creation paired with the mode of the name differing only in case sends the mode",
			settings: []SettingPlan{creation, func() SettingPlan { s := creationMode; s.Environment = mustEnvironmentName(t, "Production"); return s }()},
			want:     []LogicalChange{createProduction(protected)},
		},
		{
			name:     "mode change of an existing environment is relevant to its identity and then to the observed mode",
			settings: []SettingPlan{existing, modeChange},
			want:     []LogicalChange{changeStagingMode},
		},
		{
			name:     "environment changes follow the other changes in the plan order",
			settings: []SettingPlan{mismatch, creation, creationMode, existing, modeChange},
			want:     []LogicalChange{allowAutoMerge, createProduction(protected), changeStagingMode},
		},
		{
			name:     "matched environment does not prevent the other logical changes",
			settings: []SettingPlan{mismatch, existing},
			want:     []LogicalChange{allowAutoMerge},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newRepositoryPlan(tc.settings...).LogicalChanges()
			if err != nil {
				t.Fatalf("LogicalChanges() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("LogicalChanges() =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}

	// environmentPlans と judgeApplicability が作らない組
	panicCases := []struct {
		name     string
		settings []SettingPlan
	}{
		{
			name:     "mode of an absent environment without its creation panics",
			settings: []SettingPlan{creationMode},
		},
		{
			name:     "mode of an absent environment following a matched existence panics",
			settings: []SettingPlan{{Key: setting.EnvironmentResource, Environment: production, ObservedEnvironment: lookup(t, "production", github.Environment{ID: 1, Name: "production"}), Result: Match}, creationMode},
		},
		{
			name:     "mode mismatch without the existence panics",
			settings: []SettingPlan{modeChange},
		},
		{
			name:     "creation followed by a mode mismatch of the same name panics",
			settings: []SettingPlan{creation, {Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Observed(all), Desired: protected, Result: Mismatch, Applicability: Applicable}},
		},
		{
			name:     "creation followed by a matched mode of the same name panics",
			settings: []SettingPlan{creation, {Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Observed(protected), Desired: protected, Result: Match}},
		},
		{
			name: "applicable not comparable existence panics",
			settings: []SettingPlan{{
				Key: setting.EnvironmentResource, Environment: production, ObservedEnvironment: lookup(t, "production", github.Environment{ID: 1, Name: "prod\u00fcction"}),
				Result: NotComparable, Applicability: Applicable,
			}},
		},
		{
			name: "applicable mode not comparable for another reason panics",
			settings: []SettingPlan{creation, {
				Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentNameUndetermined),
				Desired: protected, Result: NotComparable, Applicability: Applicable,
			}},
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

	selectedMain := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{
		Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "main"}},
	})
	selectedRelease := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{
		Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "release"}},
	})
	selectedPolicyPlan := func(environment setting.EnvironmentName, observed, desired setting.Value, result ReconciliationResult) SettingPlan {
		return SettingPlan{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment, Observed: github.Observed(observed), Desired: desired, Result: result, Applicability: Applicable}
	}
	rejectCases := []struct {
		name     string
		settings []SettingPlan
		want     string
	}{
		{
			name:     "mode change from all to selected is rejected naming the deployment branch policy unit",
			settings: []SettingPlan{existing, selectedPolicyPlan(staging, all, selectedMain, Mismatch)},
			want:     "owner/repo: environments.staging.deployment_branch_policy: applying mode selected is not supported",
		},
		{
			name:     "pattern-only difference within selected is rejected",
			settings: []SettingPlan{existing, selectedPolicyPlan(staging, selectedMain, selectedRelease, Mismatch)},
			want:     "owner/repo: environments.staging.deployment_branch_policy: applying mode selected is not supported",
		},
		{
			name: "creation with a not comparable selected mode is rejected naming the deployment branch policy unit",
			settings: []SettingPlan{creation, {
				Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
				Desired: selectedMain, Result: NotComparable, Applicability: Applicable,
			}},
			want: "owner/repo: environments.production.deployment_branch_policy: applying mode selected is not supported",
		},
		{
			name: "the first of several selected changes in the plan order is named",
			settings: []SettingPlan{existing, selectedPolicyPlan(staging, all, selectedMain, Mismatch), creation, {
				Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
				Desired: selectedMain, Result: NotComparable, Applicability: Applicable,
			}},
			want: "owner/repo: environments.staging.deployment_branch_policy: applying mode selected is not supported",
		},
		{
			name:     "not-applicable change is reported before the selected rejection",
			settings: []SettingPlan{selectedPolicyPlan(staging, all, selectedMain, Mismatch), {Key: setting.EnvironmentResource, Environment: production, Result: NotComparable, Applicability: NotApplicable}},
			want:     "owner/repo: environments.production cannot be applied",
		},
	}
	for _, tc := range rejectCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newRepositoryPlan(tc.settings...).LogicalChanges()
			if err == nil || err.Error() != tc.want {
				t.Errorf("LogicalChanges() error = %v, want %q", err, tc.want)
			}
		})
	}

	t.Run("matched selected does not prevent the other logical changes", func(t *testing.T) {
		matched := selectedPolicyPlan(staging, selectedMain, selectedMain, Match)
		got, err := newRepositoryPlan(mismatch, existing, matched).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		if want := []LogicalChange{allowAutoMerge}; !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() =\n%+v\nwant\n%+v", got, want)
		}
	})

	t.Run("change from selected to all stays one update relevant to the observed patterns", func(t *testing.T) {
		got, err := newRepositoryPlan(existing, selectedPolicyPlan(staging, selectedMain, all, Mismatch)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{Operations: []UpdateOperation{{
			Unit: stagingPolicyUnit, Desired: all,
			RelevantSettings: []RelevantSetting{
				{Unit: stagingUnit, EnvironmentAtApproval: existingStaging},
				{Unit: stagingPolicyUnit, ValueAtApproval: github.Observed(selectedMain)},
			},
		}}}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() =\n%+v\nwant\n%+v", got, want)
		}
	})

	t.Run("not-applicable environment is rejected naming the environment", func(t *testing.T) {
		undetermined := SettingPlan{Key: setting.EnvironmentResource, Environment: production, Result: NotComparable, Applicability: NotApplicable}
		_, err := newRepositoryPlan(undetermined).LogicalChanges()
		if want := "owner/repo: environments.production cannot be applied"; err == nil || err.Error() != want {
			t.Errorf("LogicalChanges() error = %v, want %q", err, want)
		}
	})
}
