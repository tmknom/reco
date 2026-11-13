package changeplan

// Applicability は、変更予定 (不一致または比較不能と照合した設定) を安全に変更できるかの判断。
// ゼロ値の ApplicabilityNotJudged は、一致した設定に判断がないことを表し、三つ目の判断ではない。
type Applicability int

const (
	ApplicabilityNotJudged Applicability = iota
	Applicable
	NotApplicable
)

// judgeApplicability は、照合結果の設定を安全に適用できるかを判断する。
// 不一致は、アーキテクチャ設計の未管理設定を保全する更新方式で、追加の条件なしに適用できる。
// 条件を確認できない比較不能は、適用不能として扱う。
func judgeApplicability(r ReconciliationResult) Applicability {
	switch r {
	case Mismatch:
		return Applicable
	case NotComparable:
		return NotApplicable
	default:
		return ApplicabilityNotJudged
	}
}
