// Package changeplandisplay は、複数の操作が同じ形で示す表示のうち、変更計画の概念で決まるものを持つ。
package changeplandisplay

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

const unreadWhileDisabled = "reco cannot read them while GitHub Actions is disabled"

// UnknownValueText は、GitHub が reco の知らない値を返した設定の値の表記。表示仕様は、変更予定の現在値と、変更対象外の設定の予期しない変化の値に同じ表記を使う。
const UnknownValueText = "(unknown value)"

// PlannedChangeText は、変更予定を表示仕様の現在 -> 期待値の形で返し、適用不能の理由、変更予定の注記、再開する設定、副作用をその下に添える。
// 末尾に改行を含む複数行になりうる。一致は空文字列を返す。
func PlannedChangeText(s changeplan.SettingPlan) string {
	if s.Unit.Key == setting.RulesetResource {
		return rulesetChangeText(s)
	}
	var b strings.Builder
	var current string
	switch s.Result {
	case changeplan.Match:
		return ""
	case changeplan.Mismatch:
		observed, _ := s.Observed.Value()
		current = observed.String()
	case changeplan.NotComparable:
		current = repositoryStateText(s.Observed.UnavailableReason())
	}
	desired := s.Desired.String()
	if s.Unit.Key == setting.EnvironmentReviewerSet {
		desired = desiredReviewersWithLogins(s).String()
	}
	if s.EnablesActionsByPolicy() {
		desired += " (enables GitHub Actions)"
	}
	if showsValuesOnSeparateLines(s) {
		fmt.Fprintf(&b, "  %s:\n    %s\n    -> %s\n", s.Unit.String(), current, desired)
	} else {
		fmt.Fprintf(&b, "  %s: %s -> %s\n", s.Unit.String(), current, desired)
	}
	b.WriteString(deploymentPatternChangesText(s))
	b.WriteString(reviewerChangesText(s))
	if s.Applicability.IsNotApplicable() {
		fmt.Fprintf(&b, "    Applicability: not applicable (%s)\n", notApplicableReasonText(s))
	}
	for _, note := range s.Notes() {
		fmt.Fprintf(&b, "    Note: %s\n", plannedChangeNoteText(note))
	}
	if len(s.ResumedSettings) > 0 {
		fmt.Fprintf(&b, "    Previous settings: %s\n", resumedSettingsText(s.ResumedSettings))
	}
	for _, effect := range s.SideEffects {
		fmt.Fprintf(&b, "    Side effect: %s\n", sideEffectText(effect))
	}
	return b.String()
}

// repositoryStateText は、比較不能な設定予定の現在に示す、観測不能の理由から分かる状態を返す。
// 未定義の理由は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func repositoryStateText(reason github.UnavailableReason) string {
	switch reason {
	case github.UnavailableActionsDisabled:
		return "(GitHub Actions disabled)"
	case github.UnavailableNotSelected:
		return "(not in use)"
	case github.UnavailablePublicRepository:
		return "(public repository)"
	case github.UnavailableEnvironmentAbsent:
		return "(environment absent)"
	case github.UnavailableNameUndetermined:
		return "(existence unknown)"
	case github.UnavailablePrivateRepository:
		return "(private repository)"
	case github.UnavailableUninterpretedParts:
		return "(uninterpreted parts)"
	case github.UnavailableUnknownValue:
		return UnknownValueText
	case github.UnavailableNotReturned:
		return "(not returned)"
	}
	panic(fmt.Sprintf("changeplandisplay: undefined unavailable reason %d", int(reason)))
}

