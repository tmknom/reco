package apply

import (
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// accessLevelChange は、access_level を none から user にする論理的な設定変更。
func accessLevelChange() changeplan.LogicalChange {
	return singleUpdate(setting.ActionsAccessLevel, setting.AccessLevelValue(setting.AccessUser), relevantTo(setting.ActionsAccessLevel, setting.AccessLevelValue(setting.AccessNone)))
}

// allowAutoMergeChange は、allow_auto_merge を false から true にする論理的な設定変更。
func allowAutoMergeChange() changeplan.LogicalChange {
	return singleUpdate(setting.AllowAutoMerge, setting.BoolValue(true), relevantTo(setting.AllowAutoMerge, setting.BoolValue(false)))
}

// deleteBranchOnMergeChange は、delete_branch_on_merge を false から true にする論理的な設定変更。
func deleteBranchOnMergeChange() changeplan.LogicalChange {
	return singleUpdate(setting.DeleteBranchOnMerge, setting.BoolValue(true), relevantTo(setting.DeleteBranchOnMerge, setting.BoolValue(false)))
}

// defaultWorkflowPermissionsChange は、default_workflow_permissions を write から read にする論理的な設定変更。
func defaultWorkflowPermissionsChange() changeplan.LogicalChange {
	return singleUpdate(
		setting.DefaultWorkflowPermissions,
		setting.PermissionLevelValue(setting.PermissionRead),
		relevantTo(setting.DefaultWorkflowPermissions, setting.PermissionLevelValue(setting.PermissionWrite)),
	)
}

// canApprovePullRequestReviewsChange は、can_approve_pull_request_reviews を true から false にする論理的な設定変更。
func canApprovePullRequestReviewsChange() changeplan.LogicalChange {
	return singleUpdate(setting.CanApprovePullRequestReviews, setting.BoolValue(false), relevantTo(setting.CanApprovePullRequestReviews, setting.BoolValue(true)))
}

// allowedActionsChange は、Actions 許可ポリシーを all から local_only にする、副作用のない論理的な設定変更。
func allowedActionsChange() changeplan.LogicalChange {
	return singleUpdate(
		setting.AllowedActions,
		setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly),
		relevantTo(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsAll)),
	)
}

// selectedActionsChange は、Actions 許可ポリシーが selected のまま、選択方式の Actions 許可条件の verified_allowed だけを
// false から true にする論理的な設定変更。許可パターンは2つ持ち、順序だけの違いを作れるようにする。
func selectedActionsChange() changeplan.LogicalChange {
	return singleUpdate(
		setting.SelectedActions,
		setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{VerifiedAllowed: true, PatternsAllowed: []string{"a/*", "b/*"}}),
		relevantTo(setting.SelectedActions, setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{PatternsAllowed: []string{"a/*", "b/*"}})),
	)
}

// enableActionsChange は、enableActionsNotComparable から導く論理的な設定変更。基準はどちらも Actions が無効による観測不能。
func enableActionsChange() changeplan.LogicalChange {
	return singleUpdate(setting.AllowedActions, setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly), []changeplan.RelevantSetting{
		{Unit: setting.Unit{Key: setting.AllowedActions}, AnticipatedValue: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)},
		{Unit: setting.Unit{Key: setting.SelectedActions}, AnticipatedValue: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)},
	})
}

// enabledStateChange は、Actions の有効状態を to にする論理的な設定変更。基準は有効状態の反対の観測値だけである。
func enabledStateChange(to bool) changeplan.LogicalChange {
	return singleUpdate(setting.ActionsEnabled, setting.BoolValue(to), relevantTo(setting.ActionsEnabled, setting.BoolValue(!to)))
}

// shaPinningRequiredChange は、Actions が有効のリポジトリで sha_pinning_required を false から true にする論理的な設定変更。
func shaPinningRequiredChange() changeplan.LogicalChange {
	return singleUpdate(setting.SHAPinningRequired, setting.BoolValue(true), []changeplan.RelevantSetting{
		{Unit: setting.Unit{Key: setting.SHAPinningRequired}, AnticipatedValue: github.Observed(setting.BoolValue(false))},
		{Unit: setting.Unit{Key: setting.ActionsEnabled}, AnticipatedValue: github.Observed(setting.BoolValue(true))},
	})
}

