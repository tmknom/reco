package setting

import (
	"cmp"
	"slices"
	"strconv"
	"strings"
)

// DeploymentBranchPolicy は、Environment のデプロイ元制限の値。方式と、方式が selected のときに限って持つ個別パターンの集合からなる。
type DeploymentBranchPolicy struct {
	Mode DeploymentBranchPolicyMode
	// Patterns は受け取った順のまま持つ。方式が selected でなければ持たない。
	Patterns []DeploymentPattern
}

// DeploymentBranchPolicyMode は、デプロイ元制限の方式。値は show の表示の文字列。
type DeploymentBranchPolicyMode string

const (
	DeploymentBranchPolicyAll               DeploymentBranchPolicyMode = "all"
	DeploymentBranchPolicyProtectedBranches DeploymentBranchPolicyMode = "protected_branches"
	DeploymentBranchPolicySelected          DeploymentBranchPolicyMode = "selected"
)

// ParseDeploymentBranchPolicyMode は s が定義された方式に完全一致するときだけ、その方式と真を返す。
func ParseDeploymentBranchPolicyMode(s string) (DeploymentBranchPolicyMode, bool) {
	switch m := DeploymentBranchPolicyMode(s); m {
	case DeploymentBranchPolicyAll, DeploymentBranchPolicyProtectedBranches, DeploymentBranchPolicySelected:
		return m, true
	default:
		return "", false
	}
}

// DeploymentPattern は、デプロイ元制限の個別パターン。同じかどうかは種類と名前の組で決まり、名前は大文字と小文字を区別する。
type DeploymentPattern struct {
	Type DeploymentPatternType
	Name string
}

// DeploymentPatternType は、個別パターンの種類。値は GitHub REST API の文字列で、表示にもそのまま使う。
type DeploymentPatternType string

const (
	DeploymentPatternBranch DeploymentPatternType = "branch"
	DeploymentPatternTag    DeploymentPatternType = "tag"
)

// ParseDeploymentPatternType は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseDeploymentPatternType(s string) (DeploymentPatternType, bool) {
	switch t := DeploymentPatternType(s); t {
	case DeploymentPatternBranch, DeploymentPatternTag:
		return t, true
	default:
		return "", false
	}
}

// PatternsString は、個別パターンを [branch "a", tag "b"] の表記で返す。
// 名前は任意の文字を含み得るため、strconv.Quote で制御文字をエスケープし、一つのパターンが複数の行に見えないようにする。
func (p DeploymentBranchPolicy) PatternsString() string {
	formatted := make([]string, len(p.Patterns))
	for i, pattern := range p.Patterns {
		formatted[i] = string(pattern.Type) + " " + strconv.Quote(pattern.Name)
	}
	return "[" + strings.Join(formatted, ", ") + "]"
}

// equal は、方式が同じで、個別パターンが種類と名前の組の集合として同じかを返す。
func (p DeploymentBranchPolicy) equal(other DeploymentBranchPolicy) bool {
	return p.Mode == other.Mode && slices.Equal(deploymentPatternSet(p.Patterns), deploymentPatternSet(other.Patterns))
}

// deploymentPatternSet は、保持する並びを表示と送信に使うため、引数を書き換えずに写しを整列する。
func deploymentPatternSet(patterns []DeploymentPattern) []DeploymentPattern {
	return slices.Compact(slices.SortedFunc(slices.Values(patterns), func(a, b DeploymentPattern) int {
		return cmp.Or(cmp.Compare(a.Type, b.Type), cmp.Compare(a.Name, b.Name))
	}))
}