// notApplicableReasonText は、適用不能の理由を、成り立っていない条件と設定ファイルの直し方を示す文で返す。
// 公開範囲の理由は複数の設定が持つため、直し方に挙げるキーを s の管理の単位から決める。
// 未定義の理由は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func notApplicableReasonText(s changeplan.SettingPlan) string {
	key := s.Unit.Key
	switch s.Applicability.Reason() {
	case changeplan.NotApplicableActionsRemainDisabled:
		return "GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it"
	case changeplan.NotApplicablePublicRepository:
		return "the repository is public; remove " + removalKeyName(key) + ", which applies only to private repositories"
	case changeplan.NotApplicableNameUndetermined:
		if key == setting.RulesetResource {
			return "a ruleset on GitHub has a name reco cannot compare, such as one with non-ASCII characters; remove this ruleset from the configuration"
		}
		return "an environment on GitHub has a name reco cannot compare, such as one with non-ASCII characters; remove this environment from the configuration"
	case changeplan.NotApplicableDependabotAlertsRemainDisabled:
		return "Dependabot alerts is disabled; declare dependabot_alerts: true to enable it"
	case changeplan.NotApplicablePrivateRepository:
		// GitHub が非公開リポジトリに適用しない設定は、reco が管理しないのではなく、適用されないことを示す
		if key == setting.ForkPullRequestApproval {
			return "the repository is private; remove " + removalKeyName(key) + ", which applies only to public repositories"
		}
		return "the repository is private; remove " + key.DeclaredKeyName() + ", which reco manages only on public repositories"
	case changeplan.NotApplicableSecretScanningRemainsDisabled:
		return "Secret scanning is disabled; declare secret_scanning: true to enable it"
	case changeplan.NotApplicableSecretScanningUnknown:
		return "GitHub returned a value of secret_scanning that reco does not know; remove secret_scanning_push_protection from the configuration"
	case changeplan.NotApplicableNoMergeMethodEnabled:
		return noMergeMethodEnabledText(s.Applicability.UnmanagedSettings())
	case changeplan.NotApplicableUninterpretedParts:
		return "the ruleset on GitHub contains parts that reco does not interpret; remove this ruleset from the configuration"
	case changeplan.NotApplicableAboveMaximumAllowed:
		maximum := s.Applicability.MaximumAllowed()
		return fmt.Sprintf("%d days exceeds the maximum of %d days that GitHub allows for this repository; declare %s: %d or less", s.Desired.RetentionDays(), maximum, key.DeclaredKeyName(), maximum)
	case changeplan.NotApplicableNoReviewers:
		return "the environment would have no required reviewers, which GitHub requires to set prevent_self_review; declare reviewers with at least one reviewer or remove prevent_self_review"
	case changeplan.NotApplicableMergeMethodRemainsDisabled:
		if method, ok := key.CorrespondingMergeMethod(); ok {
			methodName, name := method.DeclaredKeyName(), key.DeclaredKeyName()
			return methodName + " is disabled, and GitHub does not accept changes to " + name + " while it is disabled; declare " + methodName + ": true or remove " + name
		}
	case changeplan.NotApplicableUnknownValue:
		return "GitHub returned a value that reco does not know; remove " + removalKeyName(key) + " from the configuration"
	case changeplan.NotApplicableNotReturned:
		return "GitHub did not return the value; remove " + key.DeclaredKeyName() + " from the configuration"
	}
	panic(fmt.Sprintf("changeplandisplay: undefined not-applicable reason %d", int(s.Applicability.Reason())))
}

// removalKeyName は、key の宣言を消すときに消すキーの名前を返す。
func removalKeyName(key setting.Key) string {
	if key == setting.ActionsAccessLevel {
		return "access"
	}
	if key == setting.ForkPullRequestApproval {
		return "fork_pr_contributor_approval"
	}
	return key.DeclaredKeyName()
}

// noMergeMethodEnabledText は、更新後に有効なマージ方式が残らない理由を、無効のまま残るマージ方式と、そのどれかを有効にする宣言で返す。
// 変更予定のマージ方式は宣言されているため、無効のまま残るのは 1 つか 2 つで、ほかの数は不変条件の破れとして panic する。
func noMergeMethodEnabledText(keys []setting.Key) string {
	names := make([]string, len(keys))
	for i, key := range keys {
		names[i] = strings.TrimPrefix(string(key), "general.")
	}
	switch len(names) {
	case 1:
		return names[0] + " is disabled, so no merge method would remain enabled; declare " + names[0] + ": true to enable it"
	case 2:
		return names[0] + " and " + names[1] + " are disabled, so no merge method would remain enabled; declare " +
			names[0] + ": true or " + names[1] + ": true to enable one of them"
	}
	panic(fmt.Sprintf("changeplandisplay: %d merge methods left disabled", len(names)))
}