// shaPinningRequiredAfterEnablingChange は、Actions が無効のリポジトリで、Actions を有効にする変更のあとに sha_pinning_required を true にする論理的な設定変更。
func shaPinningRequiredAfterEnablingChange() changeplan.LogicalChange {
	return singleUpdate(setting.SHAPinningRequired, setting.BoolValue(true), relevantTo(setting.ActionsEnabled, setting.BoolValue(true)))
}

// creationChange は、計画時に存在しなかった name の Environment を、デプロイ元制限を送らずに作成する論理的な設定変更。
func creationChange(t *testing.T, name string) changeplan.LogicalChange {
	t.Helper()
	environment := mustEnvironmentName(t, name)
	unit := setting.Unit{Key: setting.EnvironmentResource, Environment: environment}
	return changeplan.LogicalChange{
		DesiredState: []changeplan.DesiredSetting{{Unit: unit, Value: setting.Present()}},
		Operations: []changeplan.UpdateOperation{{
			Kind: changeplan.EnvironmentCreation, Unit: unit,
			RelevantSettings: []changeplan.RelevantSetting{{Unit: unit, AnticipatedValue: github.Observed(setting.Absent())}},
		}},
	}
}

// creationWithModeChange は、計画時に存在しなかった name の Environment を、デプロイ元制限の方式を mode にして作成する論理的な設定変更。
func creationWithModeChange(t *testing.T, name string, mode setting.DeploymentBranchPolicyMode) changeplan.LogicalChange {
	t.Helper()
	environment := mustEnvironmentName(t, name)
	unit := setting.Unit{Key: setting.EnvironmentResource, Environment: environment}
	return changeplan.LogicalChange{
		DesiredState: []changeplan.DesiredSetting{{Unit: unit, Value: setting.Present()}, {Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}, Value: modeValue(mode)}},
		Operations: []changeplan.UpdateOperation{{
			Kind: changeplan.EnvironmentCreationWithMode, Unit: unit, Mode: mode,
			RelevantSettings: []changeplan.RelevantSetting{{Unit: unit, AnticipatedValue: github.Observed(setting.Absent())}},
		}},
	}
}

// modeChange は、計画時に approved と観測した、宣言の名前 name の Environment の方式を desired にする論理的な設定変更。
func modeChange(t *testing.T, name string, approved github.Environment, desired setting.DeploymentBranchPolicyMode) changeplan.LogicalChange {
	t.Helper()
	environment := mustEnvironmentName(t, name)
	policy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}
	return changeplan.LogicalChange{DesiredState: []changeplan.DesiredSetting{{Unit: policy, Value: modeValue(desired)}}, Operations: []changeplan.UpdateOperation{{
		Kind: changeplan.DeploymentBranchPolicyModeUpdate, Unit: policy, Mode: desired,
		RelevantSettings: []changeplan.RelevantSetting{
			{
				Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: environment}, AnticipatedValue: github.Observed(setting.Present()),
				AnticipatedEnvironmentIdentity: changeplan.ObservedEnvironmentIdentity(approved.ID),
			},
			{Unit: policy, AnticipatedValue: github.Observed(setting.DeploymentBranchPolicyValue(approved.DeploymentBranchPolicy()))},
		},
	}}}
}

func newRepositoryPlan(t *testing.T, settings ...changeplan.SettingPlan) changeplan.RepositoryPlan {
	t.Helper()
	return newRepositoryPlanOf(t, "owner/repo", 1, settings...)
}

// newRepositoryPlanOf は、name の対象リポジトリを ID id と観測した、リポジトリの計画。
func newRepositoryPlanOf(t *testing.T, name string, id repository.ID, settings ...changeplan.SettingPlan) changeplan.RepositoryPlan {
	t.Helper()
	return changeplan.RepositoryPlan{Target: mustReference(t, name), RepositoryID: id, Settings: settings}
}

// relevantTo は、key の設定だけを関係する現在値の設定とし、承認時に予定した値を観測値 observed とする変更前提の基準。
func relevantTo(key setting.Key, observed setting.Value) []changeplan.RelevantSetting {
	return []changeplan.RelevantSetting{{Unit: setting.Unit{Key: key}, AnticipatedValue: github.Observed(observed)}}
}

