package github

import (
	"slices"

	"github.com/tmknom/reco/internal/setting"
)

// resourceScope は、一回の観測でリポジトリリソースごとの読み取りを送るリポジトリリソースの範囲。
// Environment では個別パターンと変数の一覧、Ruleset では値の詳細の読み取りを、範囲にあるリポジトリリソースにだけ送る。
type resourceScope struct {
	all          bool
	environments []setting.EnvironmentName
	rulesets     []setting.RulesetName
}

// allResources は、一覧のすべてのリポジトリリソースを範囲とする。
func allResources() resourceScope {
	return resourceScope{all: true}
}

// scopeOfNamed は、種類ごとに、named が名指したどれかと同じ名前のリポジトリリソースだけを範囲とする。
func scopeOfNamed(named NamedResources) resourceScope {
	return resourceScope{environments: named.Environments, rulesets: named.Rulesets}
}

// includesEnvironment は、一覧の綴り observed の Environment が範囲にあるかを返す。
// Environments.Lookup と同じ比較で「同じ」のときだけ含めるため、Lookup が返す Environment は必ず範囲にある。
// 同一性を判定できない名前は含めない。同じ名前がなければ Lookup は観測不能を返し、その中の値を読まないため。
func (s resourceScope) includesEnvironment(observed string) bool {
	if s.all {
		return true
	}
	return slices.ContainsFunc(s.environments, func(name setting.EnvironmentName) bool {
		return name.Compare(observed) == setting.NameSame
	})
}

// includesRuleset は、一覧の綴り observed の Ruleset が範囲にあるかを返す。
// Rulesets.Lookup と同じ比較で「同じ」のときだけ含めるため、Lookup が存在するとして返す Ruleset は必ず値を読んでいる。
// 同一性を判定できない名前は含めない。同じ名前がなければ Lookup は観測不能を返し、その値を読まないため。
func (s resourceScope) includesRuleset(observed string) bool {
	if s.all {
		return true
	}
	return slices.ContainsFunc(s.rulesets, func(name setting.RulesetName) bool {
		return name.Compare(observed) == setting.NameSame
	})
}
