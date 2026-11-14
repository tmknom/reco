package setting

import (
	"strconv"
	"strings"
)

// SelectedActionsCriteria は、選択方式の Actions 許可条件の3項目をまとめた値。
type SelectedActionsCriteria struct {
	GitHubOwnedAllowed bool
	VerifiedAllowed    bool
	// PatternsAllowed は受け取った順のまま持つ。nil と長さ 0 のスライスは、どちらも空の集合を表す。
	PatternsAllowed []string
}

// PatternsAllowedString は、許可パターンを ["a", "b"] の表記で返す。
// パターンは任意の文字を含み得るため、strconv.Quote で制御文字をエスケープし、一つのパターンが複数の行に見えないようにする。
func (c SelectedActionsCriteria) PatternsAllowedString() string {
	quoted := make([]string, len(c.PatternsAllowed))
	for i, p := range c.PatternsAllowed {
		quoted[i] = strconv.Quote(p)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
