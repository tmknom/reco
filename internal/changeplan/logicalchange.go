package changeplan

import (
	"fmt"

	"github.com/tmknom/reco/internal/setting"
)

// LogicalChange は、承認の対象になる論理的な設定変更。
// 実行は承認した変更内容だけを使い、設定宣言を読み直さずに変更前提と期待状態を確かめる。
type LogicalChange struct {
	Key setting.Key
	// Observed は計画時の観測値で、承認時の値として変更前提の基準になる。
	Observed setting.Value
	Desired  setting.Value
}

// LogicalChanges は、変更予定から論理的な設定変更を導く。
// 適用可能と判断されていても、LogicalChange が変更前提の基準を運べない変更予定は error にする。
// 比較不能な変更予定は基準になる観測値がなく、副作用を伴う変更予定は失われる値を基準に持てないためである。
// 失われる値を更新直前に確かめずに更新すると、承認の前に示していない値を失わせ得る。
// 遷移先が selected のポリシーの変更予定は、同じ計画に並ぶ選択方式の Actions 許可条件の比較不能な変更予定によって拒否される。
// そのため論理的な設定変更になるのは、単独の管理設定の変更として実現できる変更予定だけで、変更予定と論理的な設定変更は一対一になる。
func (p Plan) LogicalChanges() ([]LogicalChange, error) {
	var changes []LogicalChange
	for _, s := range p.Settings {
		if s.Result == Match {
			continue
		}
		if s.Applicability != Applicable || s.Result != Mismatch || len(s.SideEffects) > 0 {
			return nil, fmt.Errorf("%s: %s cannot be applied", p.Target, s.Key)
		}
		// 不一致は観測値があるときだけ生じるため、値の有無は確かめない
		observed, _ := s.Observed.Value()
		changes = append(changes, LogicalChange{Key: s.Key, Observed: observed, Desired: s.Desired})
	}
	return changes, nil
}
