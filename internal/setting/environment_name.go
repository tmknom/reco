package setting

import (
	"errors"
	"strings"
)

// EnvironmentName は、設定宣言で書ける Environment の名前。ドメインモデルの同一性を判定できる名前に限る。
// フィールドを公開しないのは、ゼロ値のほかは ParseEnvironmentName の検証を通った値しか作れない状態にするため。
// 綴りは宣言のまま持ち、名前が同じかどうかは EqualFold と Compare で判定する。
type EnvironmentName struct {
	name string
}

// EnvironmentNameComparison は、宣言の名前と観測の名前を突き合わせた結果。
// ゼロ値を定義済みの結果と取り違えないよう 1 から始める。
type EnvironmentNameComparison int

const (
	// EnvironmentNameSame は、GitHub が同じ Environment とみなす名前であること。
	EnvironmentNameSame EnvironmentNameComparison = iota + 1
	// EnvironmentNameDifferent は、GitHub が別の Environment とみなす名前であること。
	EnvironmentNameDifferent
	// EnvironmentNameUndetermined は、GitHub が同じとみなすかを reco が判定できないこと。
	EnvironmentNameUndetermined
)

// maxEnvironmentNameLength は、GitHub の公式資料が記載する Environment の名前の長さの上限。
const maxEnvironmentNameLength = 255

// ParseEnvironmentName は、s が YAML 設定仕様の Environment の名前の規則を満たすことを確かめて Environment の名前にする。
func ParseEnvironmentName(s string) (EnvironmentName, error) {
	switch {
	case s == "":
		return EnvironmentName{}, errors.New("must not be empty")
	case !isPrintableASCII(s):
		return EnvironmentName{}, errors.New("must consist of printable ASCII characters")
	case strings.TrimSpace(s) != s:
		return EnvironmentName{}, errors.New("must not have leading or trailing spaces")
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
	return equalFoldASCII(n.name, other.name)
}

// Compare は、宣言の名前 n と、GitHub が返した観測の名前を突き合わせる。
// 観測の名前が同一性を判定できる名前でなく、綴りも一致しないときは、GitHub が同じとみなし得るため判定できないとする。
func (n EnvironmentName) Compare(observed string) EnvironmentNameComparison {
	switch {
	case equalFoldASCII(n.name, observed):
		return EnvironmentNameSame
	case isComparableEnvironmentName(observed):
		return EnvironmentNameDifferent
	default:
		return EnvironmentNameUndetermined
	}
}

func isComparableEnvironmentName(s string) bool {
	return isPrintableASCII(s) && strings.TrimSpace(s) == s
}

func isPrintableASCII(s string) bool {
	for i := range len(s) {
		if s[i] < ' ' || s[i] > '~' {
			return false
		}
	}
	return true
}

// equalFoldASCII は、ASCII の英字だけを大文字小文字を区別せずに比べる。
// strings.EqualFold は Unicode の単純な畳み込みで ſ と s を同じとみなし、同一性を判定できる名前の比較を広げてしまうため使わない。
func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range len(a) {
		if lowerASCII(a[i]) != lowerASCII(b[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}
