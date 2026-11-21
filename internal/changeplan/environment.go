package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// environmentPlans は、一つの Environment の宣言 e を、宣言の名前で引いた観測結果 observed と照合し、存在、デプロイ元制限の順に計画を返す。
// 宣言は存在することの期待値を兼ねるため、存在の計画は必ず返す。
// デプロイ元制限は宣言したときだけ返し、存在しない Environment では属する先が存在しないことを理由とする比較不能になる。
func environmentPlans(e configuration.EnvironmentDeclaration, observed github.EnvironmentObservation) []SettingPlan {
	result := NotComparable
	if _, ok := observed.Existing(); ok {
		result = Match
	} else if observed.Absent() {
		result = Mismatch
	}
	plans := []SettingPlan{{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: e.Name}, ObservedEnvironment: observed, Result: result}}
	if e.DeploymentBranchPolicy != nil {
		policy := newSettingPlan(setting.EnvironmentDeploymentBranchPolicy, observed.DeploymentBranchPolicy(), setting.DeploymentBranchPolicyValue(*e.DeploymentBranchPolicy))
		policy.Unit.Environment = e.Name
		plans = append(plans, policy)
	}
	return plans
}

// environmentChangeOf は、Environment の変更予定 s を、それを含む論理的な設定変更にして返す。
// 存在しない Environment のデプロイ元制限の変更予定は、同じ名前の作成の論理的な設定変更にまとめ、単独の論理的な設定変更にしないため偽を返す。
// 更新操作の並びと基準はアーキテクチャ設計の非原子的な更新と中間状態に従う。
// 対にならない形（作成のない存在しないことを理由にした方式、存在が一致でない方式の不一致）と適用可能な比較不能の存在は、
// environmentPlans と judgeApplicability が作らないため、不変条件の破れとして panic する。
func (r RepositoryPlan) environmentChangeOf(s SettingPlan) (LogicalChange, bool) {
	if s.Unit.Key == setting.EnvironmentResource {
		return r.creationOf(s), true
	}
	existence, ok := r.settingPlanOf(setting.Unit{Key: setting.EnvironmentResource, Environment: s.Unit.Environment})
	switch {
	case isPolicyOfAbsentEnvironment(s) && ok && existence.IsCreation():
		return LogicalChange{}, false
	case s.Result == Mismatch && ok && existence.Result == Match:
		environment := RelevantSetting{Unit: existence.Unit, EnvironmentAtApproval: existence.ObservedEnvironment}
		return LogicalChange{DesiredState: []DesiredSetting{{Unit: s.Unit, Value: s.Desired}}, Operations: deploymentBranchPolicyOperations(s, environment)}, true
	}
	panic(fmt.Sprintf("changeplan: %s has no environment plan to pair with", s.Unit.String()))
}

// creationOf は、Environment の作成の変更予定 s と、同じ名前のデプロイ元制限の変更予定を一つにまとめた論理的な設定変更を返す。
// 存在しない Environment の方式を基準に載せないのは、存在しないことの基準が成立すれば同じ理由の観測不能に必ずなり、確かめる条件が重なるため。
// 作成でない Environment の変更予定と、存在しないことを理由にした比較不能でない方式との組は、environmentPlans と judgeApplicability が作らないため、
// 不変条件の破れとして panic する。
func (r RepositoryPlan) creationOf(s SettingPlan) LogicalChange {
	if !s.IsCreation() {
		panic(fmt.Sprintf("changeplan: %s is a planned change other than a creation", s.Unit.String()))
	}
	change := LogicalChange{DesiredState: []DesiredSetting{{Unit: s.Unit}}}
	existence := []RelevantSetting{{Unit: s.Unit, EnvironmentAtApproval: s.ObservedEnvironment}}
	policy, ok := r.settingPlanOf(setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: s.Unit.Environment})
	if !ok {
		change.Operations = []UpdateOperation{{Kind: EnvironmentCreation, Unit: s.Unit, RelevantSettings: existence}}
		return change
	}
	if !isPolicyOfAbsentEnvironment(policy) {
		panic(fmt.Sprintf("changeplan: creation of %s is paired with %s that is not of an absent environment", s.Unit.String(), policy.Unit.String()))
	}
	// 方式の単位は作成の綴りで作り、期待状態と更新操作が作成と同じ綴りの名前を示すようにする
	policyUnit := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: s.Unit.Environment}
	desired := policy.Desired.DeploymentBranchPolicy()
	change.DesiredState = append(change.DesiredState, DesiredSetting{Unit: policyUnit, Value: policy.Desired})
	change.Operations = []UpdateOperation{{Kind: EnvironmentCreationWithMode, Unit: s.Unit, Mode: desired.Mode, RelevantSettings: existence}}
	if desired.Mode != setting.DeploymentBranchPolicySelected {
		return change
	}
	created := RelevantSetting{Unit: s.Unit, CreatedByPrecedingOperation: true}
	change.Operations = append(change.Operations, deploymentPatternOperations(policyUnit, created, emptySelected(), nil, desired.Patterns)...)
	return change
}

