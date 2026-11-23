package setting

// RulesetName は、ドメインモデルの同一性を判定できる名前に限った Ruleset の名前。宣言と観測の文脈が同じ定義で使う。
// フィールドを公開しないのは、ゼロ値のほかは ParseRulesetName の検証を通った値しか作れない状態にするため。
// 綴りは与えられたまま持ち、名前が同じかどうかは EqualFold と Compare で判定する。
type RulesetName struct {
	name string
}

// ParseRulesetName は、s が YAML 設定仕様の Ruleset の名前の規則を満たすことを確かめて Ruleset の名前にする。
// Environment の名前と違い、"." と ".." と長さを確かめない。YAML 設定仕様の Ruleset の名前の規則がそれらを制限しないため。
func ParseRulesetName(s string) (RulesetName, error) {
	if err := checkComparableName(s); err != nil {
		return RulesetName{}, err
	}
	return RulesetName{name: s}, nil
}

// String は、宣言の綴りのままの名前を返す。
func (n RulesetName) String() string {
	return n.name
}

// EqualFold は、二つの宣言の名前を GitHub が同じ Ruleset とみなすかを返す。
// どちらも同一性を判定できる名前のため、ASCII の英字の大文字小文字を区別せずに比べれば足りる。
func (n RulesetName) EqualFold(other RulesetName) bool {
	return upperASCII(n.name) == upperASCII(other.name)
}

// Compare は、宣言の名前 n と、GitHub が返した観測の名前を突き合わせる。
func (n RulesetName) Compare(observed string) NameComparison {
	return compareName(n.name, observed)
}
