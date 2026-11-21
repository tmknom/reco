// Package changeplandisplay は、複数の操作が同じ形で示す表示のうち、変更計画の概念で決まるものを持つ。
package changeplandisplay

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

const unreadWhileDisabled = "reco cannot read them while GitHub Actions is disabled"

// PlannedChangeText は、変更予定を CLI 仕様の現在 -> 期待値の形で返し、適用不能の理由、再開する設定、副作用をその下に添える。
// 末尾に改行を含む複数行になりうる。一致は空文字列を返す。
func PlannedChangeText(s changeplan.SettingPlan) string {
	var b strings.Builder
	var current string
	switch s.Result {
	case changeplan.Match:
		return ""
	case changeplan.Mismatch:
		if s.IsCreation() {
			// 存在しないことは確かめた事実のため、比較不能の印の括弧に入れない
			current = "absent"
			break
		}
		observed, _ := s.Observed.Value()
		current = observed.String()
	case changeplan.NotComparable:
		current = repositoryStateText(s.UnavailableReason())
	}
	desired := "present"
	if s.Unit.Key != setting.EnvironmentResource {
		desired = s.Desired.String()
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
	if s.Applicability == changeplan.NotApplicable {
		fmt.Fprintf(&b, "    Applicability: not applicable (%s)\n", notApplicableReasonText(s.NotApplicableReason))
	}
	if len(s.ResumedSettings) > 0 {
		fmt.Fprintf(&b, "    Previous settings: %s\n", resumedSettingsText(s.ResumedSettings))
	}
	for _, effect := range s.SideEffects {
		fmt.Fprintf(&b, "    Side effect: %s\n", sideEffectText(effect))
	}
	return b.String()
}

// repositoryStateText は、比較不能な設定予定の現在に示す、値を返さない理由から分かるリポジトリの状態を返す。
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
	case github.UnavailableEnvironmentNameUndetermined:
		return "(existence unknown)"
	}
	panic(fmt.Sprintf("changeplandisplay: undefined unavailable reason %d", int(reason)))
}

// notApplicableReasonText は、適用不能の理由を、利用者が対処を選べる CLI 仕様の文言で返す。
// 未定義の理由は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func notApplicableReasonText(reason changeplan.NotApplicableReason) string {
	switch reason {
	case changeplan.NotApplicableActionsRemainDisabled:
		return "GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it"
	case changeplan.NotApplicablePublicRepository:
		return "the repository is public; remove access_level, which applies only to private repositories"
	case changeplan.NotApplicableEnvironmentNameUndetermined:
		return "an environment on GitHub has a name reco cannot compare, such as one with non-ASCII characters; remove this environment from the configuration"
	case changeplan.NotApplicableDependabotAlertsRemainDisabled:
		return "Dependabot alerts is disabled; declare dependabot_alerts: true to enable it"
	}
	panic(fmt.Sprintf("changeplandisplay: undefined not-applicable reason %d", int(reason)))
}

// resumedSettingsText は、再開する設定を、CLI 仕様の Previous settings: の文言で返す。
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

// sideEffectText は、副作用を CLI 仕様の Side effect: の文言で返す。
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
	case changeplan.SideEffectDependencyGraphEnabled:
		return "GitHub also enables the dependency graph"
	case changeplan.SideEffectDependencyGraphDisabled:
		return "GitHub also disables the dependency graph"
	case changeplan.SideEffectSecurityUpdatesDisabled:
		// 計画時に有効と観測した値だけを副作用にするため、観測値は必ずある
		current, _ := effect.Observed.Value()
		return "GitHub also disables dependabot_security_updates, which is currently " + current.String()
	}
	panic(fmt.Sprintf("changeplandisplay: undefined side effect kind %d", int(effect.Kind)))
}

// showsValuesOnSeparateLines は、現在と期待値を別の行に書くかを返す。
// 選択方式の Actions 許可条件と、個別パターンを並べる selected のデプロイ元制限は値が長く、1 行では -> を見つけにくい。
func showsValuesOnSeparateLines(s changeplan.SettingPlan) bool {
	if s.Unit.Key == setting.SelectedActions {
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

// patternListText は、個別パターンの一覧を、デプロイ元制限の値の中と同じ表記で返す。
func patternListText(patterns []setting.DeploymentPattern) string {
	return setting.DeploymentBranchPolicy{Patterns: patterns}.PatternsString()
}
