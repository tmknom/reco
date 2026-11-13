package changeplan

import (
	"fmt"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// LogicalChange は、承認の対象になる論理的な設定変更。
// 実行は承認した変更内容だけを使い、設定宣言を読み直さずに変更前提と期待状態を確かめる。
type LogicalChange struct {
	Key setting.Key
	// Observed は計画時の観測値で、承認時の値として変更前提の基準になる。
	Observed SettingValue
	Desired  SettingValue
}

// LogicalChanges は、変更予定から論理的な設定変更を導く。
// 管理設定はどれも単独で更新できるため、変更予定と論理的な設定変更は一対一になる。
// 比較不能な変更予定は、変更前提の基準になる観測値がないため、適用可能と判断されていても error にする。
func (p Plan) LogicalChanges() ([]LogicalChange, error) {
	var changes []LogicalChange
	for _, s := range p.Settings {
		if s.Result == Match {
			continue
		}
		if s.Applicability != Applicable || s.Result != Mismatch {
			return nil, fmt.Errorf("%s: %s cannot be applied", p.Target, s.Key)
		}
		changes = append(changes, LogicalChange{Key: s.Key, Observed: s.Observed, Desired: s.Desired})
	}
	return changes, nil
}

// Reconcile は、c の期待値を observed と照合する。observed は c の設定の領域を読み取った観測の結果に限る。
func (c LogicalChange) Reconcile(observed github.ObservationResult) ReconciliationResult {
	return compare(ObservedValue(c.Key, observed), c.Desired)
}
