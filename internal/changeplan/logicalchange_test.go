package changeplan

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

func TestUpdateOperation_CreatesEnvironment(t *testing.T) {
	cases := []struct {
		name string
		kind UpdateOperationKind
		want bool
	}{
		{name: "creation without a mode is a creation", kind: EnvironmentCreation, want: true},
		{name: "creation with a mode is a creation", kind: EnvironmentCreationWithMode, want: true},
		// 方式を送る作成と同じ形の要求を送り、取り違えやすいため、作成でない種類の代表にする
		{name: "mode update of an existing environment is not a creation", kind: DeploymentBranchPolicyModeUpdate, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (UpdateOperation{Kind: tc.kind}).CreatesEnvironment(); got != tc.want {
				t.Errorf("CreatesEnvironment() = %t, want %t", got, tc.want)
			}
		})
	}
}

// TestLogicalChange_SettingsToChange は、変更対象外の設定から除く、論理的な設定変更が変える設定の範囲を確かめる。
//
//trace:checks INV-001 FR-012
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
				change.DesiredState = append(change.DesiredState, DesiredSetting{Unit: setting.Unit{Key: key}})
			}
			if got := change.SettingsToChange(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("SettingsToChange() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks INV-001 INV-002 INV-004 FR-008 FR-011
func TestRepositoryPlan_LogicalChanges(t *testing.T) {
	match := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match}
	mismatch := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
	newRepositoryPlan := func(settings ...SettingPlan) RepositoryPlan {
		return RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}
	relevant := func(key setting.Key, anticipated github.SettingObservation[setting.Value]) RelevantSetting {
		return RelevantSetting{Unit: setting.Unit{Key: key}, AnticipatedValue: anticipated}
	}
	// settingUpdate は、key の値 desired を送る更新操作。
	settingUpdate := func(key setting.Key, desired setting.Value, relevant ...RelevantSetting) UpdateOperation {
		return UpdateOperation{Kind: SettingUpdate, Unit: setting.Unit{Key: key}, Value: desired, RelevantSettings: relevant}
	}
	// settingChange は、key を desired にする期待状態と、その値を送る更新操作一つからなる論理的な設定変更。
	settingChange := func(key setting.Key, desired setting.Value, relevant ...RelevantSetting) LogicalChange {
		return LogicalChange{DesiredState: []DesiredSetting{{Unit: setting.Unit{Key: key}, Value: desired}}, Operations: []UpdateOperation{settingUpdate(key, desired, relevant...)}}
	}

	t.Run("merge method changes are one logical change at the first one and allow_update_branch is another", func(t *testing.T) {
		squash := SettingPlan{Unit: setting.Unit{Key: setting.AllowSquashMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
		rebase := SettingPlan{Unit: setting.Unit{Key: setting.AllowRebaseMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false), Result: Mismatch, Applicability: Applicable}
		updateBranch := SettingPlan{Unit: setting.Unit{Key: setting.AllowUpdateBranch}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
		got, err := newRepositoryPlan(mismatch, squash, rebase, updateBranch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			settingChange(setting.AllowAutoMerge, setting.BoolValue(true), relevant(setting.AllowAutoMerge, github.Observed(setting.BoolValue(false)))),
			mergeMethodsChange([]SettingPlan{squash, rebase}),
			settingChange(setting.AllowUpdateBranch, setting.BoolValue(true), relevant(setting.AllowUpdateBranch, github.Observed(setting.BoolValue(false)))),
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() =\n%+v\nwant\n%+v", got, want)
		}
	})

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
		want := []LogicalChange{settingChange(setting.AllowAutoMerge, setting.BoolValue(true), relevant(setting.AllowAutoMerge, github.Observed(setting.BoolValue(false))))}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("applicable mismatches of both settings become logical changes in the plan order", func(t *testing.T) {
		deleteBranch := SettingPlan{Unit: setting.Unit{Key: setting.DeleteBranchOnMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
		got, err := newRepositoryPlan(deleteBranch, mismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			settingChange(setting.DeleteBranchOnMerge, setting.BoolValue(true), relevant(setting.DeleteBranchOnMerge, github.Observed(setting.BoolValue(false)))),
			settingChange(setting.AllowAutoMerge, setting.BoolValue(true), relevant(setting.AllowAutoMerge, github.Observed(setting.BoolValue(false)))),
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	enabling := SettingPlan{Unit: setting.Unit{Key: setting.ActionsEnabled}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}

	t.Run("applicable change of the enabled state becomes a logical change relevant only to its own observed value", func(t *testing.T) {
		got, err := newRepositoryPlan(enabling).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{settingChange(setting.ActionsEnabled, setting.BoolValue(true), relevant(setting.ActionsEnabled, github.Observed(setting.BoolValue(false))))}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	criteria := setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"octo-org/*"}})
	disabled := github.Unavailable[setting.Value](github.UnavailableActionsDisabled)
	policy := func(p setting.AllowedActionsPolicy) setting.Value { return setting.AllowedActionsPolicyValue(p) }
	policyChange := func(observed, desired setting.AllowedActionsPolicy, effects ...SideEffect) SettingPlan {
		return SettingPlan{
			Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Observed(policy(observed)), Desired: policy(desired),
			Result: Mismatch, Applicability: Applicable, SideEffects: effects,
		}
	}

	t.Run("applicable mismatch without side effects becomes a logical change", func(t *testing.T) {
		got, err := newRepositoryPlan(policyChange(setting.AllowedActionsAll, setting.AllowedActionsLocalOnly)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{settingChange(setting.AllowedActions, policy(setting.AllowedActionsLocalOnly), relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsAll))))}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("mismatch with a side effect is also relevant to the lost value shown before approval", func(t *testing.T) {
		lost := SideEffect{Kind: SideEffectSelectedActionsDiscarded, Observed: github.Observed(criteria)}
		got, err := newRepositoryPlan(policyChange(setting.AllowedActionsSelected, setting.AllowedActionsAll, lost)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{settingChange(
			setting.AllowedActions, policy(setting.AllowedActionsAll),
			relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsSelected))),
			relevant(setting.SelectedActions, github.Observed(criteria)),
		)}
		want[0].SideEffectSettings = []setting.Unit{{Key: setting.SelectedActions}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("policy not comparable because actions is disabled is relevant to the unavailable values", func(t *testing.T) {
		enable := SettingPlan{
			Unit: setting.Unit{Key: setting.AllowedActions}, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly), Result: NotComparable, Applicability: Applicable,
			SideEffects: []SideEffect{{Kind: SideEffectSelectedActionsDiscarded, Observed: disabled}},
		}
		got, err := newRepositoryPlan(enable).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{settingChange(setting.AllowedActions, policy(setting.AllowedActionsLocalOnly), relevant(setting.AllowedActions, disabled), relevant(setting.SelectedActions, disabled))}
		want[0].SideEffectSettings = []setting.Unit{{Key: setting.SelectedActions}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	notComparableCriteria := func(reason github.UnavailableReason) SettingPlan {
		return SettingPlan{
			Unit: setting.Unit{Key: setting.SelectedActions}, Observed: github.Unavailable[setting.Value](reason),
			Desired: criteria, Result: NotComparable, Applicability: Applicable,
		}
	}
	// toSelected は、ポリシーを selected にする更新操作のあとに続く、選択方式の Actions 許可条件の更新操作。
	// 基準は1つ目による予定どおりの変化を反映したポリシーだけで、2つ目がすべて上書きする3項目は並ばない。
	toSelected := settingUpdate(setting.SelectedActions, criteria, relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsSelected))))
	// toSelectedDesired は、ポリシーと許可条件をまとめた論理的な設定変更の期待状態で、ポリシー、許可条件の順に並ぶ。
	toSelectedDesired := []DesiredSetting{{Unit: setting.Unit{Key: setting.AllowedActions}, Value: policy(setting.AllowedActionsSelected)}, {Unit: setting.Unit{Key: setting.SelectedActions}, Value: criteria}}
	workflow := SettingPlan{
		Unit: setting.Unit{Key: setting.DefaultWorkflowPermissions}, Observed: github.Observed(setting.PermissionLevelValue(setting.PermissionWrite)),
		Desired: setting.PermissionLevelValue(setting.PermissionRead), Result: Mismatch, Applicability: Applicable,
	}

	t.Run("policy to selected and the criteria not comparable become one logical change of two updates at the policy position", func(t *testing.T) {
		got, err := newRepositoryPlan(mismatch, policyChange(setting.AllowedActionsAll, setting.AllowedActionsSelected), notComparableCriteria(github.UnavailableNotSelected), workflow).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			settingChange(setting.AllowAutoMerge, setting.BoolValue(true), relevant(setting.AllowAutoMerge, github.Observed(setting.BoolValue(false)))),
			{DesiredState: toSelectedDesired, Operations: []UpdateOperation{
				settingUpdate(setting.AllowedActions, policy(setting.AllowedActionsSelected), relevant(setting.AllowedActions, github.Observed(policy(setting.AllowedActionsAll)))),
				toSelected,
			}},
			settingChange(workflow.Unit.Key, workflow.Desired, relevant(workflow.Unit.Key, workflow.Observed)),
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("policy enabling actions with selected and the criteria not comparable become one logical change of two updates", func(t *testing.T) {
		enableSelected := SettingPlan{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: disabled, Desired: policy(setting.AllowedActionsSelected), Result: NotComparable, Applicability: Applicable}
		got, err := newRepositoryPlan(enableSelected, notComparableCriteria(github.UnavailableActionsDisabled)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{DesiredState: toSelectedDesired, Operations: []UpdateOperation{
			settingUpdate(setting.AllowedActions, policy(setting.AllowedActionsSelected), relevant(setting.AllowedActions, disabled)),
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
				{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Observed(policy(setting.AllowedActionsSelected)), Desired: policy(setting.AllowedActionsSelected), Result: Match},
				notComparableCriteria(github.UnavailableNotSelected),
			},
		},
		{
			name: "policy to selected with the criteria mismatch panics",
			settings: []SettingPlan{
				policyChange(setting.AllowedActionsAll, setting.AllowedActionsSelected),
				{Unit: setting.Unit{Key: setting.SelectedActions}, Observed: github.Observed(criteria), Desired: criteria, Result: Mismatch, Applicability: Applicable},
			},
		},
		{
			name:     "enabled state with a policy panics",
			settings: []SettingPlan{enabling, {Unit: setting.Unit{Key: setting.AllowedActions}, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly), Result: NotComparable, Applicability: Applicable}},
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
			setting: SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: NotApplicable},
		},
		{
			name:    "mismatch without an applicability judgment is an error",
			setting: SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: ApplicabilityNotJudged},
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
		Unit: setting.Unit{Key: setting.SHAPinningRequired}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable,
	}
	actionsEnabled := relevant(setting.ActionsEnabled, github.Observed(setting.BoolValue(true)))
	shaPinningChange := settingChange(setting.SHAPinningRequired, setting.BoolValue(true), relevant(setting.SHAPinningRequired, github.Observed(setting.BoolValue(false))), actionsEnabled)

	t.Run("sha_pinning_required mismatch is relevant to its observed value and then to actions being enabled", func(t *testing.T) {
		got, err := newRepositoryPlan(shaPinningMismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		if want := []LogicalChange{shaPinningChange}; !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("sha_pinning_required not comparable after enabling is a separate logical change relevant only to actions being enabled", func(t *testing.T) {
		shaPinningNotComparable := SettingPlan{Unit: setting.Unit{Key: setting.SHAPinningRequired}, Observed: disabled, Desired: setting.BoolValue(true), Result: NotComparable, Applicability: Applicable}
		got, err := newRepositoryPlan(enabling, shaPinningNotComparable).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			settingChange(setting.ActionsEnabled, setting.BoolValue(true), relevant(setting.ActionsEnabled, github.Observed(setting.BoolValue(false)))),
			settingChange(setting.SHAPinningRequired, setting.BoolValue(true), actionsEnabled),
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
		if len(got) != 2 || !slices.Equal(got[0].Units(), []setting.Unit{{Key: setting.AllowedActions}}) || !reflect.DeepEqual(got[1], shaPinningChange) {
			t.Errorf("LogicalChanges() = %+v, want the policy change and then %+v", got, shaPinningChange)
		}
	})
}

// TestRepositoryPlan_LogicalChangesOfRepositoryVariables は、変数の変更予定が、変数ごとに一つの更新操作からなる別々の論理的な設定変更になり、計画の並びで実行されることを確かめる。
//
//trace:checks INV-005 FR-011
func TestRepositoryPlan_LogicalChangesOfRepositoryVariables(t *testing.T) {
	mismatch := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
	creation := SettingPlan{Unit: variableUnit(t, "ZETA"), Observed: github.Observed(setting.Absent()), Desired: setting.VariableValue("z"), Result: Mismatch, Applicability: Applicable}
	valueChange := SettingPlan{Unit: variableUnit(t, "ALPHA"), Observed: github.Observed(setting.VariableValue("1")), Desired: setting.VariableValue("2"), Result: Mismatch, Applicability: Applicable}
	// 値が一致した変数は、変更予定がないため論理的な設定変更にならない
	matched := SettingPlan{Unit: variableUnit(t, "MATCHED"), Observed: github.Observed(setting.VariableValue("m")), Desired: setting.VariableValue("m"), Result: Match}
	deletion := SettingPlan{Unit: variableUnit(t, "LEGACY"), Observed: github.Observed(setting.VariableValue("old")), Desired: setting.Absent(), Result: Mismatch, Applicability: Applicable}
	plan := RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: []SettingPlan{mismatch, creation, deletion, matched, valueChange}}

	got, err := plan.LogicalChanges()
	if err != nil {
		t.Fatalf("LogicalChanges() error = %v", err)
	}
	// 削除も宣言の並びのまま変数ごとに一つで、期待状態は存在しないことである
	want := []LogicalChange{
		{DesiredState: []DesiredSetting{{Unit: mismatch.Unit, Value: mismatch.Desired}}, Operations: []UpdateOperation{operationOf(mismatch)}},
		{DesiredState: []DesiredSetting{{Unit: creation.Unit, Value: creation.Desired}}, Operations: []UpdateOperation{variableOperationOf(creation, creation.Observed)}},
		{DesiredState: []DesiredSetting{{Unit: deletion.Unit, Value: setting.Absent()}}, Operations: []UpdateOperation{{
			Kind: RepositoryVariableDeletion, Unit: deletion.Unit, RelevantSettings: []RelevantSetting{{Unit: deletion.Unit, AnticipatedValue: deletion.Observed}},
		}}},
		{DesiredState: []DesiredSetting{{Unit: valueChange.Unit, Value: valueChange.Desired}}, Operations: []UpdateOperation{variableOperationOf(valueChange, valueChange.Observed)}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
	}
}

// TestRepositoryPlan_LogicalChangesOfEnvironmentVariables は、Environment Variable の変更予定を、変数ごとに一つの更新操作からなる論理的な設定変更にし、
// 計画時に存在した Environment では Environment、変数の順に、作成する Environment では変数が存在しないことだけを基準に載せることを確かめる。
//
//trace:checks INV-001 INV-004 INV-005 FR-010 FR-011
func TestRepositoryPlan_LogicalChangesOfEnvironmentVariables(t *testing.T) {
	production, staging := mustEnvironmentName(t, "production"), mustEnvironmentName(t, "staging")
	region, debug := mustVariableName(t, "REGION"), mustVariableName(t, "DEBUG")
	stagingExistence := SettingPlan{
		Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: staging}, Observed: github.Observed(setting.Present()), Desired: setting.Present(),
		EnvironmentIdentity: ObservedEnvironmentIdentity(7), Result: Match,
	}
	stagingEnvironment := RelevantSetting{Unit: stagingExistence.Unit, AnticipatedValue: github.Observed(setting.Present()), AnticipatedIdentity: ObservedEnvironmentIdentity(7)}
	productionUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	productionCreation := SettingPlan{Unit: productionUnit, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch, Applicability: Applicable}
	dev := mustEnvironmentName(t, "dev")
	devUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: dev}
	variablePlan := func(unit setting.Unit, observed github.SettingObservation[setting.Value], result ReconciliationResult) SettingPlan {
		return SettingPlan{Unit: unit, Observed: observed, Desired: setting.VariableValue("eu"), Result: result, Applicability: Applicable}
	}
	variableChange := func(kind UpdateOperationKind, unit setting.Unit, relevant ...RelevantSetting) LogicalChange {
		return LogicalChange{
			DesiredState: []DesiredSetting{{Unit: unit, Value: setting.VariableValue("eu")}},
			Operations:   []UpdateOperation{{Kind: kind, Unit: unit, Value: setting.VariableValue("eu"), RelevantSettings: relevant}},
		}
	}
	absentVariable := github.Observed(setting.Absent())
	cases := []struct {
		name     string
		settings []SettingPlan
		want     []LogicalChange
	}{
		{
			name:     "value change of an existing environment is based on the environment and then the planned value",
			settings: []SettingPlan{stagingExistence, variablePlan(environmentVariableUnit(staging, region), github.Observed(setting.VariableValue("us")), Mismatch)},
			want: []LogicalChange{variableChange(EnvironmentVariableValueUpdate, environmentVariableUnit(staging, region),
				stagingEnvironment, RelevantSetting{Unit: environmentVariableUnit(staging, region), AnticipatedValue: github.Observed(setting.VariableValue("us"))})},
		},
		{
			name: "variables of an environment to create follow its creation one change each and are based only on their absence",
			settings: []SettingPlan{
				productionCreation,
				variablePlan(environmentVariableUnit(production, region), github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), NotComparable),
				variablePlan(environmentVariableUnit(production, debug), github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), NotComparable),
			},
			want: []LogicalChange{
				{
					DesiredState: []DesiredSetting{{Unit: productionUnit, Value: setting.Present()}},
					Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: productionUnit, RelevantSettings: []RelevantSetting{{Unit: productionUnit, AnticipatedValue: github.Observed(setting.Absent())}}}},
				},
				variableChange(EnvironmentVariableCreation, environmentVariableUnit(production, region), RelevantSetting{Unit: environmentVariableUnit(production, region), AnticipatedValue: absentVariable}),
				variableChange(EnvironmentVariableCreation, environmentVariableUnit(production, debug), RelevantSetting{Unit: environmentVariableUnit(production, debug), AnticipatedValue: absentVariable}),
			},
		},
		{
			name: "deletion in an existing environment is based on the environment and then the planned value",
			settings: []SettingPlan{stagingExistence, {
				Unit: environmentVariableUnit(staging, region), Observed: github.Observed(setting.VariableValue("us")), Desired: setting.Absent(), Result: Mismatch, Applicability: Applicable,
			}},
			want: []LogicalChange{{
				DesiredState: []DesiredSetting{{Unit: environmentVariableUnit(staging, region), Value: setting.Absent()}},
				Operations: []UpdateOperation{{Kind: EnvironmentVariableDeletion, Unit: environmentVariableUnit(staging, region), RelevantSettings: []RelevantSetting{
					stagingEnvironment, {Unit: environmentVariableUnit(staging, region), AnticipatedValue: github.Observed(setting.VariableValue("us"))},
				}}},
			}},
		},
		{
			// 作られたばかりの Environment には消す変数がないため、存在しないことを作成の期待状態に加え、作成のあとの観測で確かめる
			name: "declared absence in an environment to create joins the desired state of that creation without an update operation",
			settings: []SettingPlan{
				productionCreation,
				variablePlan(environmentVariableUnit(production, region), github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), NotComparable),
				{Unit: environmentVariableUnit(production, debug), Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.Absent(), Result: NotComparable, Applicability: Applicable},
				{Unit: devUnit, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch, Applicability: Applicable},
				{Unit: environmentVariableUnit(dev, region), Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.Absent(), Result: NotComparable, Applicability: Applicable},
			},
			want: []LogicalChange{
				{
					DesiredState: []DesiredSetting{{Unit: productionUnit, Value: setting.Present()}, {Unit: environmentVariableUnit(production, debug), Value: setting.Absent()}},
					Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: productionUnit, RelevantSettings: []RelevantSetting{{Unit: productionUnit, AnticipatedValue: github.Observed(setting.Absent())}}}},
				},
				variableChange(EnvironmentVariableCreation, environmentVariableUnit(production, region), RelevantSetting{Unit: environmentVariableUnit(production, region), AnticipatedValue: absentVariable}),
				{
					DesiredState: []DesiredSetting{{Unit: devUnit, Value: setting.Present()}, {Unit: environmentVariableUnit(dev, region), Value: setting.Absent()}},
					Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: devUnit, RelevantSettings: []RelevantSetting{{Unit: devUnit, AnticipatedValue: github.Observed(setting.Absent())}}}},
				},
			},
		},
		{
			name: "declared absence follows the deployment branch policy in the desired state of a creation with a mode",
			settings: []SettingPlan{
				productionCreation,
				{
					Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
					Desired: setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}), Result: NotComparable, Applicability: Applicable,
				},
				{Unit: environmentVariableUnit(production, debug), Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.Absent(), Result: NotComparable, Applicability: Applicable},
			},
			want: []LogicalChange{{
				DesiredState: []DesiredSetting{
					{Unit: productionUnit, Value: setting.Present()},
					{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Value: setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll})},
					{Unit: environmentVariableUnit(production, debug), Value: setting.Absent()},
				},
				Operations: []UpdateOperation{{
					Kind: EnvironmentCreationWithMode, Unit: productionUnit, Mode: setting.DeploymentBranchPolicyAll,
					RelevantSettings: []RelevantSetting{{Unit: productionUnit, AnticipatedValue: github.Observed(setting.Absent())}},
				}},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: tc.settings}.LogicalChanges()
			if err != nil {
				t.Fatalf("LogicalChanges() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("LogicalChanges() =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}

	// 対にならない形は、reconcile と judgeApplicability が作らないため、基準のない更新操作を返さずに panic する
	panics := []struct {
		name     string
		settings []SettingPlan
	}{
		{name: "variable without the plan of its environment panics", settings: []SettingPlan{variablePlan(environmentVariableUnit(staging, region), absentVariable, Mismatch)}},
		{
			name:     "not comparable variable of an existing environment panics",
			settings: []SettingPlan{stagingExistence, variablePlan(environmentVariableUnit(staging, region), github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), NotComparable)},
		},
		{
			name:     "mismatched variable of an environment to create panics",
			settings: []SettingPlan{productionCreation, variablePlan(environmentVariableUnit(production, region), absentVariable, Mismatch)},
		},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("LogicalChanges() did not panic")
				}
			}()
			_, _ = RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: tc.settings}.LogicalChanges()
		})
	}
}

