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
	plans := []SettingPlan{{Key: setting.EnvironmentResource, Environment: e.Name, ObservedEnvironment: observed, Result: result}}
	if e.DeploymentBranchPolicy != nil {
		policy := newSettingPlan(setting.EnvironmentDeploymentBranchPolicy, observed.DeploymentBranchPolicy(), setting.DeploymentBranchPolicyValue(*e.DeploymentBranchPolicy))
		policy.Environment = e.Name
		plans = append(plans, policy)
	}
	return plans
}

// environmentOperationsOf は、Environment の変更予定 s を実現する更新操作を、実行順に返す。
// 存在しない Environment のデプロイ元制限の変更予定は、同じ名前の作成の更新操作にまとめ、単独の更新操作にしないため偽を返す。
// 更新操作の並びと基準はアーキテクチャ設計の非原子的な更新と中間状態に従う。
// 方式が selected でない期待値は、個別パターンを持っていても POST を並べない。宣言していない要素を送らないため。
// 存在しない Environment の方式を基準に載せないのは、存在しないことの基準が成立すれば同じ理由の観測不能に必ずなり、確かめる条件が重なるため。
// 対にならない形（作成のない存在しないことを理由にした方式、存在が一致でない方式の不一致、作成と方式の不一致の組）と適用可能な比較不能の存在は、
// environmentPlans と judgeApplicability が作らないため、不変条件の破れとして panic する。
func (r RepositoryPlan) environmentOperationsOf(s SettingPlan) ([]UpdateOperation, bool) {
	if s.Key == setting.EnvironmentResource {
		if !s.IsCreation() {
			panic(fmt.Sprintf("changeplan: %s is a planned change other than a creation", s.Unit().String()))
		}
		var desired setting.Value
		if policy, ok := r.settingPlanOf(setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: s.Environment}); ok {
			if !isPolicyOfAbsentEnvironment(policy) {
				panic(fmt.Sprintf("changeplan: creation of %s is paired with %s that is not of an absent environment", s.Unit().String(), policy.Unit().String()))
			}
			desired = policy.Desired
		}
		operations := []UpdateOperation{{Unit: s.Unit(), Desired: desired, RelevantSettings: []RelevantSetting{{Unit: s.Unit(), EnvironmentAtApproval: s.ObservedEnvironment}}}}
		if desired.IsZero() || desired.DeploymentBranchPolicy().Mode != setting.DeploymentBranchPolicySelected {
			return operations, true
		}
		// 作成の Unit の綴りで作り、Units が作成から導く方式の単位と重なるようにする
		policyUnit := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: s.Environment}
		created := RelevantSetting{Unit: s.Unit(), CreatedByPrecedingOperation: true}
		return append(operations, deploymentPatternOperations(policyUnit, created, desired, emptySelected(), nil, desired.DeploymentBranchPolicy().Patterns)...), true
	}
	existence, ok := r.settingPlanOf(setting.Unit{Key: setting.EnvironmentResource, Environment: s.Environment})
	switch {
	case isPolicyOfAbsentEnvironment(s) && ok && existence.IsCreation():
		return nil, false
	case s.Result == Mismatch && ok && existence.Result == Match:
		environment := RelevantSetting{Unit: existence.Unit(), EnvironmentAtApproval: existence.ObservedEnvironment}
		observed, _ := s.Observed.Value()
		before, after := observed.DeploymentBranchPolicy(), s.Desired.DeploymentBranchPolicy()
		if before.Mode != setting.DeploymentBranchPolicySelected || after.Mode != setting.DeploymentBranchPolicySelected {
			operations := []UpdateOperation{{Unit: s.Unit(), Desired: s.Desired, RelevantSettings: []RelevantSetting{environment, {Unit: s.Unit(), ValueAtApproval: s.Observed}}}}
			if after.Mode != setting.DeploymentBranchPolicySelected {
				return operations, true
			}
			return append(operations, deploymentPatternOperations(s.Unit(), environment, s.Desired, emptySelected(), nil, after.Patterns)...), true
		}
		return deploymentPatternOperations(s.Unit(), environment, s.Desired, before, before.PatternsNotIn(after), after.PatternsNotIn(before)), true
	}
	panic(fmt.Sprintf("changeplan: %s has no environment plan to pair with", s.Unit().String()))
}

// emptySelected は、方式の PUT か作成のあとに予定する、方式 selected で個別パターンが空の値を返す。
func emptySelected() setting.DeploymentBranchPolicy {
	return setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{}}
}

// deploymentPatternOperations は、個別パターンを外す DELETE、足す POST の順に更新操作を並べる。
// from は最初の更新操作の基準のデプロイ元制限で、各更新操作の基準は、直前までの更新操作を反映した値にする。
// desired は宣言したデプロイ元制限の値の全体で、どの更新操作の期待値にもなる。
func deploymentPatternOperations(unit setting.Unit, environment RelevantSetting, desired setting.Value, from setting.DeploymentBranchPolicy, remove, add []setting.DeploymentPattern) []UpdateOperation {
	patterns := slices.Clone(from.Patterns)
	var operations []UpdateOperation
	next := func(direction DeploymentPatternDirection, pattern setting.DeploymentPattern) {
		base := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: patterns})
		operations = append(operations, UpdateOperation{
			Unit: unit, Desired: desired, DeploymentPatternChange: DeploymentPatternChange{Direction: direction, Pattern: pattern},
			RelevantSettings: []RelevantSetting{environment, {Unit: unit, ValueAtApproval: github.Observed(base)}},
		})
	}
	for _, pattern := range remove {
		next(RemoveDeploymentPattern, pattern)
		patterns = slices.DeleteFunc(patterns, func(p setting.DeploymentPattern) bool { return p == pattern })
	}
	for _, pattern := range add {
		next(AddDeploymentPattern, pattern)
		patterns = append(patterns, pattern)
	}
	return operations
}

// isPolicyOfAbsentEnvironment は、s が、属する Environment が存在しないことを理由に比較不能な方式の計画かを返す。
func isPolicyOfAbsentEnvironment(s SettingPlan) bool {
	return s.Result == NotComparable && s.Observed.UnavailableReason() == github.UnavailableEnvironmentAbsent
}
