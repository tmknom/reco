package changeplan

import (
	"fmt"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// Applicability は、変更予定 (不一致または比較不能と照合した設定) を安全に変更できるかの判断。
// ゼロ値の ApplicabilityNotJudged は、一致した設定に判断がないことを表し、三つ目の判断ではない。
type Applicability int

const (
	ApplicabilityNotJudged Applicability = iota
	Applicable
	NotApplicable
)

// judgeApplicability は、管理設定の計画の観測結果から、変更予定を安全に適用できるかを判断する。
// 不一致は、アーキテクチャ設計の未管理設定を保全する更新方式で、追加の条件なしに適用できる。
// 比較不能は、アーキテクチャ設計の遷移表に挙がる観測不能に限って適用できる。
// 遷移表にない観測不能を適用可能として黙って通さないよう、不変条件の破れとして panic する。
func judgeApplicability(s SettingPlan) Applicability {
	switch s.Result {
	case Match:
		return ApplicabilityNotJudged
	case Mismatch:
		return Applicable
	case NotComparable:
	}
	reason := s.Observed.UnavailableReason()
	switch {
	case s.Key == setting.AllowedActions && reason == github.UnavailableActionsDisabled:
		return Applicable
	case s.Key == setting.SelectedActions && (reason == github.UnavailableNotSelected || reason == github.UnavailableActionsDisabled):
		return Applicable
	}
	panic(fmt.Sprintf("changeplan: %s is not comparable for an unexpected reason %d", s.Key, int(reason)))
}
