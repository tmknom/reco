package setting

// ResetParameter は、宣言が省略し、値の全体を置き換える更新が、送らなかったことで GitHub の決める値に変えるパラメータ。
// Value は、変わる前の観測値の表記である。
type ResetParameter struct {
	Rule  RulesetRuleType
	Name  string
	Value string
}

// OmittedParameterChange は、宣言が省略したパラメータの、二つの観測値のあいだの変化。値は表記で持つ。
type OmittedParameterChange struct {
	Rule          RulesetRuleType
	Name          string
	Before, After string
}

// ParametersResetBy は、観測した値 r を宣言の値 declared に置き換える更新が、declared の省略によって変えるパラメータを、宣言のルールの順、同じルールの中では一覧の順で返す。
// 宣言にない型のルールは、外れることが宣言した変更のため、当たらない。
func (r Ruleset) ParametersResetBy(declared Ruleset) []ResetParameter {
	var reset []ResetParameter
	for _, rule := range declared.Rules {
		for _, p := range omittedParametersOf(rule) {
			if v, ok := r.optionalParameterValue(p); ok && v.comparison != p.whenOmitted {
				reset = append(reset, ResetParameter{Rule: p.rule, Name: p.name, Value: v.text})
			}
		}
	}
	return reset
}

// OmittedParameterChangesTo は、宣言の値 declared が省略したパラメータについて、観測した値 r から after への変化を、ParametersResetBy と同じ並びで返す。
// 宣言したパラメータは期待状態の確認が比べるため、返さない。どちらかの値が同じ型のルールを持たなければ、比べる値がなく返さない。
func (r Ruleset) OmittedParameterChangesTo(after, declared Ruleset) []OmittedParameterChange {
	var changes []OmittedParameterChange
	for _, rule := range declared.Rules {
		for _, p := range omittedParametersOf(rule) {
			b, beforeOK := r.optionalParameterValue(p)
			a, afterOK := after.optionalParameterValue(p)
			if beforeOK && afterOK && b.comparison != a.comparison {
				changes = append(changes, OmittedParameterChange{Rule: p.rule, Name: p.name, Before: b.text, After: a.text})
			}
		}
	}
	return changes
}