func TestRepositoryPlan_SettingPlanOf(t *testing.T) {
	first := SettingPlan{Unit: variableUnit(t, "FIRST"), Desired: setting.VariableValue("1")}
	second := SettingPlan{Unit: variableUnit(t, "SECOND"), Desired: setting.VariableValue("2")}
	got, ok := RepositoryPlan{Settings: []SettingPlan{first, second}}.settingPlanOf(second.Unit)
	if !ok || got.Unit != second.Unit {
		t.Errorf("settingPlanOf(%s) = %+v, %v, want the plan of the same variable name", second.Unit.String(), got, ok)
	}
}

// TestRepositoryPlan_LogicalChangesOfEnvironments は、Environment の変更予定から導く更新操作の形と、その変更前提の基準を確かめる。
//
//trace:checks INV-001 INV-002 INV-003 INV-004 FR-010 FR-011
func TestRepositoryPlan_LogicalChangesOfEnvironments(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	staging := mustEnvironmentName(t, "staging")
	mismatch := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
	all := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll})
	protected := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches})
	creation := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch, Applicability: Applicable}
	creationMode := SettingPlan{
		Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
		Desired: protected, Result: NotComparable, Applicability: Applicable,
	}
	existing := SettingPlan{
		Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: staging}, Observed: github.Observed(setting.Present()), Desired: setting.Present(),
		EnvironmentIdentity: ObservedEnvironmentIdentity(7), Result: Match,
	}
	modeChange := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}, Observed: github.Observed(all), Desired: protected, Result: Mismatch, Applicability: Applicable}
	newRepositoryPlan := func(settings ...SettingPlan) RepositoryPlan {
		return RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}
	productionUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	stagingUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: staging}
	stagingPolicyUnit := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}
	productionPolicyUnit := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}
	allowAutoMerge := LogicalChange{
		DesiredState: []DesiredSetting{{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Value: setting.BoolValue(true)}},
		Operations: []UpdateOperation{{
			Kind: SettingUpdate, Unit: setting.Unit{Key: setting.AllowAutoMerge}, Value: setting.BoolValue(true),
			RelevantSettings: []RelevantSetting{{Unit: setting.Unit{Key: setting.AllowAutoMerge}, AnticipatedValue: github.Observed(setting.BoolValue(false))}},
		}},
	}
	// 作成の基準は存在しないことだけで、方式を宣言していても方式の基準は並ばない
	productionAbsent := []RelevantSetting{{Unit: productionUnit, AnticipatedValue: github.Observed(setting.Absent())}}
	createProduction := LogicalChange{
		DesiredState: []DesiredSetting{{Unit: productionUnit, Value: setting.Present()}},
		Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: productionUnit, RelevantSettings: productionAbsent}},
	}
	// createProductionWithMode は、デプロイ元制限を desired と宣言した作成の、期待状態と、方式を送る作成の更新操作に、operations を続けたもの。
	createProductionWithMode := func(desired setting.Value, operations ...UpdateOperation) LogicalChange {
		creation := UpdateOperation{Kind: EnvironmentCreationWithMode, Unit: productionUnit, Mode: desired.DeploymentBranchPolicy().Mode, RelevantSettings: productionAbsent}
		return LogicalChange{
			DesiredState: []DesiredSetting{{Unit: productionUnit, Value: setting.Present()}, {Unit: productionPolicyUnit, Value: desired}},
			Operations:   append([]UpdateOperation{creation}, operations...),
		}
	}
	// stagingPolicyChange は、存在する staging のデプロイ元制限を desired にする期待状態と、それを実現する operations からなる論理的な設定変更。
	stagingPolicyChange := func(desired setting.Value, operations ...UpdateOperation) LogicalChange {
		return LogicalChange{DesiredState: []DesiredSetting{{Unit: stagingPolicyUnit, Value: desired}}, Operations: operations}
	}
	stagingEnvironment := RelevantSetting{Unit: stagingUnit, AnticipatedValue: github.Observed(setting.Present()), AnticipatedIdentity: ObservedEnvironmentIdentity(7)}
	// stagingModeUpdate は、計画時に observed だった staging の方式を desired の方式にする更新操作。
	// 方式の変更の基準は、Environment が存在し ID が同じこと、方式が計画時の観測値と同じことの順に並ぶ
	stagingModeUpdate := func(observed, desired setting.Value) UpdateOperation {
		return UpdateOperation{
			Kind: DeploymentBranchPolicyModeUpdate, Unit: stagingPolicyUnit, Mode: desired.DeploymentBranchPolicy().Mode,
			RelevantSettings: []RelevantSetting{stagingEnvironment, {Unit: stagingPolicyUnit, AnticipatedValue: github.Observed(observed)}},
		}
	}
	changeStagingMode := stagingPolicyChange(protected, stagingModeUpdate(all, protected))

	branch := func(name string) setting.DeploymentPattern {
		return setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: name}
	}
	tag := func(name string) setting.DeploymentPattern {
		return setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: name}
	}
	// selectedOf は、個別パターンを引数の並びのまま持つ selected の値。空でも空でない集合として持つ。
	selectedOf := func(patterns ...setting.DeploymentPattern) setting.Value {
		return setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{
			Mode: setting.DeploymentBranchPolicySelected, Patterns: append([]setting.DeploymentPattern{}, patterns...),
		})
	}
	selectedMain := selectedOf(branch("main"))
	selectedPolicyPlan := func(environment setting.EnvironmentName, observed, desired setting.Value, result ReconciliationResult) SettingPlan {
		return SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}, Observed: github.Observed(observed), Desired: desired, Result: result, Applicability: Applicable}
	}
	creationSelected := func(environment setting.EnvironmentName, desired setting.Value) SettingPlan {
		return SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
			Desired: desired, Result: NotComparable, Applicability: Applicable,
		}
	}
	// patternStep は、個別パターンの更新操作一つと、その更新操作の基準のデプロイ元制限の値。
	// 存在する Environment の基準は、存在と ID、デプロイ元制限の値の順に並ぶ。
	type patternStep struct {
		kind    UpdateOperationKind
		pattern setting.DeploymentPattern
		base    setting.Value
	}
	patternSteps := func(steps ...patternStep) []UpdateOperation {
		var operations []UpdateOperation
		for _, step := range steps {
			operations = append(operations, UpdateOperation{
				Kind: step.kind, Unit: stagingPolicyUnit, Pattern: step.pattern,
				RelevantSettings: []RelevantSetting{stagingEnvironment, {Unit: stagingPolicyUnit, AnticipatedValue: github.Observed(step.base)}},
			})
		}
		return operations
	}
	// 作成に続く更新操作の基準は、先行する更新操作が作成して存在することと、デプロイ元制限の値の順に並ぶ。Unit は作成の綴りで作る。
	productionSteps := func(steps ...patternStep) []UpdateOperation {
		var operations []UpdateOperation
		for _, step := range steps {
			operations = append(operations, UpdateOperation{
				Kind: step.kind, Unit: productionPolicyUnit, Pattern: step.pattern,
				RelevantSettings: []RelevantSetting{{Unit: productionUnit, AnticipatedValue: github.Observed(setting.Present()), AnticipatedIdentity: CreatedByPrecedingOperation()}, {Unit: productionPolicyUnit, AnticipatedValue: github.Observed(step.base)}},
			})
		}
		return operations
	}

	cases := []struct {
		name     string
		settings []SettingPlan
		want     []LogicalChange
	}{
		{
			name:     "creation without a mode is one update sending no mode and relevant only to the absence",
			settings: []SettingPlan{creation},
			want:     []LogicalChange{createProduction},
		},
		{
			name:     "creation with a mode is one logical change of one update sending the mode at the creation position",
			settings: []SettingPlan{creation, creationMode},
			want:     []LogicalChange{createProductionWithMode(protected)},
		},
		{
			name: "creation paired with the mode of the name differing only in case sends the mode",
			settings: []SettingPlan{creation, func() SettingPlan {
				s := creationMode
				s.Unit.Environment = mustEnvironmentName(t, "Production")
				return s
			}()},
			want: []LogicalChange{createProductionWithMode(protected)},
		},
		{
			name:     "mode change of an existing environment is relevant to its identity and then to the observed mode",
			settings: []SettingPlan{existing, modeChange},
			want:     []LogicalChange{changeStagingMode},
		},
		{
			name:     "environment changes follow the other changes in the plan order",
			settings: []SettingPlan{mismatch, creation, creationMode, existing, modeChange},
			want:     []LogicalChange{allowAutoMerge, createProductionWithMode(protected), changeStagingMode},
		},
		{
			name:     "matched environment does not prevent the other logical changes",
			settings: []SettingPlan{mismatch, existing},
			want:     []LogicalChange{allowAutoMerge},
		},
		{
			name:     "all to selected puts the mode and then adds the declared patterns in the declared order, each relevant to the value advanced by the preceding updates",
			settings: []SettingPlan{existing, selectedPolicyPlan(staging, all, selectedOf(branch("main"), tag("v*")), Mismatch)},
			want: []LogicalChange{stagingPolicyChange(selectedOf(branch("main"), tag("v*")), append([]UpdateOperation{stagingModeUpdate(all, selectedOf(branch("main"), tag("v*")))},
				patternSteps(
					patternStep{DeploymentPatternAddition, branch("main"), selectedOf()},
					patternStep{DeploymentPatternAddition, tag("v*"), selectedOf(branch("main"))},
				)...)...)},
		},
		{
			name:     "all to selected without patterns is one mode update",
			settings: []SettingPlan{existing, selectedPolicyPlan(staging, all, selectedOf(), Mismatch)},
			want:     []LogicalChange{stagingPolicyChange(selectedOf(), stagingModeUpdate(all, selectedOf()))},
		},
		{
			name: "selected to selected removes the patterns not declared in the observed order and then adds the missing ones in the declared order without updating the mode",
			settings: []SettingPlan{existing, selectedPolicyPlan(staging,
				selectedOf(branch("b"), branch("a")), selectedOf(branch("d"), branch("a"), branch("c")), Mismatch)},
			want: []LogicalChange{stagingPolicyChange(selectedOf(branch("d"), branch("a"), branch("c")), patternSteps(
				patternStep{DeploymentPatternRemoval, branch("b"), selectedOf(branch("b"), branch("a"))},
				patternStep{DeploymentPatternAddition, branch("d"), selectedOf(branch("a"))},
				patternStep{DeploymentPatternAddition, branch("c"), selectedOf(branch("a"), branch("d"))},
			)...)},
		},
		{
			name:     "selected to no patterns removes each pattern relevant to the value without the patterns removed before",
			settings: []SettingPlan{existing, selectedPolicyPlan(staging, selectedOf(branch("a"), branch("b")), selectedOf(), Mismatch)},
			want: []LogicalChange{stagingPolicyChange(selectedOf(), patternSteps(
				patternStep{DeploymentPatternRemoval, branch("a"), selectedOf(branch("a"), branch("b"))},
				patternStep{DeploymentPatternRemoval, branch("b"), selectedOf(branch("b"))},
			)...)},
		},
		{
			name:     "patterns of the same name and another type are replaced",
			settings: []SettingPlan{existing, selectedPolicyPlan(staging, selectedOf(branch("main")), selectedOf(tag("main")), Mismatch)},
			want: []LogicalChange{stagingPolicyChange(selectedOf(tag("main")), patternSteps(
				patternStep{DeploymentPatternRemoval, branch("main"), selectedOf(branch("main"))},
				patternStep{DeploymentPatternAddition, tag("main"), selectedOf()},
			)...)},
		},
		{
			name:     "creation with patterns is one logical change of the creation followed by the additions relevant to the created environment",
			settings: []SettingPlan{creation, creationSelected(production, selectedOf(branch("main"), tag("v*")))},
			want: []LogicalChange{createProductionWithMode(selectedOf(branch("main"), tag("v*")), productionSteps(
				patternStep{DeploymentPatternAddition, branch("main"), selectedOf()},
				patternStep{DeploymentPatternAddition, tag("v*"), selectedOf(branch("main"))},
			)...)},
		},
		{
			name:     "creation with selected and no patterns is one update",
			settings: []SettingPlan{creation, creationSelected(production, selectedOf())},
			want:     []LogicalChange{createProductionWithMode(selectedOf())},
		},
		{
			name: "desired mode and additions after a creation use the spelling of the creation",
			settings: []SettingPlan{creation, func() SettingPlan {
				return creationSelected(mustEnvironmentName(t, "Production"), selectedMain)
			}()},
			want: []LogicalChange{createProductionWithMode(selectedMain, productionSteps(patternStep{DeploymentPatternAddition, branch("main"), selectedOf()})...)},
		},
		{
			name: "selected of two environments are two logical changes in the plan order",
			settings: []SettingPlan{
				existing, selectedPolicyPlan(staging, all, selectedMain, Mismatch),
				creation, creationSelected(production, selectedMain),
			},
			want: []LogicalChange{
				stagingPolicyChange(selectedMain, append([]UpdateOperation{stagingModeUpdate(all, selectedMain)}, patternSteps(patternStep{DeploymentPatternAddition, branch("main"), selectedOf()})...)...),
				createProductionWithMode(selectedMain, productionSteps(patternStep{DeploymentPatternAddition, branch("main"), selectedOf()})...),
			},
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

	// reconcile と judgeApplicability が作らない組
	panicCases := []struct {
		name     string
		settings []SettingPlan
	}{
		{
			name:     "mode of an absent environment without its creation panics",
			settings: []SettingPlan{creationMode},
		},
		{
			name: "mode of an absent environment following a matched existence panics",
			settings: []SettingPlan{{
				Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Observed: github.Observed(setting.Present()), Desired: setting.Present(),
				EnvironmentIdentity: ObservedEnvironmentIdentity(1), Result: Match,
			}, creationMode},
		},
		{
			name:     "mode mismatch without the existence panics",
			settings: []SettingPlan{modeChange},
		},
		{
			name:     "creation followed by a mode mismatch of the same name panics",
			settings: []SettingPlan{creation, {Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(all), Desired: protected, Result: Mismatch, Applicability: Applicable}},
		},
		{
			name:     "creation followed by a matched mode of the same name panics",
			settings: []SettingPlan{creation, {Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(protected), Desired: protected, Result: Match}},
		},
		{
			name: "applicable not comparable existence panics",
			settings: []SettingPlan{{
				Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentNameUndetermined), Desired: setting.Present(),
				Result: NotComparable, Applicability: Applicable,
			}},
		},
		{
			name: "applicable mode not comparable for another reason panics",
			settings: []SettingPlan{creation, {
				Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentNameUndetermined),
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
		want := []LogicalChange{stagingPolicyChange(all, stagingModeUpdate(selectedMain, all))}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() =\n%+v\nwant\n%+v", got, want)
		}
	})

	t.Run("not-applicable environment is rejected naming the environment", func(t *testing.T) {
		undetermined := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Result: NotComparable, Applicability: NotApplicable}
		_, err := newRepositoryPlan(undetermined).LogicalChanges()
		if want := "owner/repo: environments.production cannot be applied"; err == nil || err.Error() != want {
			t.Errorf("LogicalChanges() error = %v, want %q", err, want)
		}
	})
}