// singleUpdate は、key を desired にする更新操作一つからなる論理的な設定変更。
func singleUpdate(key setting.Key, desired setting.Value, relevant []changeplan.RelevantSetting) changeplan.LogicalChange {
	unit := setting.Unit{Key: key}
	return changeplan.LogicalChange{
		DesiredState: []changeplan.DesiredSetting{{Unit: unit, Value: desired}},
		Operations:   []changeplan.UpdateOperation{{Kind: changeplan.SettingUpdate, Unit: unit, Value: desired, RelevantSettings: relevant}},
	}
}

// deploymentPatternsChange は、計画時に approved と観測した、宣言の名前 name の Environment のデプロイ元制限を desired にする論理的な設定変更。
// 更新操作ごとに進む基準を手で写さず、changeplan が導いた並びを使う。
func deploymentPatternsChange(t *testing.T, name string, approved github.Environment, desired setting.DeploymentBranchPolicy) changeplan.LogicalChange {
	t.Helper()
	environment := mustEnvironmentName(t, name)
	observed := github.Environments{Existing: []github.Environment{approved}}.Lookup(environment)
	return onlyLogicalChange(t, newRepositoryPlan(t,
		changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: environment}, Observed: observed.Existence(), Desired: setting.Present(),
			EnvironmentIdentity: changeplan.ObservedEnvironmentIdentity(approved.ID), Result: changeplan.Match, Applicability: changeplan.Applicable(),
		},
		changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}, Observed: observed.DeploymentBranchPolicy(),
			Desired: setting.DeploymentBranchPolicyValue(desired), Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
		},
	))
}

// creationWithPatternsChange は、計画時に存在しなかった name の Environment を、デプロイ元制限を desired にして作成する論理的な設定変更。
// 更新操作ごとに進む基準を手で写さず、changeplan が導いた並びを使う。
func creationWithPatternsChange(t *testing.T, name string, desired setting.DeploymentBranchPolicy) changeplan.LogicalChange {
	t.Helper()
	environment := mustEnvironmentName(t, name)
	observed := github.Environments{}.Lookup(environment)
	return onlyLogicalChange(t, newRepositoryPlan(t,
		changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: environment}, Observed: observed.Existence(), Desired: setting.Present(),
			Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
		},
		changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}, Observed: observed.DeploymentBranchPolicy(),
			Desired: setting.DeploymentBranchPolicyValue(desired), Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
		},
	))
}

// mergeMethodMismatch は、マージ方式 key を計画時の観測値 observed から desired にする、適用可能な変更予定。
func mergeMethodMismatch(key setting.Key, observed, desired bool) changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Unit: setting.Unit{Key: key}, Observed: github.Observed(setting.BoolValue(observed)), Desired: setting.BoolValue(desired),
		Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
	}
}

// mergeMethodSwapChange は、merge commit だけが有効のリポジトリで、有効なマージ方式を squash merge に入れ替える論理的な設定変更。
// まとめた更新操作を手で写さず、changeplan がまとめたものを使う。
func mergeMethodSwapChange(t *testing.T) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t, mergeMethodMismatch(setting.AllowSquashMerge, false, true), mergeMethodMismatch(setting.AllowMergeCommit, true, false)))
}

// onlyLogicalChange は、r から導いた論理的な設定変更がちょうど一つであるときの、その論理的な設定変更。
func onlyLogicalChange(t *testing.T, r changeplan.RepositoryPlan) changeplan.LogicalChange {
	t.Helper()
	changes, err := r.LogicalChanges()
	if err != nil || len(changes) != 1 {
		t.Fatalf("LogicalChanges() = %+v, %v, want one logical change", changes, err)
	}
	return changes[0]
}

// variableMismatch は、計画時に observed と観測した、宣言の名前 name の Repository Variable を、値 desired にする変更予定。
func variableMismatch(t *testing.T, name string, observed setting.Value, desired string) changeplan.SettingPlan {
	t.Helper()
	return changeplan.SettingPlan{
		Unit: setting.Unit{Key: setting.RepositoryVariable, Variable: mustVariableName(t, name)}, Observed: github.Observed(observed),
		Desired: setting.VariableValue(desired), Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
	}
}

