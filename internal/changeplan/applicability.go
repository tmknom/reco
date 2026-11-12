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
// General は宣言された設定だけを部分更新するため、不一致は追加の条件なしに適用できる。
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
