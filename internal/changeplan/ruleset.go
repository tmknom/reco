package changeplan

import (
	"fmt"

	"github.com/tmknom/reco/internal/github"
)

// rulesetChangeOf は、Ruleset の変更予定 s を、更新操作一つの論理的な設定変更にする。
// 計画時の観測が存在しないことなら作成、値なら値の変更で、どちらも計画時の観測を変更前提の基準にし、宣言の値を送る。
// 値の変更は、計画時に観測した ID も変更前提の基準にする。作成の ID の基準はゼロ値である。
// 不一致でない計画は、作成とも値の変更とも決められないため、不変条件の破れとして panic する。
func rulesetChangeOf(s SettingPlan) LogicalChange {
	if s.Result != Mismatch {
		panic(fmt.Sprintf("changeplan: %s is not a mismatch to change", s.Unit.String()))
	}
	kind := RulesetValueUpdate
	if s.IsCreation() {
		kind = RulesetCreation
	}
	return LogicalChange{
		DesiredState: []DesiredSetting{{Unit: s.Unit, Value: s.Desired}},
		Operations: []UpdateOperation{{
			Kind: kind, Unit: s.Unit, Value: s.Desired,
			PreconditionBases: []PreconditionBasis{{Unit: s.Unit, AnticipatedValue: AnticipateUnchanged(s.Observed), AnticipatedRulesetIdentity: s.RulesetIdentity}},
		}},
	}
}

// AnticipatedRulesetID は、Ruleset の値の変更の宛先の、承認時に予定した ID を返す。
// 宛先は更新直前の観測から引かず、計画時に観測した ID に固定する (INV-004)。
// 更新操作の管理の単位の変更前提に、観測した ID の基準がなければ、宛先を決められない不変条件の破れとして panic する。
func (o UpdateOperation) AnticipatedRulesetID() github.RulesetID {
	for _, basis := range o.PreconditionBases {
		if !basis.Unit.Same(o.Unit) {
			continue
		}
		if id, ok := basis.AnticipatedRulesetIdentity.ID(); ok {
			return id
		}
	}
	panic("changeplan: " + o.Unit.String() + " has no observed ID to send the update to")
}
