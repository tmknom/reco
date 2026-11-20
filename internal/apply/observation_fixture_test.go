package apply

import (
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// actionsEnabled は、Actions が有効で、Actions 許可ポリシーが selected でない Actions の実行許可の観測結果。
func actionsEnabled(policy setting.AllowedActionsPolicy, shaPinningRequired bool) *github.ActionsPermissions {
	return &github.ActionsPermissions{
		Enabled:            true,
		AllowedActions:     github.Observed(policy),
		SHAPinningRequired: github.Observed(shaPinningRequired),
		SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableNotSelected),
	}
}

// actionsSelected は、Actions 許可ポリシーが selected で、選択方式の Actions 許可条件が criteria の Actions の実行許可の観測結果。
func actionsSelected(criteria setting.SelectedActionsCriteria) *github.ActionsPermissions {
	return &github.ActionsPermissions{
		Enabled:            true,
		AllowedActions:     github.Observed(setting.AllowedActionsSelected),
		SHAPinningRequired: github.Observed(false),
		SelectedActions:    github.Observed(criteria),
	}
}

// actionsDisabled は、Actions が無効のリポジトリの Actions の実行許可の観測結果。
func actionsDisabled() *github.ActionsPermissions {
	return &github.ActionsPermissions{
		AllowedActions:     github.Unavailable[setting.AllowedActionsPolicy](github.UnavailableActionsDisabled),
		SHAPinningRequired: github.Unavailable[bool](github.UnavailableActionsDisabled),
		SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableActionsDisabled),
	}
}

// environments は、existing を一覧の全体とする Environment の観測結果。
func environments(existing ...github.Environment) *github.Environments {
	return &github.Environments{Existing: existing}
}

// observedEnvironment は、ID id、GitHub の綴り name、方式 mode で、保護ルールのない Environment の観測結果。
func observedEnvironment(id github.EnvironmentID, name string, mode setting.DeploymentBranchPolicyMode) github.Environment {
	return github.Environment{ID: id, Name: name, DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: mode}}
}

// modeValue は、方式 mode のデプロイ元制限の値。
func modeValue(mode setting.DeploymentBranchPolicyMode) setting.Value {
	return setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: mode})
}
