package setting

import "strconv"

// optionalParameter は、ルールの省略できるパラメータ一つの定義。
// 照合、適用可能性、更新前後の比較がパラメータごとに同じ規則を当てるため、定義を一覧に集め、どの判定もこの一覧を回す。
type optionalParameter struct {
	rule RulesetRuleType
	name string
	// whenOmitted は、rules を置き換える更新で送らなかったときに GitHub が保存する値の、比較用の表記（GitHub Rules API 仕様の pull_request と required_status_checks のパラメータ）。
	whenOmitted string
	// valueOf は、ルールがこのパラメータを持てば、その値と真を返す。
	valueOf func(RulesetRule) (optionalParameterValue, bool)
}

// optionalParameterValue は、省略できるパラメータの値。comparison は順序と重複を除いた比較用の表記で、text は受け取った並びのままの表示の表記である。
type optionalParameterValue struct {
	comparison, text string
}

// optionalParameters は、省略できるパラメータを、ルール型ごとに YAML 設定仕様のパラメータの表の並びで持つ。
var optionalParameters = []optionalParameter{
	{
		rule:        RulesetRulePullRequest,
		name:        "allowed_merge_methods",
		whenOmitted: allowedMergeMethodSetText([]AllowedMergeMethod{AllowedMergeMethodMerge, AllowedMergeMethodSquash, AllowedMergeMethodRebase}),
		valueOf: func(rule RulesetRule) (optionalParameterValue, bool) {
			methods := rule.pullRequest.AllowedMergeMethods
			if rule.ruleType != RulesetRulePullRequest || methods == nil {
				return optionalParameterValue{}, false
			}
			return optionalParameterValue{comparison: allowedMergeMethodSetText(methods), text: AllowedMergeMethodsText(methods)}, true
		},
	},
	{
		rule:        RulesetRulePullRequest,
		name:        "require_extra_approval_for_unattributed_changes",
		whenOmitted: strconv.FormatBool(true),
		valueOf: func(rule RulesetRule) (optionalParameterValue, bool) {
			if rule.ruleType != RulesetRulePullRequest {
				return optionalParameterValue{}, false
			}
			return boolParameterValue(rule.pullRequest.RequireExtraApprovalForUnattributedChanges)
		},
	},
	{
		rule:        RulesetRuleRequiredStatusChecks,
		name:        "do_not_enforce_on_create",
		whenOmitted: strconv.FormatBool(false),
		valueOf: func(rule RulesetRule) (optionalParameterValue, bool) {
			if rule.ruleType != RulesetRuleRequiredStatusChecks {
				return optionalParameterValue{}, false
			}
			return boolParameterValue(rule.statusChecks.DoNotEnforceOnCreate)
		},
	},
}

// optionalParameterValue は、観測した値 r の、p のルール型の最初のルールが持つ p の値を返す。
// GitHub は同じ型のルールを二つ持つ Ruleset を作らせないため、最初の一つで足りる。ルールがないか値を持たなければ偽を返す。
func (r Ruleset) optionalParameterValue(p optionalParameter) (optionalParameterValue, bool) {
	for _, rule := range r.Rules {
		if v, ok := p.valueOf(rule); ok {
			return v, true
		}
	}
	return optionalParameterValue{}, false
}

// omits は、r のどれかの p のルール型のルールが、p を省略しているかを返す。
func (r Ruleset) omits(p optionalParameter) bool {
	for _, rule := range r.Rules {
		if _, ok := p.valueOf(rule); rule.ruleType == p.rule && !ok {
			return true
		}
	}
	return false
}

// comparedOptionalParameters は、a と b のどちらも省略していない省略できるパラメータを返す。
// 省略するのは宣言の値だけで、宣言は同じ型のルールを一つしか持てず、観測した値は必ず持つため、ルールごとでなく値ごとに決めれば足りる。
func comparedOptionalParameters(a, b Ruleset) []optionalParameter {
	var compared []optionalParameter
	for _, p := range optionalParameters {
		if !a.omits(p) && !b.omits(p) {
			compared = append(compared, p)
		}
	}
	return compared
}

// omittedParametersOf は、rule の型の省略できるパラメータのうち、rule が持たないものを一覧の順で返す。
func omittedParametersOf(rule RulesetRule) []optionalParameter {
	var omitted []optionalParameter
	for _, p := range optionalParameters {
		if _, ok := p.valueOf(rule); p.rule == rule.ruleType && !ok {
			omitted = append(omitted, p)
		}
	}
	return omitted
}

// boolParameterValue は、真偽値のパラメータの値を返す。nil なら偽を返す。
func boolParameterValue(v *bool) (optionalParameterValue, bool) {
	if v == nil {
		return optionalParameterValue{}, false
	}
	text := strconv.FormatBool(*v)
	return optionalParameterValue{comparison: text, text: text}, true
}
