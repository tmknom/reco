package setting

import (
	"slices"
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

// equal は、YAML 設定仕様の許可パターンの照合規則に従って、二つの値が同じかどうかを返す。
func (c SelectedActionsCriteria) equal(other SelectedActionsCriteria) bool {
	return c.GitHubOwnedAllowed == other.GitHubOwnedAllowed &&
		c.VerifiedAllowed == other.VerifiedAllowed &&
		slices.Equal(patternSet(c.PatternsAllowed), patternSet(other.PatternsAllowed))
}

// patternSet は、保持する並びを表示と送信に使うため、引数を書き換えずに写しを整列する。
func patternSet(patterns []string) []string {
	return slices.Compact(slices.Sorted(slices.Values(patterns)))
}
