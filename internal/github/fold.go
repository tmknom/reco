package github

import (
	"strings"
	"unicode"
)

// simpleFoldKey は、strings.EqualFold が同じとみなす名前どうしで、またそのときに限って一致する鍵を返す。
// 一覧の要素数の2乗にならずに重複を検査するため。EqualFold は rune ごとに unicode.SimpleFold の軌道が同じかで比べるため、
// 各 rune を軌道の最小の rune に置き換える。大文字化と小文字化の往復では、軌道の違う U+0131（LATIN SMALL LETTER DOTLESS I）と i が同じになり、使えない。
func simpleFoldKey(name string) string {
	return strings.Map(func(r rune) rune {
		smallest := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			smallest = min(smallest, f)
		}
		return smallest
	}, name)
}
