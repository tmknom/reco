package github

import "github.com/tmknom/reco/internal/setting"

// RulesetObservation は、同一性を判定できる名前で引いた一つの Ruleset の観測結果。存在する、存在しない、理由を伴う観測不能のいずれか一つを持つ。
// 存在する Ruleset が解釈しない部分を含むときは、存在を確かめていても、値の観測としては解釈しない部分を理由とする観測不能になる。
// 状態を取り違えた値を作れないよう、フィールドを公開せず Rulesets.Lookup でだけ作る。
// どの状態でもないゼロ値は作れるが、値の観測として読もうとすると panic で検出する。Existing はゼロ値に偽を返す。
type RulesetObservation struct {
	state   resourceState
	ruleset Ruleset
	reason  UnavailableReason
}

// SettingObservation は、その Ruleset そのものの観測結果を、全体を解釈できた値なら Ruleset の値、存在しないなら Absent の観測値で返す。
// 照合、変更前提の確認、期待状態の確認が、リポジトリ設定と同じ判定で比べられるようにするため。ID は含めない。
// 存在するのに値を読んでいない Ruleset は、読む範囲と Lookup の比較が食い違った不変条件の破れで、Ruleset.Value の panic で検出する。
func (o RulesetObservation) SettingObservation() SettingObservation[setting.Value] {
	switch o.state {
	case resourceExisting:
		if value, ok := o.ruleset.Value(); ok {
			return Observed(setting.RulesetValue(value))
		}
		return Unavailable[setting.Value](UnavailableUninterpretedParts)
	case resourceAbsent:
		return Observed(setting.Absent())
	case resourceUnavailable:
		return Unavailable[setting.Value](o.reason)
	}
	panic("github: ruleset observation has no state")
}

// Existing は、存在するときだけ、観測した Ruleset を GitHub の ID と綴りのまま返し、真を返す。
// 解釈しない部分を含む Ruleset も、存在すれば返す。観測不能、存在しない、ゼロ値では偽を返す。
func (o RulesetObservation) Existing() (Ruleset, bool) {
	if o.state != resourceExisting {
		return Ruleset{}, false
	}
	return o.ruleset, true
}

// UninterpretedParts は、存在する Ruleset が解釈しない部分を含むときだけ、解釈しなかったものを返す。ほかの観測結果では nil を返す。
// 比較できない理由に伴う解釈しなかったものを、理由と同じ観測結果から読めるようにするため。
func (o RulesetObservation) UninterpretedParts() []UninterpretedPart {
	if o.state != resourceExisting {
		return nil
	}
	if _, ok := o.ruleset.Value(); ok {
		return nil
	}
	return o.ruleset.UninterpretedParts()
}
