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

// actionsWithUnknownPolicy は、Actions が有効で、Actions 許可ポリシーに GitHub が reco の知らない値を返した Actions の実行許可の観測結果。
// 取得条件を判断できない選択方式の Actions 許可条件も、ポリシーと同じ理由で観測不能になる。
func actionsWithUnknownPolicy(shaPinningRequired bool) *github.ActionsPermissions {
	return &github.ActionsPermissions{
		Enabled:            true,
		AllowedActions:     github.Unavailable[setting.AllowedActionsPolicy](github.UnavailableUnknownValue),
		SHAPinningRequired: github.Observed(shaPinningRequired),
		SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableUnknownValue),
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
	return github.Environment{ID: id, Name: name, DeploymentBranchPolicyMode: mode, Protection: unprotected()}
}

// unprotected は、保護ルールのない Environment の保護設定の観測結果。観測は保護ルールがなければ担当者の集合を空集合の観測値にする。
func unprotected() github.EnvironmentProtection {
	return github.EnvironmentProtection{ReviewerSet: github.Observed(setting.ReviewerSetOfRule(nil))}
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
	observedPatterns := github.Observed(github.DeploymentPatterns{Existing: slices.Clone(patterns), IDs: identified})
	return github.Environment{
		ID: id, Name: name,
		DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected,
		DeploymentPatterns:         &observedPatterns,
		Protection:                 unprotected(),
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

// withReviewers は、e に、自己承認の禁止が preventSelfReview で、種類が利用者で ID が ids の担当者を並べた required_reviewers の保護ルールを持たせた Environment の観測結果。
// ids が空なら、担当者のいない保護ルールになる。
func withReviewers(e github.Environment, preventSelfReview bool, ids ...int64) github.Environment {
	e.Protection.ReviewerSet = github.Observed(reviewerRule(ids...))
	e.Protection.PreventSelfReview = preventSelfReview
	return e
}

// reviewerRule は、種類が利用者で ID が ids の担当者を並べた required_reviewers の保護ルールの担当者の集合。ids が空なら、担当者のいない保護ルールになる。
func reviewerRule(ids ...int64) setting.ReviewerSet {
	rule := &setting.RequiredReviewers{Reviewers: []setting.Reviewer{}}
	for _, id := range ids {
		rule.Reviewers = append(rule.Reviewers, setting.Reviewer{Type: setting.ReviewerUser, ID: id})
	}
	return setting.ReviewerSetOfRule(rule)
}

// variables は、existing を一覧の全体とする変数の一覧の観測結果。Repository Variables と Environment Variables で同じ形である。
func variables(existing ...github.Variable) *github.Variables {
	return &github.Variables{Existing: existing}
}

// withVariables は、e に Environment Variables の観測結果 existing を一覧の全体として持たせた Environment の観測結果。
func withVariables(e github.Environment, existing ...github.Variable) github.Environment {
	e.Variables = variables(existing...)
	return e
}

// withSecrets は、e に Environment Secret の一覧の観測結果 existing を一覧の全体として持たせた Environment の観測結果。
// 削除で失われるものまで読んだ Environment は、変数の一覧も持つため、withVariables と組み合わせて使う。
func withSecrets(e github.Environment, existing ...github.EnvironmentSecret) github.Environment {
	e.Secrets = &github.EnvironmentSecrets{Existing: existing}
	return e
}

// deletableEnvironment は、ID id、GitHub の綴り name で、削除で失われる値の項目をどれも保護ルールのない値と異なる値で持ち、
// 変数 REGION と TIER、Secret DEPLOY_KEY と TOKEN を持つ、削除で失われるものまで読んだ Environment の観測結果。
// 項目ごとに一つだけ変えた観測を作れるよう、どの項目も既定値にしない。
func deletableEnvironment(id github.EnvironmentID, name string) github.Environment {
	e := withReviewers(selectedEnvironment(id, name, []setting.DeploymentPattern{branchPattern("main")}, 10), true, 1)
	e.Protection.WaitTimer = 30
	e.Protection.CanAdminsBypass = true
	e = withVariables(e, github.Variable{Name: "REGION", Value: "ap-northeast-1"}, github.Variable{Name: "TIER", Value: "1"})
	return withSecrets(e, github.EnvironmentSecret{Name: "DEPLOY_KEY", UpdatedAt: "2026-01-01T00:00:00Z"}, github.EnvironmentSecret{Name: "TOKEN", UpdatedAt: "2026-01-02T00:00:00Z"})
}
