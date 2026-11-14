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
// 比較不能な変更予定は、変更前提の基準になる観測値がないため、適用可能と判断されていても error にする。
// 論理的な設定変更にするのは不一致の変更予定だけで、どれも単独の管理設定の変更として実現できるため、変更予定と論理的な設定変更は一対一になる。
func (p Plan) LogicalChanges() ([]LogicalChange, error) {
	var changes []LogicalChange
	for _, s := range p.Settings {
		if s.Result == Match {
			continue
		}
		if s.Applicability != Applicable || s.Result != Mismatch {
			return nil, fmt.Errorf("%s: %s cannot be applied", p.Target, s.Key)
		}
		// 不一致は観測値があるときだけ生じるため、値の有無は確かめない
		observed, _ := s.Observed.Value()
		changes = append(changes, LogicalChange{Key: s.Key, Observed: observed, Desired: s.Desired})
	}
	return changes, nil
}