// resumedSettingsText は、再開する設定を、Previous settings: の行の文で返す。
// 設定の名前は英語の列挙にし、値を示せない理由の代名詞を挙げる設定の数に合わせる。
func resumedSettingsText(keys []setting.Key) string {
	names := make([]string, len(keys))
	for i, key := range keys {
		names[i] = strings.TrimPrefix(string(key), "actions.permissions.")
	}
	var listed, unread string
	switch len(names) {
	case 1:
		listed, unread = names[0], "reco cannot read it while GitHub Actions is disabled"
	case 2:
		listed, unread = strings.Join(names, " and "), unreadWhileDisabled
	default:
		listed, unread = strings.Join(names[:len(names)-1], ", ")+", and "+names[len(names)-1], unreadWhileDisabled
	}
	return "GitHub Actions resumes with " + listed + " kept from before it was disabled (" + unread + ")"
}

// sideEffectText は、副作用を Side effect: の行の文で返す。
// 失われる値を観測できなかったときは、値を推測せずに失われる可能性だけを示す。
// Dependency graph は reco が観測しない設定のため、現在値を示さない。
// 未定義の種類は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func sideEffectText(effect changeplan.SideEffect) string {
	switch effect.Kind {
	case changeplan.SideEffectSelectedActionsDiscarded:
		if lost, ok := effect.Observed.Value(); ok {
			return "GitHub discards the current selected_actions (" + lost.String() + ")"
		}
		return `GitHub may discard selected_actions if allowed_actions was "selected" before GitHub Actions was disabled (` + unreadWhileDisabled + `)`
	case changeplan.SideEffectPreventSelfReviewDiscarded:
		// 計画時に真と観測した値だけを副作用にするため、観測値は必ずある
		current, _ := effect.Observed.Value()
		return "GitHub also discards prevent_self_review, which is currently " + current.String()
	case changeplan.SideEffectDependencyGraphEnabled:
		return "GitHub also enables the dependency graph"
	case changeplan.SideEffectDependencyGraphDisabled:
		return "GitHub also disables the dependency graph"
	case changeplan.SideEffectSecurityUpdatesDisabled:
		// 計画時に有効と観測した値だけを副作用にするため、観測値は必ずある
		current, _ := effect.Observed.Value()
		return "GitHub also disables dependabot_security_updates, which is currently " + current.String()
	case changeplan.SideEffectPushProtectionDisabled:
		// 観測値は計画時に有効と観測した値で、観測不能は push protection が知らない値だったことを表す
		if current, ok := effect.Observed.Value(); ok {
			return "GitHub also disables secret_scanning_push_protection, which is currently " + current.String()
		}
		return "GitHub may also disable secret_scanning_push_protection (reco does not know its current value)"
	}
	panic(fmt.Sprintf("changeplandisplay: undefined side effect kind %d", int(effect.Kind)))
}

// plannedChangeNoteText は、変更予定の注記を Note: の行の文で返す。文言は設定の意味の事実だけを述べ、評価を含めない。
// 未定義の種類は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func plannedChangeNoteText(note changeplan.PlannedChangeNote) string {
	switch note {
	case changeplan.NoteForkWorkflowsGetWriteTokens:
		return "Workflows triggered by pull requests from forks get a GITHUB_TOKEN with write permissions"
	case changeplan.NoteForkWorkflowsGetSecretsAndVariables:
		return "Workflows triggered by pull requests from forks can use secrets and variables"
	case changeplan.NoteOIDCSubjectChanges:
		return "This changes how the subject claim of OIDC tokens is composed; trust conditions in external cloud providers that match the current subject claim may stop matching"
	}
	panic(fmt.Sprintf("changeplandisplay: undefined planned change note %d", int(note)))
}

