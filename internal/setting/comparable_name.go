package setting

import (
	"errors"
	"strings"
)

// NameComparison は、宣言の名前と観測の名前を突き合わせた結果。
// ゼロ値を定義済みの結果と取り違えないよう 1 から始める。
type NameComparison int

const (
	// NameSame は、GitHub が同じリポジトリリソースとみなす名前であること。
	NameSame NameComparison = iota + 1
	// NameDifferent は、GitHub が別のリポジトリリソースとみなす名前であること。
	NameDifferent
	// NameUndetermined は、GitHub が同じとみなすかを reco が判定できないこと。
	NameUndetermined
)

// checkComparableName は、s がドメインモデルの同一性を判定できる名前であることを確かめる。
// 誤りは、空、印字可能 ASCII 以外、前後の空白の順に示す。
func checkComparableName(s string) error {
	switch {
	case s == "":
		return errors.New("must not be empty")
	case !isPrintableASCII(s):
		return errors.New("must consist of printable ASCII characters")
	case strings.TrimSpace(s) != s:
		return errors.New("must not have leading or trailing spaces")
	}
	return nil
}

// compareName は、同一性を判定できる宣言の名前 declared と、GitHub が返した観測の名前を突き合わせる。
// 宣言の名前は ASCII だけからなるため、ASCII の英字の大文字小文字を区別せずに比べれば、GitHub が同じとみなす観測の名前を見落とさない。
// 観測の名前が同一性を判定できる名前でなく、綴りも一致しないときは、GitHub が同じとみなし得るため判定できないとする。
func compareName(declared, observed string) NameComparison {
	switch {
	case upperASCII(declared) == upperASCII(observed):
		return NameSame
	case isComparableName(observed):
		return NameDifferent
	default:
		return NameUndetermined
	}
}

func isComparableName(s string) bool {
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