// variableCreationChange は、計画時に存在しなかった name の Repository Variable を、値 desired で作成する論理的な設定変更。
// 変更前提の基準を手で写さず、changeplan が導いた論理的な設定変更を使う。
func variableCreationChange(t *testing.T, name, desired string) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t, variableMismatch(t, name, setting.Absent(), desired)))
}

// variableValueChange は、計画時に値 observed と観測した name の Repository Variable の値を desired にする論理的な設定変更。
// 変更前提の基準を手で写さず、changeplan が導いた論理的な設定変更を使う。
func variableValueChange(t *testing.T, name, observed, desired string) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t, variableMismatch(t, name, setting.VariableValue(observed), desired)))
}

// variableDeletionChange は、計画時に値 observed と観測した name の Repository Variable を、存在しないことの宣言に基づいて削除する論理的な設定変更。
// 期待値が存在しないことで、値を持つ期待値の変更予定とは作る状態が違うため、variableMismatch を使わない。
// 変更前提の基準を手で写さず、changeplan が導いた論理的な設定変更を使う。
func variableDeletionChange(t *testing.T, name, observed string) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t, changeplan.SettingPlan{
		Unit: setting.Unit{Key: setting.RepositoryVariable, Variable: mustVariableName(t, name)}, Observed: github.Observed(setting.VariableValue(observed)),
		Desired: setting.Absent(), Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
	}))
}

// environmentVariablePlan は、宣言の名前 environment の Environment の、宣言の名前 name の Environment Variable を、
// 計画時の観測結果 observed から値 desired にする変更予定。result は observed から決まる照合結果である。
func environmentVariablePlan(t *testing.T, environment, name string, observed github.SettingObservation[setting.Value], desired string, result changeplan.ReconciliationResult) changeplan.SettingPlan {
	t.Helper()
	return changeplan.SettingPlan{
		Unit:     setting.Unit{Key: setting.EnvironmentVariable, Environment: mustEnvironmentName(t, environment), Variable: mustVariableName(t, name)},
		Observed: observed, Desired: setting.VariableValue(desired), Result: result, Applicability: changeplan.Applicable(),
	}
}

// environmentVariableValueChange は、計画時に approved と観測した、宣言の名前 environment の Environment の Environment Variable name の値を、
// 計画時の値 observed から desired にする論理的な設定変更。approved は name を値 observed で持つ。
// 変更前提の基準を手で写さず、changeplan が導いた論理的な設定変更を使う。
func environmentVariableValueChange(t *testing.T, environment string, approved github.Environment, name, observed, desired string) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t,
		changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, environment)}, Observed: github.Observed(setting.Present()), Desired: setting.Present(),
			EnvironmentIdentity: changeplan.ObservedEnvironmentIdentity(approved.ID), Result: changeplan.Match, Applicability: changeplan.Applicable(),
		},
		environmentVariablePlan(t, environment, name, github.Observed(setting.VariableValue(observed)), desired, changeplan.Mismatch),
	))
}

// environmentVariableDeletionChange は、計画時に approved と観測した、宣言の名前 environment の Environment の Environment Variable name を、
// 計画時の値 observed から存在しないことの宣言に基づいて削除する論理的な設定変更。approved は name を値 observed で持つ。
// 変更前提の基準を手で写さず、changeplan が導いた論理的な設定変更を使う。
func environmentVariableDeletionChange(t *testing.T, environment string, approved github.Environment, name, observed string) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t,
		changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, environment)}, Observed: github.Observed(setting.Present()), Desired: setting.Present(),
			EnvironmentIdentity: changeplan.ObservedEnvironmentIdentity(approved.ID), Result: changeplan.Match, Applicability: changeplan.Applicable(),
		},
		changeplan.SettingPlan{
			Unit:     setting.Unit{Key: setting.EnvironmentVariable, Environment: mustEnvironmentName(t, environment), Variable: mustVariableName(t, name)},
			Observed: github.Observed(setting.VariableValue(observed)), Desired: setting.Absent(), Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
		},
	))
}