// TestLogicalChange_ExcludedFromOutsideSettings は、変更対象外の設定から、変える設定に加えて副作用として提示した設定も除くことを確かめる。
//
//trace:checks INV-001 FR-012
func TestLogicalChange_ExcludedFromOutsideSettings(t *testing.T) {
	change := LogicalChange{
		DesiredState:       []DesiredSetting{{Unit: setting.Unit{Key: setting.SecretScanningEnabled}}},
		SideEffectSettings: []setting.Unit{{Key: setting.RepositoryPushProtection}},
	}
	want := []setting.Unit{{Key: setting.SecretScanningEnabled}, {Key: setting.RepositoryPushProtection}}
	if got := change.ExcludedFromOutsideSettings(); !reflect.DeepEqual(got, want) {
		t.Errorf("ExcludedFromOutsideSettings() = %v, want %v", got, want)
	}
}

//trace:checks INV-001 INV-004 FR-010 FR-011
func TestRepositoryPlan_LogicalChangesOfSecretScanning(t *testing.T) {
	newRepositoryPlan := func(settings ...SettingPlan) RepositoryPlan {
		return RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}
	mismatch := func(key setting.Key, observed, desired bool) SettingPlan {
		return SettingPlan{
			Unit: setting.Unit{Key: key}, Observed: github.Observed(setting.BoolValue(observed)), Desired: setting.BoolValue(desired),
			Result: Mismatch, Applicability: Applicable,
		}
	}
	keys := func(changes []LogicalChange) []setting.Key {
		var got []setting.Key
		for _, c := range changes {
			got = append(got, c.DesiredState[0].Unit.Key)
		}
		return got
	}
	dependabotAlertsDisabled := SettingPlan{
		Unit: setting.Unit{Key: setting.DependabotAlerts}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false),
		Result: Mismatch, Applicability: Applicable,
	}
	dependabotUpdatesDisabled := SettingPlan{
		Unit:     setting.Unit{Key: setting.DependabotSecurityUpdates},
		Observed: github.Observed(setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesEnabled)),
		Desired:  setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesDisabled), Result: Mismatch, Applicability: Applicable,
	}
	orders := []struct {
		name     string
		settings []SettingPlan
		want     []setting.Key
	}{
		{
			name: "disabling both pairs runs each dependent setting just before its prerequisite",
			settings: []SettingPlan{
				dependabotAlertsDisabled, dependabotUpdatesDisabled,
				mismatch(setting.SecretScanningEnabled, true, false), mismatch(setting.RepositoryPushProtection, true, false),
			},
			want: []setting.Key{setting.DependabotSecurityUpdates, setting.DependabotAlerts, setting.RepositoryPushProtection, setting.SecretScanningEnabled},
		},
	}
	for _, tc := range orders {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newRepositoryPlan(tc.settings...).LogicalChanges()
			if err != nil {
				t.Fatalf("LogicalChanges() error = %v", err)
			}
			if !reflect.DeepEqual(keys(got), tc.want) {
				t.Errorf("LogicalChanges() order = %v, want %v", keys(got), tc.want)
			}
		})
	}

	self := func(s SettingPlan) RelevantSetting {
		return RelevantSetting{Unit: s.Unit, AnticipatedValue: s.Observed}
	}
	enabling := mismatch(setting.RepositoryPushProtection, false, true)
	disabling := mismatch(setting.RepositoryPushProtection, true, false)
	changes := []struct {
		name         string
		plan         SettingPlan
		wantRelevant []RelevantSetting
	}{
		{
			name:         "enabling push protection is relevant to secret scanning being enabled",
			plan:         enabling,
			wantRelevant: []RelevantSetting{self(enabling), {Unit: setting.Unit{Key: setting.SecretScanningEnabled}, AnticipatedValue: github.Observed(setting.BoolValue(true))}},
		},
		{
			name:         "disabling push protection is relevant only to itself",
			plan:         disabling,
			wantRelevant: []RelevantSetting{self(disabling)},
		},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newRepositoryPlan(tc.plan).LogicalChanges()
			if err != nil {
				t.Fatalf("LogicalChanges() error = %v", err)
			}
			want := []LogicalChange{{
				DesiredState: []DesiredSetting{{Unit: tc.plan.Unit, Value: tc.plan.Desired}},
				Operations:   []UpdateOperation{{Kind: SettingUpdate, Unit: tc.plan.Unit, Value: tc.plan.Desired, RelevantSettings: tc.wantRelevant}},
			}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
			}
		})
	}
}

