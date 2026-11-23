package setting

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
	RulesRemoved        []RulesetRule
	RulesAdded          []RulesetRule
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
func (r Ruleset) DifferenceTo(other Ruleset) RulesetDifference {
	from, to := r.Conditions.refNameOrEmpty(), other.Conditions.refNameOrEmpty()
	return RulesetDifference{
		TargetChanged:       r.Target != other.Target,
		EnforcementChanged:  r.Enforcement != other.Enforcement,
		RefNameChanged:      (r.Conditions.RefName == nil) != (other.Conditions.RefName == nil),
		IncludeRemoved:      elementsNotIn(from.Include, to.Include),
		IncludeAdded:        elementsNotIn(to.Include, from.Include),
		ExcludeRemoved:      elementsNotIn(from.Exclude, to.Exclude),
		ExcludeAdded:        elementsNotIn(to.Exclude, from.Exclude),
		BypassActorsRemoved: elementsNotIn(r.BypassActors, other.BypassActors),
		BypassActorsAdded:   elementsNotIn(other.BypassActors, r.BypassActors),
		RulesRemoved:        elementsNotIn(r.Rules, other.Rules),
		RulesAdded:          elementsNotIn(other.Rules, r.Rules),
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

// elementsNotIn は、elements のうち others にない要素を、elements の並びで重複を除いて返す。
// 観測値の要素数は reco が制御しないため、map で引いて要素数の2乗の時間をかけない。
func elementsNotIn[T comparable](elements, others []T) []T {
	seen := make(map[T]struct{}, len(others))
	for _, e := range others {
		seen[e] = struct{}{}
	}
	var missing []T
	for _, e := range elements {
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		missing = append(missing, e)
	}
	return missing
}
