package github

import (
	"slices"

	"github.com/tmknom/reco/internal/setting"
)

// environmentScope は、一回の観測で Environment ごとの読み取り（個別パターンと変数の一覧）を送る Environment の範囲。
type environmentScope struct {
	all   bool
	names []setting.EnvironmentName
}

// allEnvironments は、一覧のすべての Environment を範囲とする。
func allEnvironments() environmentScope {
	return environmentScope{all: true}
}

// namedEnvironments は、names のどれかと同じ名前の Environment だけを範囲とする。
func namedEnvironments(names []setting.EnvironmentName) environmentScope {
	return environmentScope{names: names}
}

// includes は、一覧の綴り observed の Environment が範囲にあるかを返す。
// Environments.Lookup と同じ比較で「同じ」のときだけ含めるため、Lookup が返す Environment は必ず範囲にある。
// 同一性を判定できない名前は含めない。同じ名前がなければ Lookup は観測不能を返し、その中の値を読まないため。
func (s environmentScope) includes(observed string) bool {
	if s.all {
		return true
	}
	return slices.ContainsFunc(s.names, func(name setting.EnvironmentName) bool {
		return name.Compare(observed) == setting.EnvironmentNameSame
	})
}