// environmentCreationWithAbsentVariableChange は、計画時に存在しなかった environment の Environment を作成し、その中で存在しないことを宣言した Environment Variable name が
// 作成のあとに存在しないことを確かめる論理的な設定変更。変数への更新操作は持たない。
// 期待状態を手で写さず、changeplan がまとめたものを使う。
func environmentCreationWithAbsentVariableChange(t *testing.T, environment, name string) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t,
		changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, environment)}, Observed: github.Observed(setting.Absent()), Desired: setting.Present(),
			Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
		},
		changeplan.SettingPlan{
			Unit:     setting.Unit{Key: setting.EnvironmentVariable, Environment: mustEnvironmentName(t, environment), Variable: mustVariableName(t, name)},
			Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.Absent(), Result: changeplan.NotComparable, Applicability: changeplan.Applicable(),
		},
	))
}

// createdEnvironmentVariableChange は、計画時に存在しなかった environment の Environment に、同じ変更計画がその Environment を作成したあとで、
// Environment Variable name を値 desired で作成する論理的な設定変更。Environment の作成とは別の論理的な設定変更になる。
// 変更前提の基準を手で写さず、changeplan が導いた論理的な設定変更を使う。
func createdEnvironmentVariableChange(t *testing.T, environment, name, desired string) changeplan.LogicalChange {
	t.Helper()
	environmentName := mustEnvironmentName(t, environment)
	changes, err := newRepositoryPlan(t,
		changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: environmentName}, Observed: github.Observed(setting.Absent()), Desired: setting.Present(),
			Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
		},
		environmentVariablePlan(t, environment, name, github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), desired, changeplan.NotComparable),
	).LogicalChanges()
	if err != nil || len(changes) != 2 {
		t.Fatalf("LogicalChanges() = %+v, %v, want the creation of the environment and the creation of the variable", changes, err)
	}
	return changes[1]
}

// protectRuleset は、全体を解釈できた Ruleset の値として、既定のブランチの削除を制限し、Repository の admin に常に迂回を許す値を返す。
// 値の変更の前後は enforcement で作り分ける。
func protectRuleset(enforcement setting.RulesetEnforcement) setting.Ruleset {
	return setting.Ruleset{
		Target:       setting.RulesetTargetBranch,
		Enforcement:  enforcement,
		Conditions:   setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{}}},
		BypassActors: []setting.BypassActor{{ActorType: setting.BypassActorRepositoryRole, ActorID: 5, BypassMode: setting.BypassModeAlways}},
		Rules:        []setting.RulesetRule{setting.DeletionRule()},
	}
}

// rulesetMismatch は、計画時に observed と観測した、宣言の名前 name の Ruleset を、enforcement が active の protectRuleset にする適用可能な変更予定。
func rulesetMismatch(t *testing.T, name string, observed setting.Value) changeplan.SettingPlan {
	t.Helper()
	return changeplan.SettingPlan{
		Unit: setting.Unit{Key: setting.RulesetResource, Ruleset: mustRulesetName(t, name)}, Observed: github.Observed(observed),
		Desired: setting.RulesetValue(protectRuleset(setting.RulesetEnforcementActive)), Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
	}
}

// rulesetValueMismatch は、計画時に ID が id で enforcement が disabled の protectRuleset と観測した、宣言の名前 name の Ruleset を、
// enforcement が active の protectRuleset にする適用可能な値の変更予定。
func rulesetValueMismatch(t *testing.T, name string, id github.RulesetID) changeplan.SettingPlan {
	t.Helper()
	plan := rulesetMismatch(t, name, setting.RulesetValue(protectRuleset(setting.RulesetEnforcementDisabled)))
	plan.RulesetIdentity = changeplan.ObservedRulesetIdentity(id)
	return plan
}

// rulesetCreationChange は、計画時に存在しなかった name の Ruleset を作成する論理的な設定変更。
// 変更前提の基準を手で写さず、changeplan が導いた論理的な設定変更を使う。
func rulesetCreationChange(t *testing.T, name string) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t, rulesetMismatch(t, name, setting.Absent())))
}

// rulesetValueUpdateChange は、計画時に ID が 42 で enforcement が disabled と観測した name の Ruleset の値を、宣言の値に置き換える論理的な設定変更。
// 変更前提の基準を手で写さず、changeplan が導いた論理的な設定変更を使う。
func rulesetValueUpdateChange(t *testing.T, name string) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t, rulesetValueMismatch(t, name, 42)))
}
