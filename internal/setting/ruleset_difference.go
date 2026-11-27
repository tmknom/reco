package setting

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// RulesetDifference は、二つの Ruleset の値のあいだの値の差。変わる項目と、外れる要素・加わる要素からなる。
// パターン、Bypass、ルールは集合として比べるため、順序と重複の違いは差にならない。
type RulesetDifference struct {
	TargetChanged      bool
	EnforcementChanged bool
	// RefNameChanged は、ref_name の有無が変わること。ないほうの include と exclude は空集合として要素の差をとる。
	RefNameChanged bool
	// Removed は元の値の並びで、Added は相手の値の並びで、それぞれ重複を除いて持つ。
	IncludeRemoved      []string
	IncludeAdded        []string
	ExcludeRemoved      []string
	ExcludeAdded        []string
	BypassActorsRemoved []BypassActor
	BypassActorsAdded   []BypassActor
	// RulesRemoved と RulesAdded は、比べなかった省略できるパラメータも持ったままの要素である。
	RulesRemoved []RulesetRule
	RulesAdded   []RulesetRule
}

// Empty は、値の差がないこと、つまり二つの値が同じ値であることを返す。
func (d RulesetDifference) Empty() bool {
	return !d.TargetChanged && !d.EnforcementChanged && !d.RefNameChanged &&
		len(d.IncludeRemoved) == 0 && len(d.IncludeAdded) == 0 &&
		len(d.ExcludeRemoved) == 0 && len(d.ExcludeAdded) == 0 &&
		len(d.BypassActorsRemoved) == 0 && len(d.BypassActorsAdded) == 0 &&
		len(d.RulesRemoved) == 0 && len(d.RulesAdded) == 0
}

// DifferenceTo は、r を other に変えるときの値の差を返す。
// 等価判定 (equal) も同じ差から導き、同じ値と判定した二つの値の表示に差の行が出る食い違いを起こさない。
// 省略できるパラメータは、二つの値の両方が持つときだけ比べる。宣言は同じ型のルールを一つしか持てず、観測した値は必ず持つため、値の全体で決めれば足りる。
func (r Ruleset) DifferenceTo(other Ruleset) RulesetDifference {
	from, to := r.Conditions.refNameOrEmpty(), other.Conditions.refNameOrEmpty()
	compareOptional := !r.omitsOptionalParameters() && !other.omitsOptionalParameters()
	ruleKey := func(rule RulesetRule) ruleComparisonKey { return rule.comparisonKey(compareOptional) }
	return RulesetDifference{
		TargetChanged:       r.Target != other.Target,
		EnforcementChanged:  r.Enforcement != other.Enforcement,
		RefNameChanged:      (r.Conditions.RefName == nil) != (other.Conditions.RefName == nil),
		IncludeRemoved:      elementsNotIn(from.Include, to.Include, identity),
		IncludeAdded:        elementsNotIn(to.Include, from.Include, identity),
		ExcludeRemoved:      elementsNotIn(from.Exclude, to.Exclude, identity),
		ExcludeAdded:        elementsNotIn(to.Exclude, from.Exclude, identity),
		BypassActorsRemoved: elementsNotIn(r.BypassActors, other.BypassActors, identity),
		BypassActorsAdded:   elementsNotIn(other.BypassActors, r.BypassActors, identity),
		RulesRemoved:        elementsNotIn(r.Rules, other.Rules, ruleKey),
		RulesAdded:          elementsNotIn(other.Rules, r.Rules, ruleKey),
	}
}

// equal は、r と other が同じ値かを返す。
func (r Ruleset) equal(other Ruleset) bool {
	return r.DifferenceTo(other).Empty()
}

// refNameOrEmpty は、ref_name のない条件を、include と exclude がどちらも空の条件として返す。要素の差をとるためだけの読み替えで、条件そのものは変えない。
func (c RulesetConditions) refNameOrEmpty() RefNameCondition {
	if c.RefName == nil {
		return RefNameCondition{}
	}
	return *c.RefName
}

