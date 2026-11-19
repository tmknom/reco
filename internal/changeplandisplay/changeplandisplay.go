// Package changeplandisplay は、複数の操作が同じ形で示す表示のうち、変更計画の概念で決まるものを持つ。
package changeplandisplay

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// PlannedChangeText は、変更予定を CLI 仕様の現在 -> 期待値の形で返し、適用不能の理由と副作用をその下に添える。
// 末尾に改行を含む複数行になりうる。一致は空文字列を返す。
func PlannedChangeText(s changeplan.SettingPlan) string {
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
	if s.EnablesActionsByPolicy() {
		desired += " (enables GitHub Actions)"
	}
	if s.Key == setting.SelectedActions {
		// 3項目を並べた値は長く、1行では -> を見つけにくいため、現在と期待値を別の行に書く
		fmt.Fprintf(&b, "  %s:\n    %s\n    -> %s\n", s.Key, current, desired)
	} else {
		fmt.Fprintf(&b, "  %s: %s -> %s\n", s.Key, current, desired)
	}
	if s.Applicability == changeplan.NotApplicable {
		fmt.Fprintf(&b, "    Applicability: not applicable (%s)\n", notApplicableReasonText(s.NotApplicableReason))
	}
	if s.ReenablesActions() {
		b.WriteString("    Previous settings: GitHub Actions resumes with allowed_actions, selected_actions, and sha_pinning_required kept from before it was disabled (" + unreadWhileDisabled + ")\n")
	}
	for _, effect := range s.SideEffects {
		fmt.Fprintf(&b, "    Side effect: %s\n", sideEffectText(effect))
	}
	return b.String()
}

const unreadWhileDisabled = "reco cannot read them while GitHub Actions is disabled"

// repositoryStateText は、比較不能な設定予定の現在に示す、値を返さない理由から分かるリポジトリの状態を返す。
// 未定義の理由は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func repositoryStateText(reason github.UnavailableReason) string {
	switch reason {
	case github.UnavailableActionsDisabled:
		return "(GitHub Actions disabled)"
	case github.UnavailableNotSelected:
		return "(not in use)"
	}
	panic(fmt.Sprintf("changeplandisplay: undefined unavailable reason %d", int(reason)))
}

// notApplicableReasonText は、適用不能の理由を、利用者が対処を選べる CLI 仕様の文言で返す。
// 未定義の理由は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func notApplicableReasonText(reason changeplan.NotApplicableReason) string {
	switch reason {
	case changeplan.NotApplicableActionsRemainDisabled:
		return "GitHub Actions is disabled; declare enabled: true or allowed_actions to enable it"
	}
	panic(fmt.Sprintf("changeplandisplay: undefined not-applicable reason %d", int(reason)))
}

// sideEffectText は、副作用を CLI 仕様の Side effect: の文言で返す。
// 副作用は、Actions 許可ポリシーの変更で選択方式の Actions 許可条件が失われることだけのため、文言はその設定に限り、
// ほかの設定の副作用は誤った文言で黙って示さないよう、不変条件の破れとして panic する。
// 失われる値を観測できなかったときは、値を推測せずに失われる可能性だけを示す。
func sideEffectText(effect changeplan.SideEffect) string {
	if effect.Key != setting.SelectedActions {
		panic("changeplandisplay: no side effect wording for " + string(effect.Key))
	}
	if lost, ok := effect.Observed.Value(); ok {
		return "GitHub discards the current selected_actions (" + lost.String() + ")"
	}
	return `GitHub may discard selected_actions if allowed_actions was "selected" before GitHub Actions was disabled (` + unreadWhileDisabled + `)`
}