//trace:checks FR-011 INV-001 INV-004
func TestRepositoryPlan_LogicalChangesOfDependabot(t *testing.T) {
	newRepositoryPlan := func(settings ...SettingPlan) RepositoryPlan {
		return RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}
	updatesValue := func(state setting.DependabotSecurityUpdatesState) setting.Value {
		return setting.DependabotSecurityUpdatesValue(state)
	}
	alerts := func(observed, desired bool, without ...SettingWithoutSideEffect) SettingPlan {
		return SettingPlan{
			Unit: setting.Unit{Key: setting.DependabotAlerts}, Observed: github.Observed(setting.BoolValue(observed)), Desired: setting.BoolValue(desired),
			Result: Mismatch, Applicability: Applicable, SettingsWithoutSideEffect: without,
		}
	}
	updates := func(observed, desired setting.DependabotSecurityUpdatesState) SettingPlan {
		result := Mismatch
		if updatesValue(observed).Equal(updatesValue(desired)) {
			result = Match
		}
		plan := SettingPlan{Unit: setting.Unit{Key: setting.DependabotSecurityUpdates}, Observed: github.Observed(updatesValue(observed)), Desired: updatesValue(desired), Result: result}
		if result == Mismatch {
			plan.Applicability = Applicable
		}
		return plan
	}
	// changeOf は、計画 s の期待値を送る更新操作一つからなる論理的な設定変更で、基準は s の計画時の観測値と extra である。
	changeOf := func(s SettingPlan, extra ...RelevantSetting) LogicalChange {
		relevant := append([]RelevantSetting{{Unit: s.Unit, AnticipatedValue: s.Observed}}, extra...)
		return LogicalChange{
			DesiredState: []DesiredSetting{{Unit: s.Unit, Value: s.Desired}},
			Operations:   []UpdateOperation{{Kind: SettingUpdate, Unit: s.Unit, Value: s.Desired, RelevantSettings: relevant}},
		}
	}
	withSideEffectSettings := func(c LogicalChange, units ...setting.Unit) LogicalChange {
		c.SideEffectSettings = units
		return c
	}
	keys := func(changes []LogicalChange) []setting.Key {
		var got []setting.Key
		for _, c := range changes {
			got = append(got, c.DesiredState[0].Unit.Key)
		}
		return got
	}
	cases := []struct {
		name     string
		settings []SettingPlan
		want     []setting.Key
	}{
		{
			name:     "disabling both runs security updates before alerts",
			settings: []SettingPlan{alerts(true, false), updates(setting.DependabotSecurityUpdatesEnabled, setting.DependabotSecurityUpdatesDisabled)},
			want:     []setting.Key{setting.DependabotSecurityUpdates, setting.DependabotAlerts},
		},
		{
			name:     "enabling both runs alerts before security updates",
			settings: []SettingPlan{alerts(false, true), updates(setting.DependabotSecurityUpdatesDisabled, setting.DependabotSecurityUpdatesEnabled)},
			want:     []setting.Key{setting.DependabotAlerts, setting.DependabotSecurityUpdates},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newRepositoryPlan(tc.settings...).LogicalChanges()
			if err != nil {
				t.Fatalf("LogicalChanges() error = %v", err)
			}
			if !reflect.DeepEqual(keys(got), tc.want) {
				t.Errorf("LogicalChanges() order = %v, want %v", keys(got), tc.want)
			}
		})
	}

	t.Run("disabling alerts is relevant to the security updates shown as a side effect and to the ones not shown", func(t *testing.T) {
		paused := github.Observed(updatesValue(setting.DependabotSecurityUpdatesPaused))
		withSideEffect := alerts(true, false)
		withSideEffect.SideEffects = []SideEffect{{Kind: SideEffectDependencyGraphDisabled}, {Kind: SideEffectSecurityUpdatesDisabled, Observed: paused}}
		disabled := github.Observed(updatesValue(setting.DependabotSecurityUpdatesDisabled))
		disabledUpdates := RelevantSetting{Unit: setting.Unit{Key: setting.DependabotSecurityUpdates}, AnticipatedValue: disabled}
		withoutSideEffect := alerts(true, false, SettingWithoutSideEffect{Key: setting.DependabotSecurityUpdates, Observed: disabled})
		withoutSideEffect.SideEffects = []SideEffect{{Kind: SideEffectDependencyGraphDisabled}}
		for _, tc := range []struct {
			plan SettingPlan
			want LogicalChange
		}{
			{plan: withSideEffect, want: withSideEffectSettings(changeOf(withSideEffect, RelevantSetting{Unit: setting.Unit{Key: setting.DependabotSecurityUpdates}, AnticipatedValue: paused}), setting.Unit{Key: setting.DependabotSecurityUpdates})},
			{plan: withoutSideEffect, want: changeOf(withoutSideEffect, disabledUpdates)},
		} {
			got, err := newRepositoryPlan(tc.plan).LogicalChanges()
			if err != nil {
				t.Fatalf("LogicalChanges() error = %v", err)
			}
			if !reflect.DeepEqual(got, []LogicalChange{tc.want}) {
				t.Errorf("LogicalChanges() = %+v, want %+v", got, []LogicalChange{tc.want})
			}
		}
	})
}
