package setting

import "errors"

// EnvironmentName は、ドメインモデルの同一性を判定できる名前に限った Environment の名前。宣言と観測の文脈が同じ定義で使う。
// フィールドを公開しないのは、ゼロ値のほかは ParseEnvironmentName の検証を通った値しか作れない状態にするため。
// 綴りは与えられたまま持ち、名前が同じかどうかは EqualFold と Compare で判定する。
type EnvironmentName struct {
	name string
}

// maxEnvironmentNameLength は、GitHub の公式資料が記載する Environment の名前の長さの上限。
const maxEnvironmentNameLength = 255

// ParseEnvironmentName は、s が YAML 設定仕様の Environment の名前の規則を満たすことを確かめて Environment の名前にする。
func ParseEnvironmentName(s string) (EnvironmentName, error) {
	if err := checkComparableName(s); err != nil {
		return EnvironmentName{}, err
	}
	switch {
	case s == "." || s == "..":
		return EnvironmentName{}, errors.New(`must not be "." or ".."`)
	case len(s) > maxEnvironmentNameLength:
		return EnvironmentName{}, errors.New("must be at most 255 characters")
	}
	return EnvironmentName{name: s}, nil
}

// String は、宣言の綴りのままの名前を返す。
func (n EnvironmentName) String() string {
	return n.name
}

// EqualFold は、二つの宣言の名前を GitHub が同じ Environment とみなすかを返す。
// どちらも同一性を判定できる名前のため、ASCII の英字の大文字小文字を区別せずに比べれば足りる。
func (n EnvironmentName) EqualFold(other EnvironmentName) bool {
	return upperASCII(n.name) == upperASCII(other.name)
}

// Compare は、宣言の名前 n と、GitHub が返した観測の名前を突き合わせる。
func (n EnvironmentName) Compare(observed string) NameComparison {
	return compareName(n.name, observed)
}
