package changeplan

// isApplicable は、照合結果の設定を安全に適用できるかを返す。
// General は宣言された設定だけを部分更新するため、不一致は追加の条件なしに適用できる。
// 条件を確認できない比較不能は、適用不能として扱う。
func isApplicable(r ReconciliationResult) bool {
	return r == Mismatch
}
