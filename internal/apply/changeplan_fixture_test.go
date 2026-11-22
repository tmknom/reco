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
		DesiredState: []changeplan.DesiredSetting{{Unit: unit}},
		Operations: []changeplan.UpdateOperation{{
			Kind: changeplan.EnvironmentCreation, Unit: unit,
			RelevantSettings: []changeplan.RelevantSetting{{Unit: unit, AnticipatedEnvironment: github.Environments{}.Lookup(environment)}},
		}},
	}
}

// creationWithModeChange は、計画時に存在しなかった name の Environment を、デプロイ元制限の方式を mode にして作成する論理的な設定変更。
func creationWithModeChange(t *testing.T, name string, mode setting.DeploymentBranchPolicyMode) changeplan.LogicalChange {
	t.Helper()
	environment := mustEnvironmentName(t, name)
	unit := setting.Unit{Key: setting.EnvironmentResource, Environment: environment}
	return changeplan.LogicalChange{
		DesiredState: []changeplan.DesiredSetting{{Unit: unit}, {Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}, Value: modeValue(mode)}},
		Operations: []changeplan.UpdateOperation{{
			Kind: changeplan.EnvironmentCreationWithMode, Unit: unit, Mode: mode,
			RelevantSettings: []changeplan.RelevantSetting{{Unit: unit, AnticipatedEnvironment: github.Environments{}.Lookup(environment)}},
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
			{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: environment}, AnticipatedEnvironment: github.Environments{Existing: []github.Environment{approved}}.Lookup(environment)},
			{Unit: policy, AnticipatedValue: github.Observed(setting.DeploymentBranchPolicyValue(approved.DeploymentBranchPolicy))},
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
		changeplan.SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: environment}, ObservedEnvironment: observed, Result: changeplan.Match, Applicability: changeplan.Applicable},
		changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}, Observed: observed.DeploymentBranchPolicy(),
			Desired: setting.DeploymentBranchPolicyValue(desired), Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
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
		changeplan.SettingPlan{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: environment}, ObservedEnvironment: observed, Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
		changeplan.SettingPlan{
			Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}, Observed: observed.DeploymentBranchPolicy(),
			Desired: setting.DeploymentBranchPolicyValue(desired), Result: changeplan.NotComparable, Applicability: changeplan.Applicable,
		},
	))
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
		Desired: setting.VariableValue(desired), Result: changeplan.Mismatch, Applicability: changeplan.Applicable,
	}
}

// variableCreationChange は、計画時に存在しなかった name の Repository Variable を、値 desired で作成する論理的な設定変更。
// 変更前提の基準を手で写さず、changeplan が導いた論理的な設定変更を使う。
func variableCreationChange(t *testing.T, name, desired string) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t, variableMismatch(t, name, setting.AbsentVariable(), desired)))
}

// variableValueChange は、計画時に値 observed と観測した name の Repository Variable の値を desired にする論理的な設定変更。
// 変更前提の基準を手で写さず、changeplan が導いた論理的な設定変更を使う。
func variableValueChange(t *testing.T, name, observed, desired string) changeplan.LogicalChange {
	t.Helper()
	return onlyLogicalChange(t, newRepositoryPlan(t, variableMismatch(t, name, setting.VariableValue(observed), desired)))
}
