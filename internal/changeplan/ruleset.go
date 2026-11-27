package changeplan

import (
	"fmt"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// rulesetChangeOf は、Ruleset の変更予定 s を、更新操作一つの論理的な設定変更にする。
// 期待値が存在しないことなら、計画時の観測が値のときだけ削除で、値を送らない。
// 期待値が値なら、計画時の観測が存在しないことなら作成、値なら値の変更で、宣言の値を送る。
// いずれも計画時の観測を変更前提の基準にし、値の変更と削除は、計画時に観測した ID も基準にする。作成の ID の基準はゼロ値である。
// 不一致でない計画と、観測も期待値も存在しない計画は、種類を決められないため、不変条件の破れとして panic する。
func rulesetChangeOf(s SettingPlan) LogicalChange {
	if s.Result != Mismatch {
		panic(fmt.Sprintf("changeplan: %s is not a mismatch to change", s.Unit.String()))
	}
	kind, value := RulesetValueUpdate, s.Desired
	switch {
	case s.Desired.Equal(setting.Absent()):
		if observed, _ := s.Observed.Value(); observed.Equal(setting.Absent()) {
			panic(fmt.Sprintf("changeplan: %s is absent and declared absent, nothing to change", s.Unit.String()))
		}
		kind, value = RulesetDeletion, setting.Value{}
	case s.IsConfirmedCreation():
		kind = RulesetCreation
	}
	return LogicalChange{
		DesiredState: []configuration.ManagedSetting{{Unit: s.Unit, Desired: s.Desired}},
		Operations: []UpdateOperation{{
			Kind: kind, Unit: s.Unit, Value: value,
			PreconditionBases: []PreconditionBasis{{Unit: s.Unit, AnticipatedValue: AnticipateUnchanged(s.Observed), AnticipatedRulesetIdentity: s.RulesetIdentity}},
		}},
	}
}

// AnticipatedRulesetID は、Ruleset の値の変更と削除の宛先の、承認時に予定した ID を返す。
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
