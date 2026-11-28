package github

import (
	"slices"

	"github.com/tmknom/reco/internal/setting"
)

// resourceScope は、一回の観測でリポジトリリソースごとの読み取りを送るリポジトリリソースの範囲。
// Environment では個別パターンと変数の一覧、Ruleset では値の詳細の読み取りを、範囲にあるリポジトリリソースにだけ送る。
// environmentDeletions は、削除で失われるものを示すために変数と Secret の一覧まで読む Environment で、all に含めない。
type resourceScope struct {
	all                  bool
	environments         []setting.EnvironmentName
	environmentDeletions []setting.EnvironmentName
	rulesets             []setting.RulesetName
}

// allResources は、一覧のすべてのリポジトリリソースを範囲とする。
func allResources() resourceScope {
	return resourceScope{all: true}
}

// scopeOfNamed は、種類ごとに、named が名指したどれかと同じ名前のリポジトリリソースだけを範囲とする。
// 削除で失われるものまで読む Environment は、削除で失われる値が個別パターンを含むため、個別パターンと変数を読む範囲にも入れる。
func scopeOfNamed(named NamedResources) resourceScope {
	return resourceScope{
		environments:         slices.Concat(named.Environments, named.EnvironmentsReadForDeletion),
		environmentDeletions: named.EnvironmentsReadForDeletion,
		rulesets:             named.Rulesets,
	}
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

// includesEnvironmentDeletion は、一覧の綴り observed の Environment が、削除で失われるものまで読む範囲にあるかを返す。
// all を見ないのは、一覧のすべてを読む状態確認（show）に Secret の要求を送らせないためである。
// 比較は includesEnvironment と同じく、Environments.Lookup と同じ比較で「同じ」のときだけ含める。
func (s resourceScope) includesEnvironmentDeletion(observed string) bool {
	return slices.ContainsFunc(s.environmentDeletions, func(name setting.EnvironmentName) bool {
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
