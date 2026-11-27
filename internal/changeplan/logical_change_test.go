package changeplan

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
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

// TestUpdateOperation_Endpoint は、更新操作が更新要求を送る更新エンドポイントを、設定の識別から引けるかの規則を確かめる。
//
//trace:checks INV-001 FR-012
func TestUpdateOperation_Endpoint(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	cases := []struct {
		name         string
		operation    UpdateOperation
		wantEndpoint github.UpdateEndpoint
		wantOK       bool
	}{
		{
			name:         "setting update is sent to the endpoint of its setting",
			operation:    UpdateOperation{Kind: SettingUpdate, Unit: setting.Unit{Key: setting.DefaultWorkflowPermissions}},
			wantEndpoint: github.WorkflowPermissionsEndpoint, wantOK: true,
		},
		{
			name:         "merge methods update is sent to the endpoint of its first merge method",
			operation:    UpdateOperation{Kind: MergeMethodsUpdate, Unit: setting.Unit{Key: setting.AllowSquashMerge}},
			wantEndpoint: github.RepositoryEndpoint, wantOK: true,
		},
		{name: "environment update has no endpoint of a setting", operation: UpdateOperation{Kind: DeploymentBranchPolicyModeUpdate, Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}}},
		{name: "ruleset creation has no endpoint of a setting", operation: UpdateOperation{Kind: RulesetCreation, Unit: rulesetUnit(t, "protect")}},
		{name: "ruleset deletion has no endpoint of a setting", operation: UpdateOperation{Kind: RulesetDeletion, Unit: rulesetUnit(t, "protect")}},
		{name: "required reviewers update has no endpoint of a setting", operation: UpdateOperation{Kind: RequiredReviewersUpdate, Unit: setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: production}}},
		{name: "prevent self-review update has no endpoint of a setting", operation: UpdateOperation{Kind: PreventSelfReviewUpdate, Unit: setting.Unit{Key: setting.EnvironmentPreventSelfReview, Environment: production}}},
		{name: "wait timer update has no endpoint of a setting", operation: UpdateOperation{Kind: WaitTimerUpdate, Unit: setting.Unit{Key: setting.EnvironmentWaitTimer, Environment: production}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got, ok := tc.operation.Endpoint(); got != tc.wantEndpoint || ok != tc.wantOK {
				t.Errorf("Endpoint() = %d, %t, want %d, %t", got, ok, tc.wantEndpoint, tc.wantOK)
			}
		})
	}

	t.Run("undefined kind panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("Endpoint() did not panic")
			}
		}()
		(UpdateOperation{Unit: setting.Unit{Key: setting.AllowAutoMerge}}).Endpoint()
	})
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
				change.DesiredState = append(change.DesiredState, configuration.ManagedSetting{Unit: setting.Unit{Key: key}})
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
	mismatch := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable()}
	newRepositoryPlan := func(settings ...SettingPlan) RepositoryPlan {
		return RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}
	basis := func(key setting.Key, anticipated AnticipatedValue) PreconditionBasis {
		return PreconditionBasis{Unit: setting.Unit{Key: key}, AnticipatedValue: anticipated}
	}
	// settingUpdate は、key の値 desired を送る更新操作。
	settingUpdate := func(key setting.Key, desired setting.Value, bases ...PreconditionBasis) UpdateOperation {
		return UpdateOperation{Kind: SettingUpdate, Unit: setting.Unit{Key: key}, Value: desired, PreconditionBases: bases}
	}
	// settingChange は、key を desired にする期待状態と、その値を送る更新操作一つからなる論理的な設定変更。
	settingChange := func(key setting.Key, desired setting.Value, bases ...PreconditionBasis) LogicalChange {
		return LogicalChange{DesiredState: []configuration.ManagedSetting{{Unit: setting.Unit{Key: key}, Desired: desired}}, Operations: []UpdateOperation{settingUpdate(key, desired, bases...)}}
	}

	t.Run("merge method changes are one logical change at the first one and allow_update_branch is another", func(t *testing.T) {
		squash := SettingPlan{Unit: setting.Unit{Key: setting.AllowSquashMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable()}
		rebase := SettingPlan{Unit: setting.Unit{Key: setting.AllowRebaseMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false), Result: Mismatch, Applicability: Applicable()}
		updateBranch := SettingPlan{Unit: setting.Unit{Key: setting.AllowUpdateBranch}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable()}
		got, err := newRepositoryPlan(mismatch, squash, rebase, updateBranch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			settingChange(setting.AllowAutoMerge, setting.BoolValue(true), basis(setting.AllowAutoMerge, AnticipateValue(setting.BoolValue(false)))),
			mergeMethodsChange([]SettingPlan{squash, rebase}),
			settingChange(setting.AllowUpdateBranch, setting.BoolValue(true), basis(setting.AllowUpdateBranch, AnticipateValue(setting.BoolValue(false)))),
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
		want := []LogicalChange{settingChange(setting.AllowAutoMerge, setting.BoolValue(true), basis(setting.AllowAutoMerge, AnticipateValue(setting.BoolValue(false))))}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("applicable mismatches of both settings become logical changes in the plan order", func(t *testing.T) {
		deleteBranch := SettingPlan{Unit: setting.Unit{Key: setting.DeleteBranchOnMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable()}
		got, err := newRepositoryPlan(deleteBranch, mismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			settingChange(setting.DeleteBranchOnMerge, setting.BoolValue(true), basis(setting.DeleteBranchOnMerge, AnticipateValue(setting.BoolValue(false)))),
			settingChange(setting.AllowAutoMerge, setting.BoolValue(true), basis(setting.AllowAutoMerge, AnticipateValue(setting.BoolValue(false)))),
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	enabling := SettingPlan{Unit: setting.Unit{Key: setting.ActionsEnabled}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable()}

	t.Run("applicable change of the enabled state becomes a logical change relevant only to its own observed value", func(t *testing.T) {
		got, err := newRepositoryPlan(enabling).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{settingChange(setting.ActionsEnabled, setting.BoolValue(true), basis(setting.ActionsEnabled, AnticipateValue(setting.BoolValue(false))))}
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
			Result: Mismatch, Applicability: Applicable(), SideEffects: effects,
		}
	}

	t.Run("applicable mismatch without side effects becomes a logical change", func(t *testing.T) {
		got, err := newRepositoryPlan(policyChange(setting.AllowedActionsAll, setting.AllowedActionsLocalOnly)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{settingChange(setting.AllowedActions, policy(setting.AllowedActionsLocalOnly), basis(setting.AllowedActions, AnticipateValue(policy(setting.AllowedActionsAll))))}
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
			basis(setting.AllowedActions, AnticipateValue(policy(setting.AllowedActionsSelected))),
			basis(setting.SelectedActions, AnticipateValue(criteria)),
		)}
		want[0].SideEffectSettings = []setting.Unit{{Key: setting.SelectedActions}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("policy not comparable because actions is disabled is relevant to the unavailable values", func(t *testing.T) {
		enable := SettingPlan{
			Unit: setting.Unit{Key: setting.AllowedActions}, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly), Result: NotComparable, Applicability: Applicable(),
			SideEffects: []SideEffect{{Kind: SideEffectSelectedActionsDiscarded, Observed: disabled}},
		}
		got, err := newRepositoryPlan(enable).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{settingChange(setting.AllowedActions, policy(setting.AllowedActionsLocalOnly), basis(setting.AllowedActions, AnticipateUnchanged(disabled)), basis(setting.SelectedActions, AnticipateUnchanged(disabled)))}
		want[0].SideEffectSettings = []setting.Unit{{Key: setting.SelectedActions}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	notComparableCriteria := func(reason github.UnavailableReason) SettingPlan {
		return SettingPlan{
			Unit: setting.Unit{Key: setting.SelectedActions}, Observed: github.Unavailable[setting.Value](reason),
			Desired: criteria, Result: NotComparable, Applicability: Applicable(),
		}
	}
	// toSelected は、ポリシーを selected にする更新操作のあとに続く、選択方式の Actions 許可条件の更新操作。
	// 基準は1つ目による予定どおりの変化を反映したポリシーだけで、2つ目がすべて上書きする3項目は並ばない。
	toSelected := settingUpdate(setting.SelectedActions, criteria, basis(setting.AllowedActions, AnticipateValue(policy(setting.AllowedActionsSelected))))
	// toSelectedDesired は、ポリシーと許可条件をまとめた論理的な設定変更の期待状態で、ポリシー、許可条件の順に並ぶ。
	toSelectedDesired := []configuration.ManagedSetting{{Unit: setting.Unit{Key: setting.AllowedActions}, Desired: policy(setting.AllowedActionsSelected)}, {Unit: setting.Unit{Key: setting.SelectedActions}, Desired: criteria}}
	workflow := SettingPlan{
		Unit: setting.Unit{Key: setting.DefaultWorkflowPermissions}, Observed: github.Observed(setting.PermissionLevelValue(setting.PermissionWrite)),
		Desired: setting.PermissionLevelValue(setting.PermissionRead), Result: Mismatch, Applicability: Applicable(),
	}

	t.Run("policy to selected and the criteria not comparable become one logical change of two updates at the policy position", func(t *testing.T) {
		got, err := newRepositoryPlan(mismatch, policyChange(setting.AllowedActionsAll, setting.AllowedActionsSelected), notComparableCriteria(github.UnavailableNotSelected), workflow).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			settingChange(setting.AllowAutoMerge, setting.BoolValue(true), basis(setting.AllowAutoMerge, AnticipateValue(setting.BoolValue(false)))),
			{DesiredState: toSelectedDesired, Operations: []UpdateOperation{
				settingUpdate(setting.AllowedActions, policy(setting.AllowedActionsSelected), basis(setting.AllowedActions, AnticipateValue(policy(setting.AllowedActionsAll)))),
				toSelected,
			}},
			settingChange(workflow.Unit.Key, workflow.Desired, basis(workflow.Unit.Key, AnticipateUnchanged(workflow.Observed))),
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("policy enabling actions with selected and the criteria not comparable become one logical change of two updates", func(t *testing.T) {
		enableSelected := SettingPlan{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: disabled, Desired: policy(setting.AllowedActionsSelected), Result: NotComparable, Applicability: Applicable()}
		got, err := newRepositoryPlan(enableSelected, notComparableCriteria(github.UnavailableActionsDisabled)).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{DesiredState: toSelectedDesired, Operations: []UpdateOperation{
			settingUpdate(setting.AllowedActions, policy(setting.AllowedActionsSelected), basis(setting.AllowedActions, AnticipateUnchanged(disabled))),
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
				{Unit: setting.Unit{Key: setting.SelectedActions}, Observed: github.Observed(criteria), Desired: criteria, Result: Mismatch, Applicability: Applicable()},
			},
		},
		{
			name:     "enabled state with a policy panics",
			settings: []SettingPlan{enabling, {Unit: setting.Unit{Key: setting.AllowedActions}, Observed: disabled, Desired: policy(setting.AllowedActionsLocalOnly), Result: NotComparable, Applicability: Applicable()}},
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
		notApplicable.Applicability = NotApplicable(NotApplicableActionsRemainDisabled)
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
			setting: SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: NotApplicable(NotApplicablePublicRepository)},
		},
		{
			name:    "mismatch without an applicability judgment is an error",
			setting: SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicability{}},
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
		Unit: setting.Unit{Key: setting.SHAPinningRequired}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable(),
	}
	actionsEnabled := basis(setting.ActionsEnabled, AnticipateValue(setting.BoolValue(true)))
	shaPinningChange := settingChange(setting.SHAPinningRequired, setting.BoolValue(true), basis(setting.SHAPinningRequired, AnticipateValue(setting.BoolValue(false))), actionsEnabled)

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
		shaPinningNotComparable := SettingPlan{Unit: setting.Unit{Key: setting.SHAPinningRequired}, Observed: disabled, Desired: setting.BoolValue(true), Result: NotComparable, Applicability: Applicable()}
		got, err := newRepositoryPlan(enabling, shaPinningNotComparable).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			settingChange(setting.ActionsEnabled, setting.BoolValue(true), basis(setting.ActionsEnabled, AnticipateValue(setting.BoolValue(false)))),
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
	mismatch := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable()}
	creation := SettingPlan{Unit: variableUnit(t, "ZETA"), Observed: github.Observed(setting.Absent()), Desired: setting.VariableValue("z"), Result: Mismatch, Applicability: Applicable()}
	valueChange := SettingPlan{Unit: variableUnit(t, "ALPHA"), Observed: github.Observed(setting.VariableValue("1")), Desired: setting.VariableValue("2"), Result: Mismatch, Applicability: Applicable()}
	// 値が一致した変数は、変更予定がないため論理的な設定変更にならない
	matched := SettingPlan{Unit: variableUnit(t, "MATCHED"), Observed: github.Observed(setting.VariableValue("m")), Desired: setting.VariableValue("m"), Result: Match}
	deletion := SettingPlan{Unit: variableUnit(t, "LEGACY"), Observed: github.Observed(setting.VariableValue("old")), Desired: setting.Absent(), Result: Mismatch, Applicability: Applicable()}
	plan := RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: []SettingPlan{mismatch, creation, deletion, matched, valueChange}}

	got, err := plan.LogicalChanges()
	if err != nil {
		t.Fatalf("LogicalChanges() error = %v", err)
	}
	// 削除も宣言の並びのまま変数ごとに一つで、期待状態は存在しないことである
	want := []LogicalChange{
		{DesiredState: []configuration.ManagedSetting{{Unit: mismatch.Unit, Desired: mismatch.Desired}}, Operations: []UpdateOperation{operationOf(mismatch)}},
		{DesiredState: []configuration.ManagedSetting{{Unit: creation.Unit, Desired: creation.Desired}}, Operations: []UpdateOperation{variableOperationOf(creation, AnticipateUnchanged(creation.Observed))}},
		{DesiredState: []configuration.ManagedSetting{{Unit: deletion.Unit, Desired: setting.Absent()}}, Operations: []UpdateOperation{{
			Kind: RepositoryVariableDeletion, Unit: deletion.Unit, PreconditionBases: []PreconditionBasis{{Unit: deletion.Unit, AnticipatedValue: AnticipateUnchanged(deletion.Observed)}},
		}}},
		{DesiredState: []configuration.ManagedSetting{{Unit: valueChange.Unit, Desired: valueChange.Desired}}, Operations: []UpdateOperation{variableOperationOf(valueChange, AnticipateUnchanged(valueChange.Observed))}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
	}
}

// TestRepositoryPlan_LogicalChangesOfRulesets は、Ruleset の変更予定が、Ruleset ごとに一つの更新操作からなる別々の論理的な設定変更になり、計画の並びで実行されることを確かめる。
//
//trace:checks INV-001 INV-002 INV-004 FR-011
func TestRepositoryPlan_LogicalChangesOfRulesets(t *testing.T) {
	desired := setting.RulesetValue(protectRuleset())
	disabled := protectRuleset()
	disabled.Enforcement = setting.RulesetEnforcementDisabled
	mismatch := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable()}
	creation := SettingPlan{Unit: rulesetUnit(t, "zeta"), Observed: github.Observed(setting.Absent()), Desired: desired, Result: Mismatch, Applicability: Applicable()}
	// 値が一致した Ruleset は、変更予定がないため論理的な設定変更にならない
	matched := SettingPlan{Unit: rulesetUnit(t, "matched"), Observed: github.Observed(desired), Desired: desired, Result: Match}
	valueChange := SettingPlan{Unit: rulesetUnit(t, "alpha"), Observed: github.Observed(setting.RulesetValue(disabled)), RulesetIdentity: ObservedRulesetIdentity(42), Desired: desired, Result: Mismatch, Applicability: Applicable()}
	deletion := SettingPlan{Unit: rulesetUnit(t, "legacy"), Observed: github.Observed(setting.RulesetValue(disabled)), RulesetIdentity: ObservedRulesetIdentity(43), Desired: setting.Absent(), Result: Mismatch, Applicability: Applicable()}
	plan := RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: []SettingPlan{mismatch, creation, matched, deletion, valueChange}}

	got, err := plan.LogicalChanges()
	if err != nil {
		t.Fatalf("LogicalChanges() error = %v", err)
	}
	want := []LogicalChange{
		{DesiredState: []configuration.ManagedSetting{{Unit: mismatch.Unit, Desired: mismatch.Desired}}, Operations: []UpdateOperation{operationOf(mismatch)}},
		rulesetChangeOf(creation),
		rulesetChangeOf(deletion),
		rulesetChangeOf(valueChange),
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
	stagingEnvironment := PreconditionBasis{Unit: stagingExistence.Unit, AnticipatedValue: AnticipateValue(setting.Present()), AnticipatedEnvironmentIdentity: ObservedEnvironmentIdentity(7)}
	productionUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	productionCreation := SettingPlan{Unit: productionUnit, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch, Applicability: Applicable()}
	dev := mustEnvironmentName(t, "dev")
	devUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: dev}
	variablePlan := func(unit setting.Unit, observed github.SettingObservation[setting.Value], result ReconciliationResult) SettingPlan {
		return SettingPlan{Unit: unit, Observed: observed, Desired: setting.VariableValue("eu"), Result: result, Applicability: Applicable()}
	}
	variableChange := func(kind UpdateOperationKind, unit setting.Unit, bases ...PreconditionBasis) LogicalChange {
		return LogicalChange{
			DesiredState: []configuration.ManagedSetting{{Unit: unit, Desired: setting.VariableValue("eu")}},
			Operations:   []UpdateOperation{{Kind: kind, Unit: unit, Value: setting.VariableValue("eu"), PreconditionBases: bases}},
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
				stagingEnvironment, PreconditionBasis{Unit: environmentVariableUnit(staging, region), AnticipatedValue: AnticipateValue(setting.VariableValue("us"))})},
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
					DesiredState: []configuration.ManagedSetting{{Unit: productionUnit, Desired: setting.Present()}},
					Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: productionUnit, PreconditionBases: []PreconditionBasis{{Unit: productionUnit, AnticipatedValue: AnticipateValue(setting.Absent())}}}},
				},
				variableChange(EnvironmentVariableCreation, environmentVariableUnit(production, region), PreconditionBasis{Unit: environmentVariableUnit(production, region), AnticipatedValue: AnticipateUnchanged(absentVariable)}),
				variableChange(EnvironmentVariableCreation, environmentVariableUnit(production, debug), PreconditionBasis{Unit: environmentVariableUnit(production, debug), AnticipatedValue: AnticipateUnchanged(absentVariable)}),
			},
		},
		{
			name: "deletion in an existing environment is based on the environment and then the planned value",
			settings: []SettingPlan{stagingExistence, {
				Unit: environmentVariableUnit(staging, region), Observed: github.Observed(setting.VariableValue("us")), Desired: setting.Absent(), Result: Mismatch, Applicability: Applicable(),
			}},
			want: []LogicalChange{{
				DesiredState: []configuration.ManagedSetting{{Unit: environmentVariableUnit(staging, region), Desired: setting.Absent()}},
				Operations: []UpdateOperation{{Kind: EnvironmentVariableDeletion, Unit: environmentVariableUnit(staging, region), PreconditionBases: []PreconditionBasis{
					stagingEnvironment, {Unit: environmentVariableUnit(staging, region), AnticipatedValue: AnticipateValue(setting.VariableValue("us"))},
				}}},
			}},
		},
		{
			// 作られたばかりの Environment には消す変数がないため、存在しないことを作成の期待状態に加え、作成のあとの観測で確かめる
			name: "declared absence in an environment to create joins the desired state of that creation without an update operation",
			settings: []SettingPlan{
				productionCreation,
				variablePlan(environmentVariableUnit(production, region), github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), NotComparable),
				{Unit: environmentVariableUnit(production, debug), Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.Absent(), Result: NotComparable, Applicability: Applicable()},
				{Unit: devUnit, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch, Applicability: Applicable()},
				{Unit: environmentVariableUnit(dev, region), Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.Absent(), Result: NotComparable, Applicability: Applicable()},
			},
			want: []LogicalChange{
				{
					DesiredState: []configuration.ManagedSetting{{Unit: productionUnit, Desired: setting.Present()}, {Unit: environmentVariableUnit(production, debug), Desired: setting.Absent()}},
					Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: productionUnit, PreconditionBases: []PreconditionBasis{{Unit: productionUnit, AnticipatedValue: AnticipateValue(setting.Absent())}}}},
				},
				variableChange(EnvironmentVariableCreation, environmentVariableUnit(production, region), PreconditionBasis{Unit: environmentVariableUnit(production, region), AnticipatedValue: AnticipateUnchanged(absentVariable)}),
				{
					DesiredState: []configuration.ManagedSetting{{Unit: devUnit, Desired: setting.Present()}, {Unit: environmentVariableUnit(dev, region), Desired: setting.Absent()}},
					Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: devUnit, PreconditionBases: []PreconditionBasis{{Unit: devUnit, AnticipatedValue: AnticipateValue(setting.Absent())}}}},
				},
			},
		},
		{
			name: "declared absence follows the deployment branch policy in the desired state of a creation with a mode",
			settings: []SettingPlan{
				productionCreation,
				{
					Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
					Desired: setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}), Result: NotComparable, Applicability: Applicable(),
				},
				{Unit: environmentVariableUnit(production, debug), Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.Absent(), Result: NotComparable, Applicability: Applicable()},
			},
			want: []LogicalChange{{
				DesiredState: []configuration.ManagedSetting{
					{Unit: productionUnit, Desired: setting.Present()},
					{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Desired: setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll})},
					{Unit: environmentVariableUnit(production, debug), Desired: setting.Absent()},
				},
				Operations: []UpdateOperation{{
					Kind: EnvironmentCreationWithMode, Unit: productionUnit, Mode: setting.DeploymentBranchPolicyAll,
					PreconditionBases: []PreconditionBasis{{Unit: productionUnit, AnticipatedValue: AnticipateValue(setting.Absent())}},
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

// TestRepositoryPlan_LogicalChangesOfRequiredReviewers は、担当者の集合の変更予定から導く論理的な設定変更と、その変更前提の基準を確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestRepositoryPlan_LogicalChangesOfRequiredReviewers(t *testing.T) {
	production, staging := mustEnvironmentName(t, "production"), mustEnvironmentName(t, "staging")
	region := mustVariableName(t, "REGION")
	stagingExistence := SettingPlan{
		Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: staging}, Observed: github.Observed(setting.Present()), Desired: setting.Present(),
		EnvironmentIdentity: ObservedEnvironmentIdentity(7), Result: Match,
	}
	stagingEnvironment := PreconditionBasis{Unit: stagingExistence.Unit, AnticipatedValue: AnticipateValue(setting.Present()), AnticipatedEnvironmentIdentity: ObservedEnvironmentIdentity(7)}
	productionUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	productionCreation := SettingPlan{Unit: productionUnit, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch, Applicability: Applicable()}
	creationBasis := PreconditionBasis{Unit: productionUnit, AnticipatedValue: AnticipateValue(setting.Absent())}
	stagingReviewers := setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: staging}
	productionReviewers := setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: production}
	productionPolicy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}
	stagingPolicy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}
	user1 := setting.ReviewerSetValue(setting.ReviewerSetOf([]setting.Reviewer{{Type: setting.ReviewerUser, ID: 1}}))
	user2 := setting.ReviewerSetValue(setting.ReviewerSetOf([]setting.Reviewer{{Type: setting.ReviewerUser, ID: 2}}))
	noReviewers := setting.ReviewerSetValue(setting.ReviewerSetOf(nil))
	all := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll})
	protected := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches})
	absentEnvironment := github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent)
	selfReviewPrevented, selfReviewAllowed := setting.BoolValue(true), setting.BoolValue(false)
	stagingPrevent := setting.Unit{Key: setting.EnvironmentPreventSelfReview, Environment: staging}
	reviewersChange := func(unit setting.Unit, desired setting.Value, bases ...PreconditionBasis) LogicalChange {
		return LogicalChange{
			DesiredState: []configuration.ManagedSetting{{Unit: unit, Desired: desired}},
			Operations:   []UpdateOperation{{Kind: RequiredReviewersUpdate, Unit: unit, Value: desired, PreconditionBases: bases}},
		}
	}
	cases := []struct {
		name     string
		settings []SettingPlan
		want     []LogicalChange
	}{
		{
			name: "mismatch of an existing environment is one update based on the environment and then the observed reviewers",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingReviewers, Observed: github.Observed(user1), Desired: user2, Result: Mismatch, Applicability: Applicable()},
			},
			want: []LogicalChange{reviewersChange(stagingReviewers, user2, stagingEnvironment, PreconditionBasis{Unit: stagingReviewers, AnticipatedValue: AnticipateValue(user1)})},
		},
		{
			name: "removing every reviewer bases the update on prevent_self_review discarded as a side effect, after the reviewers",
			settings: []SettingPlan{
				stagingExistence,
				{
					Unit: stagingReviewers, Observed: github.Observed(user1), Desired: noReviewers, Result: Mismatch, Applicability: Applicable(),
					SideEffects: []SideEffect{{Kind: SideEffectPreventSelfReviewDiscarded, Observed: github.Observed(setting.BoolValue(true))}},
				},
			},
			want: []LogicalChange{{
				DesiredState: []configuration.ManagedSetting{{Unit: stagingReviewers, Desired: noReviewers}},
				Operations: []UpdateOperation{{
					Kind: RequiredReviewersUpdate, Unit: stagingReviewers, Value: noReviewers,
					PreconditionBases: []PreconditionBasis{stagingEnvironment, {Unit: stagingReviewers, AnticipatedValue: AnticipateValue(user1)}, {Unit: stagingPrevent, AnticipatedValue: AnticipateValue(selfReviewPrevented)}},
				}},
				SideEffectSettings: []setting.Unit{stagingPrevent},
			}},
		},
		{
			name: "removing every reviewer bases the update on prevent_self_review expected to stay false",
			settings: []SettingPlan{
				stagingExistence,
				{
					Unit: stagingReviewers, Observed: github.Observed(user1), Desired: noReviewers, Result: Mismatch, Applicability: Applicable(),
					SettingsExpectedUnaffected: []SettingExpectedUnaffected{{Key: setting.EnvironmentPreventSelfReview, Observed: github.Observed(selfReviewAllowed)}},
				},
			},
			want: []LogicalChange{reviewersChange(stagingReviewers, noReviewers, stagingEnvironment, PreconditionBasis{Unit: stagingReviewers, AnticipatedValue: AnticipateValue(user1)}, PreconditionBasis{Unit: stagingPrevent, AnticipatedValue: AnticipateValue(selfReviewAllowed)})},
		},
		{
			name: "removing every reviewer with declared prevent_self_review false that is true is one change of both based on the declared plan",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingReviewers, Observed: github.Observed(user1), Desired: noReviewers, Result: Mismatch, Applicability: Applicable()},
				{Unit: stagingPrevent, Observed: github.Observed(selfReviewPrevented), Desired: selfReviewAllowed, Result: Mismatch, Applicability: Applicable()},
			},
			want: []LogicalChange{{
				DesiredState: []configuration.ManagedSetting{{Unit: stagingReviewers, Desired: noReviewers}, {Unit: stagingPrevent, Desired: selfReviewAllowed}},
				Operations: []UpdateOperation{{
					Kind: RequiredReviewersUpdate, Unit: stagingReviewers, Value: noReviewers,
					PreconditionBases: []PreconditionBasis{stagingEnvironment, {Unit: stagingReviewers, AnticipatedValue: AnticipateValue(user1)}, {Unit: stagingPrevent, AnticipatedValue: AnticipateValue(selfReviewPrevented)}},
				}},
			}},
		},
		{
			name: "removing every reviewer with declared prevent_self_review false that matches is based on it without changing it",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingReviewers, Observed: github.Observed(user1), Desired: noReviewers, Result: Mismatch, Applicability: Applicable()},
				{Unit: stagingPrevent, Observed: github.Observed(selfReviewAllowed), Desired: selfReviewAllowed, Result: Match},
			},
			want: []LogicalChange{reviewersChange(stagingReviewers, noReviewers, stagingEnvironment, PreconditionBasis{Unit: stagingReviewers, AnticipatedValue: AnticipateValue(user1)}, PreconditionBasis{Unit: stagingPrevent, AnticipatedValue: AnticipateValue(selfReviewAllowed)})},
		},
		{
			name: "replacing reviewers is not based on prevent_self_review",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingReviewers, Observed: github.Observed(user1), Desired: user2, Result: Mismatch, Applicability: Applicable()},
				{Unit: stagingPrevent, Observed: github.Observed(selfReviewPrevented), Desired: selfReviewPrevented, Result: Match},
			},
			want: []LogicalChange{reviewersChange(stagingReviewers, user2, stagingEnvironment, PreconditionBasis{Unit: stagingReviewers, AnticipatedValue: AnticipateValue(user1)})},
		},
		{
			name: "reviewers of an environment to create follow its creation as another change based only on no reviewers",
			settings: []SettingPlan{
				productionCreation,
				{Unit: productionReviewers, Observed: absentEnvironment, Desired: user1, Result: NotComparable, Applicability: Applicable()},
			},
			want: []LogicalChange{
				{
					DesiredState: []configuration.ManagedSetting{{Unit: productionUnit, Desired: setting.Present()}},
					Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: productionUnit, PreconditionBases: []PreconditionBasis{creationBasis}}},
				},
				reviewersChange(productionReviewers, user1, PreconditionBasis{Unit: productionReviewers, AnticipatedValue: AnticipateValue(noReviewers)}),
			},
		},
		{
			// 作られたばかりの Environment は担当者の保護ルールを持たないため、空集合を作成の期待状態に加え、作成のあとの観測で確かめる
			name: "no reviewers in an environment to create join the desired state of the creation between the mode and the declared absence of a variable",
			settings: []SettingPlan{
				productionCreation,
				{Unit: productionPolicy, Observed: absentEnvironment, Desired: all, Result: NotComparable, Applicability: Applicable()},
				{Unit: productionReviewers, Observed: absentEnvironment, Desired: noReviewers, Result: NotComparable, Applicability: Applicable()},
				{Unit: environmentVariableUnit(production, region), Observed: absentEnvironment, Desired: setting.Absent(), Result: NotComparable, Applicability: Applicable()},
			},
			want: []LogicalChange{{
				DesiredState: []configuration.ManagedSetting{
					{Unit: productionUnit, Desired: setting.Present()},
					{Unit: productionPolicy, Desired: all},
					{Unit: productionReviewers, Desired: noReviewers},
					{Unit: environmentVariableUnit(production, region), Desired: setting.Absent()},
				},
				Operations: []UpdateOperation{{Kind: EnvironmentCreationWithMode, Unit: productionUnit, Mode: setting.DeploymentBranchPolicyAll, PreconditionBases: []PreconditionBasis{creationBasis}}},
			}},
		},
		{
			name: "no reviewers in an environment to create without a mode join the creation without an update",
			settings: []SettingPlan{
				productionCreation,
				{Unit: productionReviewers, Observed: absentEnvironment, Desired: noReviewers, Result: NotComparable, Applicability: Applicable()},
			},
			want: []LogicalChange{{
				DesiredState: []configuration.ManagedSetting{{Unit: productionUnit, Desired: setting.Present()}, {Unit: productionReviewers, Desired: noReviewers}},
				Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: productionUnit, PreconditionBases: []PreconditionBasis{creationBasis}}},
			}},
		},
		{
			name: "mode and reviewers of one existing environment are two changes in the plan order",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingPolicy, Observed: github.Observed(all), Desired: protected, Result: Mismatch, Applicability: Applicable()},
				{Unit: stagingReviewers, Observed: github.Observed(user1), Desired: user2, Result: Mismatch, Applicability: Applicable()},
			},
			want: []LogicalChange{
				{
					DesiredState: []configuration.ManagedSetting{{Unit: stagingPolicy, Desired: protected}},
					Operations: []UpdateOperation{{
						Kind: DeploymentBranchPolicyModeUpdate, Unit: stagingPolicy, Mode: setting.DeploymentBranchPolicyProtectedBranches,
						PreconditionBases: []PreconditionBasis{stagingEnvironment, {Unit: stagingPolicy, AnticipatedValue: AnticipateValue(all)}},
					}},
				},
				reviewersChange(stagingReviewers, user2, stagingEnvironment, PreconditionBasis{Unit: stagingReviewers, AnticipatedValue: AnticipateValue(user1)}),
			},
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
		{
			name:     "reviewers without the plan of their environment panic",
			settings: []SettingPlan{{Unit: stagingReviewers, Observed: github.Observed(user1), Desired: user2, Result: Mismatch, Applicability: Applicable()}},
		},
		{
			name: "not comparable reviewers of an existing environment panic",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingReviewers, Observed: absentEnvironment, Desired: user1, Result: NotComparable, Applicability: Applicable()},
			},
		},
		{
			name: "mismatched reviewers of an environment to create panic",
			settings: []SettingPlan{
				productionCreation,
				{Unit: productionReviewers, Observed: github.Observed(user1), Desired: user2, Result: Mismatch, Applicability: Applicable()},
			},
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

// TestRepositoryPlan_LogicalChangesOfPreventSelfReviewAndWaitTimer は、自己承認の禁止とデプロイの待機時間の変更予定から導く論理的な設定変更と、
// その変更前提の基準を確かめる。
//
//trace:checks INV-001 INV-004 FR-010
func TestRepositoryPlan_LogicalChangesOfPreventSelfReviewAndWaitTimer(t *testing.T) {
	production, staging := mustEnvironmentName(t, "production"), mustEnvironmentName(t, "staging")
	stagingExistence := SettingPlan{
		Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: staging}, Observed: github.Observed(setting.Present()), Desired: setting.Present(),
		EnvironmentIdentity: ObservedEnvironmentIdentity(7), Result: Match,
	}
	stagingEnvironment := PreconditionBasis{Unit: stagingExistence.Unit, AnticipatedValue: AnticipateValue(setting.Present()), AnticipatedEnvironmentIdentity: ObservedEnvironmentIdentity(7)}
	productionUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	productionCreation := SettingPlan{Unit: productionUnit, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch, Applicability: Applicable()}
	creation := LogicalChange{
		DesiredState: []configuration.ManagedSetting{{Unit: productionUnit, Desired: setting.Present()}},
		Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: productionUnit, PreconditionBases: []PreconditionBasis{{Unit: productionUnit, AnticipatedValue: AnticipateValue(setting.Absent())}}}},
	}
	stagingReviewers := setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: staging}
	stagingPrevent := setting.Unit{Key: setting.EnvironmentPreventSelfReview, Environment: staging}
	stagingWait := setting.Unit{Key: setting.EnvironmentWaitTimer, Environment: staging}
	productionReviewers := setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: production}
	productionPrevent := setting.Unit{Key: setting.EnvironmentPreventSelfReview, Environment: production}
	productionWait := setting.Unit{Key: setting.EnvironmentWaitTimer, Environment: production}
	stagingVariable := environmentVariableUnit(staging, mustVariableName(t, "REGION"))
	user1 := setting.ReviewerSetValue(setting.ReviewerSetOf([]setting.Reviewer{{Type: setting.ReviewerUser, ID: 1}}))
	noReviewers := setting.ReviewerSetValue(setting.ReviewerSetOf(nil))
	prevented, allowed := setting.BoolValue(true), setting.BoolValue(false)
	absentEnvironment := github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent)
	change := func(kind UpdateOperationKind, unit setting.Unit, desired setting.Value, bases ...PreconditionBasis) LogicalChange {
		return LogicalChange{
			DesiredState: []configuration.ManagedSetting{{Unit: unit, Desired: desired}},
			Operations:   []UpdateOperation{{Kind: kind, Unit: unit, Value: desired, PreconditionBases: bases}},
		}
	}
	cases := []struct {
		name     string
		settings []SettingPlan
		want     []LogicalChange
	}{
		{
			name: "prevent_self_review mismatch of an existing environment is one update based on the environment and then its observed value",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingPrevent, Observed: github.Observed(allowed), Desired: prevented, Result: Mismatch, Applicability: Applicable()},
			},
			want: []LogicalChange{change(PreventSelfReviewUpdate, stagingPrevent, prevented, stagingEnvironment, PreconditionBasis{Unit: stagingPrevent, AnticipatedValue: AnticipateValue(allowed)})},
		},
		{
			name: "wait_timer mismatch of an existing environment is one update based on the environment and then its observed value",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingWait, Observed: github.Observed(setting.WaitTimerValue(0)), Desired: setting.WaitTimerValue(30), Result: Mismatch, Applicability: Applicable()},
			},
			want: []LogicalChange{change(WaitTimerUpdate, stagingWait, setting.WaitTimerValue(30), stagingEnvironment, PreconditionBasis{Unit: stagingWait, AnticipatedValue: AnticipateValue(setting.WaitTimerValue(0))})},
		},
		{
			// GitHub は担当者のいない Environment への prevent_self_review を拒むため、担当者の変更のあとに別の PUT で送る
			name: "reviewers, prevent_self_review, and wait_timer of one environment are three changes in this order before its variables",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingReviewers, Observed: github.Observed(noReviewers), Desired: user1, Result: Mismatch, Applicability: Applicable()},
				{Unit: stagingPrevent, Observed: github.Observed(allowed), Desired: prevented, Result: Mismatch, Applicability: Applicable()},
				{Unit: stagingWait, Observed: github.Observed(setting.WaitTimerValue(5)), Desired: setting.WaitTimerValue(0), Result: Mismatch, Applicability: Applicable()},
				{Unit: stagingVariable, Observed: github.Observed(setting.Absent()), Desired: setting.VariableValue("value"), Result: Mismatch, Applicability: Applicable()},
			},
			want: []LogicalChange{
				change(RequiredReviewersUpdate, stagingReviewers, user1, stagingEnvironment, PreconditionBasis{Unit: stagingReviewers, AnticipatedValue: AnticipateValue(noReviewers)}),
				change(PreventSelfReviewUpdate, stagingPrevent, prevented, stagingEnvironment, PreconditionBasis{Unit: stagingPrevent, AnticipatedValue: AnticipateValue(allowed)}),
				change(WaitTimerUpdate, stagingWait, setting.WaitTimerValue(0), stagingEnvironment, PreconditionBasis{Unit: stagingWait, AnticipatedValue: AnticipateValue(setting.WaitTimerValue(5))}),
				{
					DesiredState: []configuration.ManagedSetting{{Unit: stagingVariable, Desired: setting.VariableValue("value")}},
					Operations: []UpdateOperation{{
						Kind: EnvironmentVariableCreation, Unit: stagingVariable, Value: setting.VariableValue("value"),
						PreconditionBases: []PreconditionBasis{stagingEnvironment, {Unit: stagingVariable, AnticipatedValue: AnticipateValue(setting.Absent())}},
					}},
				},
			},
		},
		{
			// 作られたばかりの Environment は required_reviewers と wait_timer の保護ルールを持たないため、作成直後の値と同じ宣言は送らずに期待状態が成り立つ
			name: "prevent_self_review false and wait_timer 0 of an environment to create join the desired state of the creation without an update",
			settings: []SettingPlan{
				productionCreation,
				{Unit: productionPrevent, Observed: absentEnvironment, Desired: allowed, Result: NotComparable, Applicability: Applicable()},
				{Unit: productionWait, Observed: absentEnvironment, Desired: setting.WaitTimerValue(0), Result: NotComparable, Applicability: Applicable()},
			},
			want: []LogicalChange{{
				DesiredState: []configuration.ManagedSetting{{Unit: productionUnit, Desired: setting.Present()}, {Unit: productionPrevent, Desired: allowed}, {Unit: productionWait, Desired: setting.WaitTimerValue(0)}},
				Operations:   creation.Operations,
			}},
		},
		{
			name: "reviewers, prevent_self_review true, and wait_timer of an environment to create follow its creation as other changes based only on the values of a created environment",
			settings: []SettingPlan{
				productionCreation,
				{Unit: productionReviewers, Observed: absentEnvironment, Desired: user1, Result: NotComparable, Applicability: Applicable()},
				{Unit: productionPrevent, Observed: absentEnvironment, Desired: prevented, Result: NotComparable, Applicability: Applicable()},
				{Unit: productionWait, Observed: absentEnvironment, Desired: setting.WaitTimerValue(30), Result: NotComparable, Applicability: Applicable()},
			},
			want: []LogicalChange{
				creation,
				change(RequiredReviewersUpdate, productionReviewers, user1, PreconditionBasis{Unit: productionReviewers, AnticipatedValue: AnticipateValue(noReviewers)}),
				change(PreventSelfReviewUpdate, productionPrevent, prevented, PreconditionBasis{Unit: productionPrevent, AnticipatedValue: AnticipateValue(allowed)}),
				change(WaitTimerUpdate, productionWait, setting.WaitTimerValue(30), PreconditionBasis{Unit: productionWait, AnticipatedValue: AnticipateValue(setting.WaitTimerValue(0))}),
			},
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
		{
			name:     "prevent_self_review without the plan of its environment panics",
			settings: []SettingPlan{{Unit: stagingPrevent, Observed: github.Observed(allowed), Desired: prevented, Result: Mismatch, Applicability: Applicable()}},
		},
		{
			name: "not comparable wait_timer of an existing environment panics",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingWait, Observed: absentEnvironment, Desired: setting.WaitTimerValue(30), Result: NotComparable, Applicability: Applicable()},
			},
		},
		{
			// 担当者が予定どおりにいない状態から prevent_self_review が変わることは観測から生じず、送れば GitHub が拒むため、黙って落とさない
			name: "prevent_self_review mismatch with declared no reviewers that match panics",
			settings: []SettingPlan{
				stagingExistence,
				{Unit: stagingReviewers, Observed: github.Observed(noReviewers), Desired: noReviewers, Result: Match},
				{Unit: stagingPrevent, Observed: github.Observed(prevented), Desired: allowed, Result: Mismatch, Applicability: Applicable()},
			},
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
	mismatch := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable()}
	all := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll})
	protected := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches})
	creation := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch, Applicability: Applicable()}
	creationMode := SettingPlan{
		Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
		Desired: protected, Result: NotComparable, Applicability: Applicable(),
	}
	existing := SettingPlan{
		Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: staging}, Observed: github.Observed(setting.Present()), Desired: setting.Present(),
		EnvironmentIdentity: ObservedEnvironmentIdentity(7), Result: Match,
	}
	modeChange := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}, Observed: github.Observed(all), Desired: protected, Result: Mismatch, Applicability: Applicable()}
	newRepositoryPlan := func(settings ...SettingPlan) RepositoryPlan {
		return RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}
	productionUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	stagingUnit := setting.Unit{Key: setting.EnvironmentResource, Environment: staging}
	stagingPolicyUnit := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging}
	productionPolicyUnit := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}
	allowAutoMerge := LogicalChange{
		DesiredState: []configuration.ManagedSetting{{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Desired: setting.BoolValue(true)}},
		Operations: []UpdateOperation{{
			Kind: SettingUpdate, Unit: setting.Unit{Key: setting.AllowAutoMerge}, Value: setting.BoolValue(true),
			PreconditionBases: []PreconditionBasis{{Unit: setting.Unit{Key: setting.AllowAutoMerge}, AnticipatedValue: AnticipateValue(setting.BoolValue(false))}},
		}},
	}
	// 作成の基準は存在しないことだけで、方式を宣言していても方式の基準は並ばない
	productionAbsent := []PreconditionBasis{{Unit: productionUnit, AnticipatedValue: AnticipateValue(setting.Absent())}}
	createProduction := LogicalChange{
		DesiredState: []configuration.ManagedSetting{{Unit: productionUnit, Desired: setting.Present()}},
		Operations:   []UpdateOperation{{Kind: EnvironmentCreation, Unit: productionUnit, PreconditionBases: productionAbsent}},
	}
	// createProductionWithMode は、デプロイ元制限を desired と宣言した作成の、期待状態と、方式を送る作成の更新操作に、operations を続けたもの。
	createProductionWithMode := func(desired setting.Value, operations ...UpdateOperation) LogicalChange {
		creation := UpdateOperation{Kind: EnvironmentCreationWithMode, Unit: productionUnit, Mode: desired.DeploymentBranchPolicy().Mode, PreconditionBases: productionAbsent}
		return LogicalChange{
			DesiredState: []configuration.ManagedSetting{{Unit: productionUnit, Desired: setting.Present()}, {Unit: productionPolicyUnit, Desired: desired}},
			Operations:   append([]UpdateOperation{creation}, operations...),
		}
	}
	// stagingPolicyChange は、存在する staging のデプロイ元制限を desired にする期待状態と、それを実現する operations からなる論理的な設定変更。
	stagingPolicyChange := func(desired setting.Value, operations ...UpdateOperation) LogicalChange {
		return LogicalChange{DesiredState: []configuration.ManagedSetting{{Unit: stagingPolicyUnit, Desired: desired}}, Operations: operations}
	}
	stagingEnvironment := PreconditionBasis{Unit: stagingUnit, AnticipatedValue: AnticipateValue(setting.Present()), AnticipatedEnvironmentIdentity: ObservedEnvironmentIdentity(7)}
	// stagingModeUpdate は、計画時に observed だった staging の方式を desired の方式にする更新操作。
	// 方式の変更の基準は、Environment が存在し ID が同じこと、方式が計画時の観測値と同じことの順に並ぶ
	stagingModeUpdate := func(observed, desired setting.Value) UpdateOperation {
		return UpdateOperation{
			Kind: DeploymentBranchPolicyModeUpdate, Unit: stagingPolicyUnit, Mode: desired.DeploymentBranchPolicy().Mode,
			PreconditionBases: []PreconditionBasis{stagingEnvironment, {Unit: stagingPolicyUnit, AnticipatedValue: AnticipateValue(observed)}},
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
		return SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}, Observed: github.Observed(observed), Desired: desired, Result: result, Applicability: Applicable()}
	}
	creationSelected := func(environment setting.EnvironmentName, desired setting.Value) SettingPlan {
		return SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent),
			Desired: desired, Result: NotComparable, Applicability: Applicable(),
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
				PreconditionBases: []PreconditionBasis{stagingEnvironment, {Unit: stagingPolicyUnit, AnticipatedValue: AnticipateValue(step.base)}},
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
				PreconditionBases: []PreconditionBasis{{Unit: productionUnit, AnticipatedValue: AnticipateValue(setting.Present()), AnticipatedEnvironmentIdentity: CreatedByPrecedingOperation()}, {Unit: productionPolicyUnit, AnticipatedValue: AnticipateValue(step.base)}},
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
			settings: []SettingPlan{creation, {Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(all), Desired: protected, Result: Mismatch, Applicability: Applicable()}},
		},
		{
			name:     "creation followed by a matched mode of the same name panics",
			settings: []SettingPlan{creation, {Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(protected), Desired: protected, Result: Match}},
		},
		{
			name: "applicable not comparable existence panics",
			settings: []SettingPlan{{
				Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined), Desired: setting.Present(),
				Result: NotComparable, Applicability: Applicable(),
			}},
		},
		{
			name: "applicable mode not comparable for another reason panics",
			settings: []SettingPlan{creation, {
				Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined),
				Desired: protected, Result: NotComparable, Applicability: Applicable(),
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
		undetermined := SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}, Result: NotComparable, Applicability: NotApplicable(NotApplicableNameUndetermined)}
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
		DesiredState:       []configuration.ManagedSetting{{Unit: setting.Unit{Key: setting.SecretScanningEnabled}}},
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
			Result: Mismatch, Applicability: Applicable(),
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
		Result: Mismatch, Applicability: Applicable(),
	}
	dependabotUpdatesDisabled := SettingPlan{
		Unit:     setting.Unit{Key: setting.DependabotSecurityUpdates},
		Observed: github.Observed(setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesEnabled)),
		Desired:  setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesDisabled), Result: Mismatch, Applicability: Applicable(),
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
		{
			name:     "a dependent setting already before its prerequisite keeps its position",
			settings: []SettingPlan{dependabotUpdatesDisabled, dependabotAlertsDisabled},
			want:     []setting.Key{setting.DependabotSecurityUpdates, setting.DependabotAlerts},
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

	self := func(s SettingPlan) PreconditionBasis {
		return PreconditionBasis{Unit: s.Unit, AnticipatedValue: AnticipateUnchanged(s.Observed)}
	}
	enabling := mismatch(setting.RepositoryPushProtection, false, true)
	disabling := mismatch(setting.RepositoryPushProtection, true, false)
	changes := []struct {
		name      string
		plan      SettingPlan
		wantBases []PreconditionBasis
	}{
		{
			name:      "enabling push protection is relevant to secret scanning being enabled",
			plan:      enabling,
			wantBases: []PreconditionBasis{self(enabling), {Unit: setting.Unit{Key: setting.SecretScanningEnabled}, AnticipatedValue: AnticipateValue(setting.BoolValue(true))}},
		},
		{
			name:      "disabling push protection is relevant only to itself",
			plan:      disabling,
			wantBases: []PreconditionBasis{self(disabling)},
		},
	}
	for _, tc := range changes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := newRepositoryPlan(tc.plan).LogicalChanges()
			if err != nil {
				t.Fatalf("LogicalChanges() error = %v", err)
			}
			want := []LogicalChange{{
				DesiredState: []configuration.ManagedSetting{{Unit: tc.plan.Unit, Desired: tc.plan.Desired}},
				Operations:   []UpdateOperation{{Kind: SettingUpdate, Unit: tc.plan.Unit, Value: tc.plan.Desired, PreconditionBases: tc.wantBases}},
			}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
			}
		})
	}
}

