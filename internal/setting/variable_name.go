package setting

import (
	"errors"
	"fmt"
	"strings"
)

// VariableName は、設定宣言で書ける変数の名前。英小文字を含まないため、GitHub が保存する綴りと一致する。
// フィールドを公開しないのは、ゼロ値のほかは ParseVariableName の検証を通った値しか作れない状態にするため。
type VariableName struct {
	name string
}

// ParseVariableName は、s が YAML 設定仕様の変数の名前の規則を満たすことを確かめて変数の名前にする。
// GITHUB_ の誤りを英小文字の誤りより先に判定するのは、大文字にした綴りも受け付けない名前に、その綴りを直し方として示さないため。
func ParseVariableName(s string) (VariableName, error) {
	folded := FoldVariableName(s)
	switch {
	case s == "":
		return VariableName{}, errors.New("must not be empty")
	case !isVariableNameCharacters(s):
		return VariableName{}, errors.New("must consist of ASCII letters, digits, and underscores")
	case '0' <= s[0] && s[0] <= '9':
		return VariableName{}, errors.New("must not start with a digit")
	case strings.HasPrefix(folded, "GITHUB_"):
		return VariableName{}, errors.New("must not start with GITHUB_")
	case folded != s:
		return VariableName{}, fmt.Errorf("must be upper case; write %q", folded)
	}
	return VariableName{name: s}, nil
}

// String は、宣言の綴りのままの名前を返す。
func (n VariableName) String() string {
	return n.name
}

// Matches は、GitHub が返した観測の名前 observed が、宣言の名前 n と同じ変数を指すかを返す。
func (n VariableName) Matches(observed string) bool {
	return FoldVariableName(observed) == n.name
}

// FoldVariableName は、変数の名前が同じかどうかを判定する鍵として、s の ASCII の英小文字だけを大文字にした綴りを返す。
// GitHub は変数の名前を大文字にして保存するため、鍵は宣言の名前の綴りと一致する。
func FoldVariableName(s string) string {
	return upperASCII(s)
}

func isVariableNameCharacters(s string) bool {
	for i := range len(s) {
		c := s[i]
		if c != '_' && !('0' <= c && c <= '9') && !('A' <= c && c <= 'Z') && !('a' <= c && c <= 'z') {
			return false
		}
	}
	return true
}
