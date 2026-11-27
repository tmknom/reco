package setting

import (
	"slices"
	"strconv"
)

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

// doNotEnforceOnCreateWhenOmitted は、rules を置き換える更新で do_not_enforce_on_create を送らなかったときに GitHub が保存する値（GitHub Rules API 仕様 7.3）。
const doNotEnforceOnCreateWhenOmitted = false

// ParametersResetBy は、観測した値 r を宣言の値 declared に置き換える更新が、declared の省略によって変えるパラメータを返す。
// 宣言にない型のルールは、外れることが宣言した変更のため、当たらない。
func (r Ruleset) ParametersResetBy(declared Ruleset) []ResetParameter {
	if !declared.omitsOptionalParameters() {
		return nil
	}
	if v, ok := r.doNotEnforceOnCreate(); ok && v != doNotEnforceOnCreateWhenOmitted {
		return []ResetParameter{{Rule: RulesetRuleRequiredStatusChecks, Name: "do_not_enforce_on_create", Value: strconv.FormatBool(v)}}
	}
	return nil
}

// OmittedParameterChangesTo は、宣言の値 declared が省略したパラメータについて、観測した値 r から after への変化を返す。
// 宣言したパラメータは期待状態の確認が比べるため、返さない。どちらかの値が同じ型のルールを持たなければ、比べる値がなく返さない。
func (r Ruleset) OmittedParameterChangesTo(after, declared Ruleset) []OmittedParameterChange {
	if !declared.omitsOptionalParameters() {
		return nil
	}
	b, beforeOK := r.doNotEnforceOnCreate()
	a, afterOK := after.doNotEnforceOnCreate()
	if !beforeOK || !afterOK || b == a {
		return nil
	}
	return []OmittedParameterChange{{Rule: RulesetRuleRequiredStatusChecks, Name: "do_not_enforce_on_create", Before: strconv.FormatBool(b), After: strconv.FormatBool(a)}}
}

// omitsOptionalParameters は、r のどれかのルールが、省略できるパラメータを省略しているかを返す。省略するのは宣言の値だけで、観測した値は必ず持つ。
func (r Ruleset) omitsOptionalParameters() bool {
	return slices.ContainsFunc(r.Rules, func(rule RulesetRule) bool {
		return rule.ruleType == RulesetRuleRequiredStatusChecks && rule.statusChecks.DoNotEnforceOnCreate == nil
	})
}

// doNotEnforceOnCreate は、観測した値 r の最初の required_status_checks のルールの do_not_enforce_on_create を返す。
// GitHub は同じ型のルールを二つ持つ Ruleset を作らせないため、最初の一つで足りる。ルールがないか値を持たなければ偽を返す。
func (r Ruleset) doNotEnforceOnCreate() (bool, bool) {
	for _, rule := range r.Rules {
		if p := rule.statusChecks.DoNotEnforceOnCreate; rule.ruleType == RulesetRuleRequiredStatusChecks && p != nil {
			return *p, true
		}
	}
	return false, false
}