// TestRepositoryPlan_LogicalChangesOfMergeMessages は、マージメッセージの変更予定をそれぞれ一つの論理的な設定変更にし、
// 対応するマージ方式を無効にする変更予定があるときだけ、マージ方式をまとめた変更より先に実行することを確かめる (General の設計の論理的な設定変更と更新操作)。
//
//trace:checks FR-010 INV-001
func TestRepositoryPlan_LogicalChangesOfMergeMessages(t *testing.T) {
	squashMessage := SettingPlan{
		Unit:     setting.Unit{Key: setting.SquashMergeCommit},
		Observed: github.Observed(setting.SquashMergeCommitDefaultValue(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitleCommitOrPRTitle, Message: setting.SquashMergeCommitMessageCommitMessages})),
		Desired:  setting.SquashMergeCommitDefaultValue(setting.SquashMergeCommitDefault{Title: setting.SquashMergeCommitTitlePRTitle, Message: setting.SquashMergeCommitMessagePRBody}),
		Result:   Mismatch, Applicability: Applicable(),
	}
	mergeCommitMessage := SettingPlan{
		Unit:     setting.Unit{Key: setting.MergeCommit},
		Observed: github.Observed(setting.MergeCommitDefaultValue(setting.MergeCommitDefault{Title: setting.MergeCommitTitleMergeMessage, Message: setting.MergeCommitMessagePRTitle})),
		Desired:  setting.MergeCommitDefaultValue(setting.MergeCommitDefault{Title: setting.MergeCommitTitlePRTitle, Message: setting.MergeCommitMessagePRBody}),
		Result:   Mismatch, Applicability: Applicable(),
	}
	boolPlan := func(key setting.Key, observed, desired bool) SettingPlan {
		plan := SettingPlan{Unit: setting.Unit{Key: key}, Observed: github.Observed(setting.BoolValue(observed)), Desired: setting.BoolValue(desired), Result: Match}
		if observed != desired {
			plan.Result, plan.Applicability = Mismatch, Applicable()
		}
		return plan
	}
	// keys は、論理的な設定変更を、それぞれが最初に変える管理設定で表す。マージ方式をまとめた変更は、まとめたうちで最初のマージ方式で表す。
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
			name:     "two merge messages are separate logical changes in the table order",
			settings: []SettingPlan{squashMessage, mergeCommitMessage},
			want:     []setting.Key{setting.SquashMergeCommit, setting.MergeCommit},
		},
		{
			name:     "enabling squash merge runs the merge methods before the squash merge message",
			settings: []SettingPlan{boolPlan(setting.AllowSquashMerge, false, true), squashMessage},
			want:     []setting.Key{setting.AllowSquashMerge, setting.SquashMergeCommit},
		},
		{
			name:     "disabling squash merge runs the squash merge message before the merge methods",
			settings: []SettingPlan{boolPlan(setting.AllowSquashMerge, true, false), squashMessage},
			want:     []setting.Key{setting.SquashMergeCommit, setting.AllowSquashMerge},
		},
		{
			name:     "squash merge declared true and matching keeps the table order",
			settings: []SettingPlan{boolPlan(setting.AllowSquashMerge, true, true), boolPlan(setting.AllowRebaseMerge, true, false), squashMessage},
			want:     []setting.Key{setting.AllowRebaseMerge, setting.SquashMergeCommit},
		},
		{
			name: "disabling merge commit moves only its message before the merge methods bundled at squash merge",
			settings: []SettingPlan{
				boolPlan(setting.AllowSquashMerge, false, true), boolPlan(setting.AllowMergeCommit, true, false), squashMessage, mergeCommitMessage,
			},
			want: []setting.Key{setting.MergeCommit, setting.AllowSquashMerge, setting.SquashMergeCommit},
		},
		{
			name: "disabling both merge methods runs both messages in the table order before the merge methods",
			settings: []SettingPlan{
				boolPlan(setting.AllowSquashMerge, true, false), boolPlan(setting.AllowMergeCommit, true, false), boolPlan(setting.AllowRebaseMerge, true, true), squashMessage, mergeCommitMessage,
			},
			want: []setting.Key{setting.SquashMergeCommit, setting.MergeCommit, setting.AllowSquashMerge},
		},
		{
			name: "disabling squash merge runs its message before the merge methods and allow_update_branch after them",
			settings: []SettingPlan{
				boolPlan(setting.AllowSquashMerge, true, false), boolPlan(setting.AllowUpdateBranch, false, true), squashMessage,
			},
			want: []setting.Key{setting.SquashMergeCommit, setting.AllowSquashMerge, setting.AllowUpdateBranch},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: tc.settings}.LogicalChanges()
			if err != nil {
				t.Fatalf("LogicalChanges() error = %v", err)
			}
			if !reflect.DeepEqual(keys(got), tc.want) {
				t.Errorf("LogicalChanges() order = %v, want %v", keys(got), tc.want)
			}
		})
	}

	t.Run("a merge message change sends its desired value and is based only on its own observation", func(t *testing.T) {
		got, err := RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: []SettingPlan{squashMessage}}.LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{
			DesiredState: []configuration.ManagedSetting{{Unit: squashMessage.Unit, Desired: squashMessage.Desired}},
			Operations: []UpdateOperation{{
				Kind: SettingUpdate, Unit: squashMessage.Unit, Value: squashMessage.Desired,
				PreconditionBases: []PreconditionBasis{{Unit: squashMessage.Unit, AnticipatedValue: AnticipateUnchanged(squashMessage.Observed)}},
			}},
		}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})
}