// deploymentBranchPolicyOperations は、存在する Environment のデプロイ元制限の不一致 s を実現する更新操作を、実行順に返す。
// environment は、その Environment の存在と ID の基準である。
// 方式が selected でない期待値は、個別パターンを持っていても加える更新操作を並べない。宣言していない要素を送らないため。
func deploymentBranchPolicyOperations(s SettingPlan, environment RelevantSetting) []UpdateOperation {
	observed, _ := s.Observed.Value()
	before, after := observed.DeploymentBranchPolicy(), s.Desired.DeploymentBranchPolicy()
	if before.Mode == setting.DeploymentBranchPolicySelected && after.Mode == setting.DeploymentBranchPolicySelected {
		return deploymentPatternOperations(s.Unit, environment, before, before.PatternsNotIn(after), after.PatternsNotIn(before))
	}
	operations := []UpdateOperation{{
		Kind: DeploymentBranchPolicyModeUpdate, Unit: s.Unit, Mode: after.Mode,
		RelevantSettings: []RelevantSetting{environment, {Unit: s.Unit, ValueAtApproval: s.Observed}},
	}}
	if after.Mode != setting.DeploymentBranchPolicySelected {
		return operations
	}
	return append(operations, deploymentPatternOperations(s.Unit, environment, emptySelected(), nil, after.Patterns)...)
}

// emptySelected は、方式の PUT か作成のあとに予定する、方式 selected で個別パターンが空の値を返す。
func emptySelected() setting.DeploymentBranchPolicy {
	return setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{}}
}

// deploymentPatternOperations は、個別パターンを取り除く更新操作、加える更新操作の順に並べる。
// from は最初の更新操作の基準のデプロイ元制限で、各更新操作の基準は、直前までの更新操作を反映した値にする。
func deploymentPatternOperations(unit setting.Unit, environment RelevantSetting, from setting.DeploymentBranchPolicy, remove, add []setting.DeploymentPattern) []UpdateOperation {
	patterns := slices.Clone(from.Patterns)
	var operations []UpdateOperation
	next := func(kind UpdateOperationKind, pattern setting.DeploymentPattern) {
		base := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: patterns})
		operations = append(operations, UpdateOperation{
			Kind: kind, Unit: unit, Pattern: pattern,
			RelevantSettings: []RelevantSetting{environment, {Unit: unit, ValueAtApproval: github.Observed(base)}},
		})
	}
	for _, pattern := range remove {
		next(DeploymentPatternRemoval, pattern)
		patterns = slices.DeleteFunc(patterns, func(p setting.DeploymentPattern) bool { return p == pattern })
	}
	for _, pattern := range add {
		next(DeploymentPatternAddition, pattern)
		patterns = append(patterns, pattern)
	}
	return operations
}

// isPolicyOfAbsentEnvironment は、s が、属する Environment が存在しないことを理由に比較不能な方式の計画かを返す。
func isPolicyOfAbsentEnvironment(s SettingPlan) bool {
	return s.Result == NotComparable && s.Observed.UnavailableReason() == github.UnavailableEnvironmentAbsent
}
