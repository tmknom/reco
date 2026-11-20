package changeplan

import (
	"fmt"

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

// environmentOperationOf は、Environment の変更予定 s を実現する更新操作を返す。
// 存在しない Environment のデプロイ元制限の変更予定は、同じ名前の作成の更新操作にまとめ、単独の更新操作にしないため偽を返す。
// 基準はアーキテクチャ設計の非原子的な更新と中間状態に従う。
// 存在しない Environment の方式を基準に載せないのは、存在しないことの基準が成立すれば同じ理由の観測不能に必ずなり、確かめる条件が重なるため。
// 対にならない形（作成のない存在しないことを理由にした方式、存在が一致でない方式の不一致、作成と方式の不一致の組）と適用可能な比較不能の存在は、
// environmentPlans と judgeApplicability が作らないため、不変条件の破れとして panic する。
func (r RepositoryPlan) environmentOperationOf(s SettingPlan) (UpdateOperation, bool) {
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
		return UpdateOperation{Unit: s.Unit(), Desired: desired, RelevantSettings: []RelevantSetting{{Unit: s.Unit(), EnvironmentAtApproval: s.ObservedEnvironment}}}, true
	}
	existence, ok := r.settingPlanOf(setting.Unit{Key: setting.EnvironmentResource, Environment: s.Environment})
	switch {
	case isPolicyOfAbsentEnvironment(s) && ok && existence.IsCreation():
		return UpdateOperation{}, false
	case s.Result == Mismatch && ok && existence.Result == Match:
		return UpdateOperation{Unit: s.Unit(), Desired: s.Desired, RelevantSettings: []RelevantSetting{
			{Unit: existence.Unit(), EnvironmentAtApproval: existence.ObservedEnvironment},
			{Unit: s.Unit(), ValueAtApproval: s.Observed},
		}}, true
	}
	panic(fmt.Sprintf("changeplan: %s has no environment plan to pair with", s.Unit().String()))
}

// isPolicyOfAbsentEnvironment は、s が、属する Environment が存在しないことを理由に比較不能な方式の計画かを返す。
func isPolicyOfAbsentEnvironment(s SettingPlan) bool {
	return s.Result == NotComparable && s.Observed.UnavailableReason() == github.UnavailableEnvironmentAbsent
}