// showsValuesOnSeparateLines は、現在と期待値を別の行に書くかを返す。
// 選択方式の Actions 許可条件、非公開リポジトリの Fork Pull Request ワークフローの実行条件、OIDC subject のカスタマイズ、マージメッセージ、個別パターンを並べる selected のデプロイ元制限、Environment の担当者の集合は値が長く、1 行では -> を見つけにくい。
func showsValuesOnSeparateLines(s changeplan.SettingPlan) bool {
	if slices.Contains([]setting.Key{setting.SelectedActions, setting.PrivateForkPullRequestWorkflows, setting.OIDCSubject, setting.SquashMergeCommit, setting.MergeCommit, setting.EnvironmentReviewerSet}, s.Unit.Key) {
		return true
	}
	if s.Unit.Key != setting.EnvironmentDeploymentBranchPolicy {
		return false
	}
	if s.Desired.DeploymentBranchPolicy().Mode == setting.DeploymentBranchPolicySelected {
		return true
	}
	observed, ok := s.Observed.Value()
	return ok && observed.DeploymentBranchPolicy().Mode == setting.DeploymentBranchPolicySelected
}

// deploymentPatternChangesText は、現在と期待値の両方が selected の確認済みの差分について、なくなるパターンと加わるパターンの行を返す。
func deploymentPatternChangesText(s changeplan.SettingPlan) string {
	if s.Unit.Key != setting.EnvironmentDeploymentBranchPolicy {
		return ""
	}
	observedValue, ok := s.Observed.Value()
	if !ok {
		return ""
	}
	observed, desired := observedValue.DeploymentBranchPolicy(), s.Desired.DeploymentBranchPolicy()
	if observed.Mode != setting.DeploymentBranchPolicySelected || desired.Mode != setting.DeploymentBranchPolicySelected {
		return ""
	}
	var b strings.Builder
	if removed := observed.PatternsNotIn(desired); len(removed) > 0 {
		fmt.Fprintf(&b, "    Patterns to remove: %s\n", patternListText(removed))
	}
	if added := desired.PatternsNotIn(observed); len(added) > 0 {
		fmt.Fprintf(&b, "    Patterns to add: %s\n", patternListText(added))
	}
	return b.String()
}

// desiredReviewersWithLogins は、担当者の集合の期待値を、観測値にいる担当者へ観測した login を添えて返す。
// 観測値がなければ、login を持たない期待値のままにする。reco は ID から login を解決しないため。
func desiredReviewersWithLogins(s changeplan.SettingPlan) setting.ReviewerSet {
	desired := s.Desired.ReviewerSet()
	if observed, ok := s.Observed.Value(); ok {
		return desired.WithLoginsFrom(observed.ReviewerSet())
	}
	return desired
}

// reviewerChangesText は、なくなる担当者と加わる担当者の行を返す。
// 比較不能な設定予定には添えない。現在の担当者を確かめておらず、差を示せないため。
func reviewerChangesText(s changeplan.SettingPlan) string {
	if s.Unit.Key != setting.EnvironmentReviewerSet {
		return ""
	}
	observedValue, ok := s.Observed.Value()
	if !ok {
		return ""
	}
	observed, desired := observedValue.ReviewerSet(), desiredReviewersWithLogins(s)
	var b strings.Builder
	if removed := observed.NotIn(desired); len(removed.Reviewers()) > 0 {
		fmt.Fprintf(&b, "    Reviewers to remove: %s\n", removed.String())
	}
	if added := desired.NotIn(observed); len(added.Reviewers()) > 0 {
		fmt.Fprintf(&b, "    Reviewers to add: %s\n", added.String())
	}
	return b.String()
}

// patternListText は、個別パターンの一覧を、デプロイ元制限の値の中と同じ表記で返す。
func patternListText(patterns []setting.DeploymentPattern) string {
	return setting.DeploymentBranchPolicy{Patterns: patterns}.PatternsString()
}
