package changeplan

import (
	"fmt"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// LogicalChange は、承認の対象になる論理的な設定変更。
// 実行は承認した変更内容だけを使い、設定宣言を読み直さずに変更前提と期待状態を確かめる。
type LogicalChange struct {
	Key     setting.Key
	Desired setting.Value
	// RelevantSettings は変更前提の基準。先頭は Key の設定で、副作用として提示した設定が続く。
	RelevantSettings []RelevantSetting
}

// RelevantSetting は、変更前提で比べる関係する現在値の設定と、その承認時に予定した値。
type RelevantSetting struct {
	Key        setting.Key
	AtApproval github.SettingObservation[setting.Value]
}

// LogicalChanges は、変更予定から論理的な設定変更を導く。
// 副作用を伴う変更予定は、副作用として提示した設定を基準に載せて受け付ける。
// 載せずに受け付けると承認の前に示していない値を失わせ得るため、受け付ける行と基準を組み立てる行を分けない。
// 遷移先が selected のポリシーの変更予定と選択方式の Actions 許可条件の比較不能な変更予定は、
// 二つの更新操作を順に行う論理的な設定変更になるため error にする。
// ポリシーは同じ計画に並ぶ許可条件によっても拒否されるが、設定宣言の組み合わせの規則に頼らずにここで拒否する。
// そのため論理的な設定変更になるのは、単独の管理設定の変更として実現できる変更予定だけで、変更予定と論理的な設定変更は一対一になる。
func (p Plan) LogicalChanges() ([]LogicalChange, error) {
	var changes []LogicalChange
	for _, s := range p.Settings {
		if s.Result == Match {
			continue
		}
		if s.Applicability != Applicable || (s.Result == NotComparable && !s.EnablesActions()) || targetsSelectedPolicy(s) {
			return nil, fmt.Errorf("%s: %s cannot be applied", p.Target, s.Key)
		}
		relevant := []RelevantSetting{{Key: s.Key, AtApproval: s.Observed}}
		for _, effect := range s.SideEffects {
			relevant = append(relevant, RelevantSetting{Key: effect.Key, AtApproval: effect.Observed})
		}
		changes = append(changes, LogicalChange{Key: s.Key, Desired: s.Desired, RelevantSettings: relevant})
	}
	return changes, nil
}

func targetsSelectedPolicy(s SettingPlan) bool {
	return s.Key == setting.AllowedActions && s.Desired.AllowedActionsPolicy() == setting.AllowedActionsSelected
}