// RequiredStatusChecksDifference は、二つの required_status_checks のパラメータのあいだの差。
type RequiredStatusChecksDifference struct {
	// ChecksRemoved は元のパラメータの並びで、ChecksAdded は相手のパラメータの並びで、それぞれ重複を除いて持つ。
	ChecksRemoved       []StatusCheck
	ChecksAdded         []StatusCheck
	StrictPolicyChanged bool
	// DoNotEnforceOnCreateChanged は、両方が持ち、値が違うときだけ真になる。
	DoNotEnforceOnCreateChanged bool
}

// Empty は、パラメータの差がないことを返す。
func (d RequiredStatusChecksDifference) Empty() bool {
	return len(d.ChecksRemoved) == 0 && len(d.ChecksAdded) == 0 && !d.StrictPolicyChanged && !d.DoNotEnforceOnCreateChanged
}

// DifferenceTo は、p を other に変えるときのパラメータの差を返す。チェックは集合として比べる。
// 差が空であることは、二つのルールを同じ値とする判定 (comparisonKey の一致) と同じでなければならない。
func (p RequiredStatusChecksParameters) DifferenceTo(other RequiredStatusChecksParameters) RequiredStatusChecksDifference {
	return RequiredStatusChecksDifference{
		ChecksRemoved:       elementsNotIn(p.RequiredStatusChecks, other.RequiredStatusChecks, identity),
		ChecksAdded:         elementsNotIn(other.RequiredStatusChecks, p.RequiredStatusChecks, identity),
		StrictPolicyChanged: p.StrictRequiredStatusChecksPolicy != other.StrictRequiredStatusChecksPolicy,
		DoNotEnforceOnCreateChanged: p.DoNotEnforceOnCreate != nil && other.DoNotEnforceOnCreate != nil &&
			*p.DoNotEnforceOnCreate != *other.DoNotEnforceOnCreate,
	}
}

// ruleComparisonKey は、二つのルールが同じ値かを決める項目を、比較演算子で比べられる形にしたもの。
type ruleComparisonKey struct {
	ruleType    RulesetRuleType
	pullRequest PullRequestParameters
	// statusChecks は、順序と重複を除いたチェックの集合の表記である。
	statusChecks string
	strictPolicy bool
	// doNotEnforceOnCreate は、比べないときと持たないときは空になる。
	doNotEnforceOnCreate string
}

// comparisonKey は、r の比較用のキーを返す。compareOptional が偽なら、省略できるパラメータをキーに含めない。
func (r RulesetRule) comparisonKey(compareOptional bool) ruleComparisonKey {
	key := ruleComparisonKey{ruleType: r.ruleType, pullRequest: r.pullRequest, strictPolicy: r.statusChecks.StrictRequiredStatusChecksPolicy}
	key.statusChecks = statusCheckSetText(r.statusChecks.RequiredStatusChecks)
	if p := r.statusChecks.DoNotEnforceOnCreate; compareOptional && p != nil {
		key.doNotEnforceOnCreate = strconv.FormatBool(*p)
	}
	return key
}

// statusCheckSetText は、チェックの集合を、並べ替えて重複を除いた一つの文字列にする。
// context は任意の文字を含み得るため、strconv.Quote で囲み、区切りと取り違えないようにする。
func statusCheckSetText(checks []StatusCheck) string {
	sorted := slices.Clone(checks)
	slices.SortFunc(sorted, func(a, b StatusCheck) int {
		return cmp.Or(strings.Compare(a.Context, b.Context), cmp.Compare(a.IntegrationID, b.IntegrationID))
	})
	var b strings.Builder
	for _, c := range slices.Compact(sorted) {
		b.WriteString(strconv.Quote(c.Context) + ":" + strconv.FormatInt(c.IntegrationID, 10) + ",")
	}
	return b.String()
}

// elementsNotIn は、elements のうち、key が others のどれとも同じにならない要素を、elements の並びで key の重複を除いて返す。
// 観測値の要素数は reco が制御しないため、map で引いて要素数の2乗の時間をかけない。
func elementsNotIn[T any, K comparable](elements, others []T, key func(T) K) []T {
	seen := make(map[K]struct{}, len(others))
	for _, e := range others {
		seen[key(e)] = struct{}{}
	}
	var missing []T
	for _, e := range elements {
		k := key(e)
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		missing = append(missing, e)
	}
	return missing
}

// identity は、比較演算子で比べられる要素を、そのままキーにする。
func identity[T comparable](e T) T {
	return e
}
