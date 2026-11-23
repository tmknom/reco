package changeplan

import (
	"fmt"

	"github.com/tmknom/reco/internal/setting"
)

// rulesetChangeOf は、Ruleset の変更予定 s を、更新操作一つの論理的な設定変更にする。
// 計画時の観測が存在しないことなら作成、値なら値の変更で、どちらも計画時の観測を変更前提の基準にし、宣言の値を送る。
// 不一致でない計画は、作成とも値の変更とも決められないため、不変条件の破れとして panic する。
func rulesetChangeOf(s SettingPlan) LogicalChange {
	if s.Result != Mismatch {
		panic(fmt.Sprintf("changeplan: %s is not a mismatch to change", s.Unit.String()))
	}
	kind := RulesetValueUpdate
	if value, _ := s.Observed.Value(); value.Equal(setting.Absent()) {
		kind = RulesetCreation
	}
	return LogicalChange{
		DesiredState: []DesiredSetting{{Unit: s.Unit, Value: s.Desired}},
		Operations: []UpdateOperation{{
			Kind: kind, Unit: s.Unit, Value: s.Desired,
			RelevantSettings: []RelevantSetting{{Unit: s.Unit, AnticipatedValue: s.Observed}},
		}},
	}
}
