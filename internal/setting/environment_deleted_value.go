package setting

// EnvironmentDeletedValue は、Environment の削除で失われる値。中の設定の全体で、reco が種類を知らない保護ルールは持たない。
// 知らない種類の保護ルールがある Environment は、失われる値の全体を示せないため、この値を作らない。
type EnvironmentDeletedValue struct {
	DeploymentBranchPolicy DeploymentBranchPolicy
	ReviewerSet            ReviewerSet
	PreventSelfReview      bool
	WaitTimer              int
	CanAdminsBypass        bool
}

// Equal は、すべての項目が照合と同じ判定で同じかを返す。個別パターンと担当者の並び、担当者の login は比べない。
func (v EnvironmentDeletedValue) Equal(other EnvironmentDeletedValue) bool {
	return v.DeploymentBranchPolicy.equal(other.DeploymentBranchPolicy) &&
		v.ReviewerSet.equal(other.ReviewerSet) &&
		v.PreventSelfReview == other.PreventSelfReview &&
		v.WaitTimer == other.WaitTimer &&
		v.CanAdminsBypass == other.CanAdminsBypass
}