//trace:checks FR-011 INV-001 INV-004
func TestRepositoryPlan_LogicalChangesOfDependabot(t *testing.T) {
	newRepositoryPlan := func(settings ...SettingPlan) RepositoryPlan {
		return RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}
	updatesValue := func(state setting.DependabotSecurityUpdatesState) setting.Value {
		return setting.DependabotSecurityUpdatesValue(state)
	}
	alerts := func(observed, desired bool, unaffected ...SettingExpectedUnaffected) SettingPlan {
		return SettingPlan{
			Unit: setting.Unit{Key: setting.DependabotAlerts}, Observed: github.Observed(setting.BoolValue(observed)), Desired: setting.BoolValue(desired),
			Result: Mismatch, Applicability: Applicable(), SettingsExpectedUnaffected: unaffected,
		}
	}
	updates := func(observed, desired setting.DependabotSecurityUpdatesState) SettingPlan {
		result := Mismatch
		if updatesValue(observed).Equal(updatesValue(desired)) {
			result = Match
		}
		plan := SettingPlan{Unit: setting.Unit{Key: setting.DependabotSecurityUpdates}, Observed: github.Observed(updatesValue(observed)), Desired: updatesValue(desired), Result: result}
		if result == Mismatch {
			plan.Applicability = Applicable()
		}
		return plan
	}
	// changeOf は、計画 s の期待値を送る更新操作一つからなる論理的な設定変更で、基準は s の計画時の観測値と extra である。
	changeOf := func(s SettingPlan, extra ...PreconditionBasis) LogicalChange {
		bases := append([]PreconditionBasis{{Unit: s.Unit, AnticipatedValue: AnticipateUnchanged(s.Observed)}}, extra...)
		return LogicalChange{
			DesiredState: []configuration.ManagedSetting{{Unit: s.Unit, Desired: s.Desired}},
			Operations:   []UpdateOperation{{Kind: SettingUpdate, Unit: s.Unit, Value: s.Desired, PreconditionBases: bases}},
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
		disabledUpdates := PreconditionBasis{Unit: setting.Unit{Key: setting.DependabotSecurityUpdates}, AnticipatedValue: AnticipateUnchanged(disabled)}
		expectedUnaffected := alerts(true, false, SettingExpectedUnaffected{Key: setting.DependabotSecurityUpdates, Observed: disabled})
		expectedUnaffected.SideEffects = []SideEffect{{Kind: SideEffectDependencyGraphDisabled}}
		for _, tc := range []struct {
			plan SettingPlan
			want LogicalChange
		}{
			{plan: withSideEffect, want: withSideEffectSettings(changeOf(withSideEffect, PreconditionBasis{Unit: setting.Unit{Key: setting.DependabotSecurityUpdates}, AnticipatedValue: AnticipateUnchanged(paused)}), setting.Unit{Key: setting.DependabotSecurityUpdates})},
			{plan: expectedUnaffected, want: changeOf(expectedUnaffected, disabledUpdates)},
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

// TestPlan_LogicalChanges は、変更計画の全体から論理的な設定変更を、承認の前に導き切る規則を確かめる。
//
//trace:checks INV-002 FR-009
func TestPlan_LogicalChanges(t *testing.T) {
	match := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match}
	mismatch := SettingPlan{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable()}
	notApplicable := mismatch
	notApplicable.Applicability = NotApplicable(NotApplicablePublicRepository)
	plan := func(settings ...[]SettingPlan) Plan {
		var p Plan
		for i, s := range settings {
			p.Repositories = append(p.Repositories, RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: repository.ID(i + 1), Settings: s})
		}
		return p
	}

	t.Run("each repository plan has its logical changes at the same position, empty without planned changes", func(t *testing.T) {
		p := plan([]SettingPlan{mismatch}, []SettingPlan{match}, []SettingPlan{mismatch})
		got, err := p.LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		first, _ := p.Repositories[0].LogicalChanges()
		want := [][]LogicalChange{first, nil, first}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("not applicable planned change in the second repository returns an error", func(t *testing.T) {
		if got, err := plan([]SettingPlan{mismatch}, []SettingPlan{notApplicable}).LogicalChanges(); err == nil {
			t.Errorf("LogicalChanges() = %+v, want an error", got)
		}
	})
}

// TestCheckUpdateEndpoint は、更新操作の更新要求の宛先を承認の前に決められるかの規則を確かめる。
//
//trace:checks INV-002 INV-004 INV-005
func TestCheckUpdateEndpoint(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	squash := configuration.ManagedSetting{Unit: setting.Unit{Key: setting.AllowSquashMerge}, Desired: setting.BoolValue(true)}
	rebase := configuration.ManagedSetting{Unit: setting.Unit{Key: setting.AllowRebaseMerge}, Desired: setting.BoolValue(false)}
	autoMerge := configuration.ManagedSetting{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Desired: setting.BoolValue(true)}

	passing := []struct {
		name      string
		operation UpdateOperation
	}{
		{name: "setting update with an update endpoint passes", operation: UpdateOperation{Kind: SettingUpdate, Unit: autoMerge.Unit, Value: autoMerge.Desired}},
		{name: "merge methods update sending only merge methods passes", operation: UpdateOperation{Kind: MergeMethodsUpdate, Unit: squash.Unit, Settings: []configuration.ManagedSetting{squash, rebase}}},
		{name: "environment creation passes without an endpoint of a setting", operation: UpdateOperation{Kind: EnvironmentCreation, Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: production}}},
		{
			name: "ruleset deletion based on an observed ID passes without looking up an endpoint",
			operation: UpdateOperation{
				Kind: RulesetDeletion, Unit: rulesetUnit(t, "protect"),
				PreconditionBases: []PreconditionBasis{{Unit: rulesetUnit(t, "protect"), AnticipatedRulesetIdentity: ObservedRulesetIdentity(42)}},
			},
		},
		{
			name: "ruleset value update based on an observed ID passes without looking up an endpoint",
			operation: UpdateOperation{
				Kind: RulesetValueUpdate, Unit: rulesetUnit(t, "protect"), Value: setting.RulesetValue(protectRuleset()),
				PreconditionBases: []PreconditionBasis{{Unit: rulesetUnit(t, "protect"), AnticipatedRulesetIdentity: ObservedRulesetIdentity(42)}},
			},
		},
	}
	for _, tc := range passing {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("checkUpdateEndpoint() panicked: %v", r)
				}
			}()
			checkUpdateEndpoint(tc.operation)
		})
	}

	panicking := []struct {
		name      string
		operation UpdateOperation
	}{
		{name: "ruleset value update without an observed ID panics", operation: UpdateOperation{Kind: RulesetValueUpdate, Unit: rulesetUnit(t, "protect"), Value: setting.RulesetValue(protectRuleset())}},
		{name: "ruleset deletion without an observed ID panics", operation: UpdateOperation{Kind: RulesetDeletion, Unit: rulesetUnit(t, "protect")}},
		{name: "setting without an update request panics", operation: UpdateOperation{Kind: SettingUpdate, Unit: setting.Unit{Key: "general.unknown"}}},
		{name: "merge method updated on its own panics", operation: UpdateOperation{Kind: SettingUpdate, Unit: squash.Unit, Value: squash.Desired}},
		{name: "merge methods update without a setting to send panics", operation: UpdateOperation{Kind: MergeMethodsUpdate, Unit: squash.Unit}},
		{name: "merge methods update with a setting other than a merge method panics", operation: UpdateOperation{Kind: MergeMethodsUpdate, Unit: squash.Unit, Settings: []configuration.ManagedSetting{squash, autoMerge}}},
	}
	for _, tc := range panicking {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("checkUpdateEndpoint() did not panic")
				}
			}()
			checkUpdateEndpoint(tc.operation)
		})
	}
}
