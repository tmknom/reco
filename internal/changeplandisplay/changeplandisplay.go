// Package changeplandisplay は、複数の操作が同じ形で示す表示のうち、変更計画の概念で決まるものを持つ。
package changeplandisplay

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/display"
	"github.com/tmknom/reco/internal/setting"
)

// PlannedChangeText は、変更予定を CLI 仕様の確認済みの差分か比較不能な設定予定の形で返し、副作用をその下に添える。
// 末尾に改行を含む複数行になりうる。一致は空文字列を返す。
// 比較不能の現在値は確認できていないため、確認済みの差分の形 (現在値 -> 期待値) で書かない。
func PlannedChangeText(s changeplan.SettingPlan) string {
	var b strings.Builder
	switch s.Result {
	case changeplan.Match:
		return ""
	case changeplan.Mismatch:
		observed, _ := s.Observed.Value()
		if s.Key == setting.SelectedActions {
			// 3項目を並べた値は長く、1行では -> を見つけにくいため、現在値と期待値を別の行に書く
			fmt.Fprintf(&b, "  %s:\n    %s\n    -> %s\n", s.Key, observed.String(), s.Desired.String())
		} else {
			fmt.Fprintf(&b, "  %s: %s -> %s\n", s.Key, observed.String(), s.Desired.String())
		}
	case changeplan.NotComparable:
		desired := s.Desired.String()
		if s.EnablesActionsByPolicy() {
			desired += " (enables GitHub Actions)"
		}
		fmt.Fprintf(&b, "  %s\n    Current: %s\n    Desired: %s\n", s.Key, display.UnavailableText(s.Observed.UnavailableReason()), desired)
	}
	for _, effect := range s.SideEffects {
		fmt.Fprintf(&b, "    Side effect: %s\n", sideEffectText(effect))
	}
	return b.String()
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
	return `GitHub may discard selected_actions if allowed_actions was "selected" before GitHub Actions was disabled (current values are unavailable)`
}
