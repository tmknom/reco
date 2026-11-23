package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// environmentChangeOf は、Environment の変更予定 s を、それを含む論理的な設定変更にして返す。
// 存在しない Environment のデプロイ元制限の変更予定は、同じ名前の作成の論理的な設定変更にまとめ、単独の論理的な設定変更にしないため偽を返す。
// 更新操作の並びと基準は Environments の設計の論理的な設定変更と更新操作に従う。
// 対にならない形（作成のない存在しないことを理由にした方式、存在が一致でない方式の不一致）と適用可能な比較不能の存在は、
// reconcile と judgeApplicability が作らないため、不変条件の破れとして panic する。
func (r RepositoryPlan) environmentChangeOf(s SettingPlan) (LogicalChange, bool) {
	if s.Unit.Key == setting.EnvironmentResource {
		return r.creationOf(s), true
	}
	existence, ok := r.settingPlanOf(setting.Unit{Key: setting.EnvironmentResource, Environment: s.Unit.Environment})
	switch {
	case isOfAbsentEnvironment(s) && ok && existence.IsEnvironmentCreation():
		return LogicalChange{}, false
	case s.Result == Mismatch && ok && existence.Result == Match:
		environment := existence.existenceBasis()
		return LogicalChange{DesiredState: []DesiredSetting{{Unit: s.Unit, Value: s.Desired}}, Operations: deploymentBranchPolicyOperations(s, environment)}, true
	}
	panic(fmt.Sprintf("changeplan: %s has no environment plan to pair with", s.Unit.String()))
}

// creationOf は、Environment の作成の変更予定 s と、同じ名前のデプロイ元制限の変更予定と、同じ名前の Environment の中で存在しないことを宣言した変数の変更予定を、
// 一つにまとめた論理的な設定変更を返す。期待状態は、Environment、方式、存在しないことを宣言した変数 (計画の並び) の順である。
// 存在しない Environment の方式と変数を基準に載せないのは、存在しないことの基準が成立すれば同じ理由の観測不能に必ずなり、確かめる条件が重なるため。
// 作成でない Environment の変更予定と、存在しないことを理由にした比較不能でない方式との組は、reconcile と judgeApplicability が作らないため、
// 不変条件の破れとして panic する。
func (r RepositoryPlan) creationOf(s SettingPlan) LogicalChange {
	if !s.IsEnvironmentCreation() {
		panic(fmt.Sprintf("changeplan: %s is a planned change other than a creation", s.Unit.String()))
	}
	change := LogicalChange{DesiredState: []DesiredSetting{{Unit: s.Unit, Value: s.Desired}}}
	existence := []RelevantSetting{s.existenceBasis()}
	// 方式の単位は作成の綴りで作り、期待状態と更新操作が作成と同じ綴りの名前を示すようにする
	policyUnit := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: s.Unit.Environment}
	policy, ok := r.settingPlanOf(policyUnit)
	if ok && !isOfAbsentEnvironment(policy) {
		panic(fmt.Sprintf("changeplan: creation of %s is paired with %s that is not of an absent environment", s.Unit.String(), policy.Unit.String()))
	}
	if ok {
		change.DesiredState = append(change.DesiredState, DesiredSetting{Unit: policyUnit, Value: policy.Desired})
	}
	for _, variable := range r.Settings {
		if isDeclaredAbsentInAbsentEnvironment(variable) && variable.Unit.Environment.EqualFold(s.Unit.Environment) {
			change.DesiredState = append(change.DesiredState, DesiredSetting{Unit: variable.Unit, Value: variable.Desired})
		}
	}
	if !ok {
		change.Operations = []UpdateOperation{{Kind: EnvironmentCreation, Unit: s.Unit, RelevantSettings: existence}}
		return change
	}
	desired := policy.Desired.DeploymentBranchPolicy()
	change.Operations = []UpdateOperation{{Kind: EnvironmentCreationWithMode, Unit: s.Unit, Mode: desired.Mode, RelevantSettings: existence}}
	if desired.Mode != setting.DeploymentBranchPolicySelected {
		return change
	}
	created := RelevantSetting{Unit: s.Unit, AnticipatedValue: github.Observed(setting.Present()), AnticipatedIdentity: CreatedByPrecedingOperation()}
	change.Operations = append(change.Operations, deploymentPatternOperations(policyUnit, created, emptySelected(), nil, desired.Patterns)...)
	return change
}

// environmentVariableOperationOf は、Environment Variable の変更予定 s の更新操作を、同じ名前の Environment の存在の計画と組んで返す。
// 基準は Environments の設計の論理的な設定変更と更新操作に従い、計画時に存在した Environment では Environment (ID を含む)、変数の順、
// 同じ変更計画が作成する Environment では変数が存在しないことだけである。
// 作成する Environment の中で存在しないことを宣言した変数は、更新操作を持たずに作成の期待状態に加わるため (creationOf)、偽を返す。
// 対にならない形 (存在の計画のない変数、存在が一致なのに比較不能な変数、作成なのに不一致の変数) は、
// reconcile と judgeApplicability が作らないため、不変条件の破れとして panic する。
func (r RepositoryPlan) environmentVariableOperationOf(s SettingPlan) (UpdateOperation, bool) {
	existence, ok := r.settingPlanOf(setting.Unit{Key: setting.EnvironmentResource, Environment: s.Unit.Environment})
	switch {
	case ok && existence.Result == Match && s.Result == Mismatch:
		operation := variableOperationOf(s, s.Observed)
		operation.RelevantSettings = append([]RelevantSetting{existence.existenceBasis()}, operation.RelevantSettings...)
		return operation, true
	case ok && existence.IsEnvironmentCreation() && isDeclaredAbsentInAbsentEnvironment(s):
		return UpdateOperation{}, false
	case ok && existence.IsEnvironmentCreation() && isOfAbsentEnvironment(s):
		return variableOperationOf(s, github.Observed(setting.Absent())), true
	}
	panic(fmt.Sprintf("changeplan: %s has no environment plan to pair with", s.Unit.String()))
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
		RelevantSettings: []RelevantSetting{environment, {Unit: s.Unit, AnticipatedValue: s.Observed}},
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
			RelevantSettings: []RelevantSetting{environment, {Unit: unit, AnticipatedValue: github.Observed(base)}},
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

// existenceBasis は、Environment の存在の計画 s を、計画時の存在の観測結果と ID を承認時に予定した値にした基準にして返す。
func (s SettingPlan) existenceBasis() RelevantSetting {
	return RelevantSetting{Unit: s.Unit, AnticipatedValue: s.Observed, AnticipatedIdentity: s.EnvironmentIdentity}
}

// isOfAbsentEnvironment は、s が、属する Environment が存在しないことを理由に比較不能な、方式か変数の計画かを返す。
func isOfAbsentEnvironment(s SettingPlan) bool {
	return s.Result == NotComparable && s.Observed.UnavailableReason() == github.UnavailableEnvironmentAbsent
}

// isDeclaredAbsentInAbsentEnvironment は、s が、属する Environment が存在しないことを理由に比較不能な、存在しないことを宣言した変数の計画かを返す。
func isDeclaredAbsentInAbsentEnvironment(s SettingPlan) bool {
	return s.Unit.Key == setting.EnvironmentVariable && isOfAbsentEnvironment(s) && s.Desired.Equal(setting.Absent())
}
