package apply

import (
	"slices"

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

// publicSecretScanning は、公開リポジトリの Secret scanning の観測結果。
func publicSecretScanning(enabled, pushProtection bool) *github.SecretScanning {
	return &github.SecretScanning{Enabled: github.Observed(enabled), PushProtection: github.Observed(pushProtection)}
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

// selectedEnvironment は、ID id、GitHub の綴り name、方式 selected で、個別パターン patterns を一覧の並びで持つ Environment の観測結果。
// patterns[i] の GitHub の ID は ids[i] である。ID の対応を map で受けないのは、個別パターンの並びが定まらなくなるため。
func selectedEnvironment(id github.EnvironmentID, name string, patterns []setting.DeploymentPattern, ids ...github.DeploymentPatternID) github.Environment {
	identified := make(map[setting.DeploymentPattern]github.DeploymentPatternID, len(patterns))
	for i, pattern := range patterns {
		identified[pattern] = ids[i]
	}
	return github.Environment{
		ID: id, Name: name,
		DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: slices.Clone(patterns)},
		DeploymentPatternIDs:   identified,
	}
}

// branchPattern は、種類が branch で名前が name の個別パターン。
func branchPattern(name string) setting.DeploymentPattern {
	return setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: name}
}

// tagPattern は、種類が tag で名前が name の個別パターン。
func tagPattern(name string) setting.DeploymentPattern {
	return setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: name}
}

// selectedPolicy は、方式 selected で、個別パターン patterns を宣言の並びで持つデプロイ元制限。
func selectedPolicy(patterns ...setting.DeploymentPattern) setting.DeploymentBranchPolicy {
	return setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: append([]setting.DeploymentPattern{}, patterns...)}
}

// variables は、existing を一覧の全体とする Repository Variables の観測結果。
func variables(existing ...github.Variable) *github.Variables {
	return &github.Variables{Existing: existing}
}
