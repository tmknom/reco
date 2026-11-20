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
		{Unit: setting.Unit{Key: setting.AllowedActions}, ValueAtApproval: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)},
		{Unit: setting.Unit{Key: setting.SelectedActions}, ValueAtApproval: github.Unavailable[setting.Value](github.UnavailableActionsDisabled)},
	})
}

// enabledStateChange は、Actions の有効状態を to にする論理的な設定変更。基準は有効状態の反対の観測値だけである。
func enabledStateChange(to bool) changeplan.LogicalChange {
	return singleUpdate(setting.ActionsEnabled, setting.BoolValue(to), relevantTo(setting.ActionsEnabled, setting.BoolValue(!to)))
}

// shaPinningRequiredChange は、Actions が有効のリポジトリで sha_pinning_required を false から true にする論理的な設定変更。
func shaPinningRequiredChange() changeplan.LogicalChange {
	return singleUpdate(setting.SHAPinningRequired, setting.BoolValue(true), []changeplan.RelevantSetting{
		{Unit: setting.Unit{Key: setting.SHAPinningRequired}, ValueAtApproval: github.Observed(setting.BoolValue(false))},
		{Unit: setting.Unit{Key: setting.ActionsEnabled}, ValueAtApproval: github.Observed(setting.BoolValue(true))},
	})
}

// shaPinningRequiredAfterEnablingChange は、Actions が無効のリポジトリで、Actions を有効にする変更のあとに sha_pinning_required を true にする論理的な設定変更。
func shaPinningRequiredAfterEnablingChange() changeplan.LogicalChange {
	return singleUpdate(setting.SHAPinningRequired, setting.BoolValue(true), relevantTo(setting.ActionsEnabled, setting.BoolValue(true)))
}

// creationChange は、計画時に存在しなかった name の Environment を作成する論理的な設定変更。
// desired がゼロ値でなければ、同じ更新要求で方式も設定する。
func creationChange(t *testing.T, name string, desired setting.Value) changeplan.LogicalChange {
	t.Helper()
	environment := mustEnvironmentName(t, name)
	unit := setting.Unit{Key: setting.EnvironmentResource, Environment: environment}
	return changeplan.LogicalChange{Operations: []changeplan.UpdateOperation{{
		Unit: unit, Desired: desired,
		RelevantSettings: []changeplan.RelevantSetting{{Unit: unit, EnvironmentAtApproval: github.Environments{}.Lookup(environment)}},
	}}}
}

// modeChange は、計画時に approved と観測した、宣言の名前 name の Environment の方式を desired にする論理的な設定変更。
func modeChange(t *testing.T, name string, approved github.Environment, desired setting.DeploymentBranchPolicyMode) changeplan.LogicalChange {
	t.Helper()
	environment := mustEnvironmentName(t, name)
	policy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: environment}
	return changeplan.LogicalChange{Operations: []changeplan.UpdateOperation{{
		Unit: policy, Desired: modeValue(desired),
		RelevantSettings: []changeplan.RelevantSetting{
			{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: environment}, EnvironmentAtApproval: github.Environments{Existing: []github.Environment{approved}}.Lookup(environment)},
			{Unit: policy, ValueAtApproval: github.Observed(setting.DeploymentBranchPolicyValue(approved.DeploymentBranchPolicy))},
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
	return []changeplan.RelevantSetting{{Unit: setting.Unit{Key: key}, ValueAtApproval: github.Observed(observed)}}
}

// singleUpdate は、key を desired にする更新操作一つからなる論理的な設定変更。
func singleUpdate(key setting.Key, desired setting.Value, relevant []changeplan.RelevantSetting) changeplan.LogicalChange {
	return changeplan.LogicalChange{Operations: []changeplan.UpdateOperation{{Unit: setting.Unit{Key: key}, Desired: desired, RelevantSettings: relevant}}}
}
