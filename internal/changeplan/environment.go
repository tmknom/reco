package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/configuration"
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
	if s.Unit.Key == setting.EnvironmentRequiredReviewers {
		return requiredReviewersChangeOf(s, existence, ok)
	}
	switch {
	case isOfAbsentEnvironment(s) && ok && existence.IsEnvironmentCreation():
		return LogicalChange{}, false
	case s.Result == Mismatch && ok && existence.Result == Match:
		environment := existence.existenceBasis()
		return LogicalChange{DesiredState: []configuration.ManagedSetting{{Unit: s.Unit, Desired: s.Desired}}, Operations: deploymentBranchPolicyOperations(s, environment)}, true
	}
	panic(fmt.Sprintf("changeplan: %s has no environment plan to pair with", s.Unit.String()))
}

// creationOf は、Environment の作成の変更予定 s と、同じ名前のデプロイ元制限の変更予定と、同じ名前の Environment の担当者がいないことの宣言と、
// 同じ名前の Environment の中で存在しないことを宣言した変数の変更予定を、一つにまとめた論理的な設定変更を返す。
// 期待状態は、Environment、方式、担当者の集合、存在しないことを宣言した変数 (計画の並び) の順である。
// 存在しない Environment の方式、担当者の集合、変数を基準に載せないのは、存在しないことの基準が成立すれば同じ理由の観測不能に必ずなり、確かめる条件が重なるため。
// 作成でない Environment の変更予定と、存在しないことを理由にした比較不能でない方式との組は、reconcile と judgeApplicability が作らないため、
// 不変条件の破れとして panic する。
func (r RepositoryPlan) creationOf(s SettingPlan) LogicalChange {
	if !s.IsEnvironmentCreation() {
		panic(fmt.Sprintf("changeplan: %s is a planned change other than a creation", s.Unit.String()))
	}
	change := LogicalChange{DesiredState: []configuration.ManagedSetting{{Unit: s.Unit, Desired: s.Desired}}}
	existence := []PreconditionBasis{s.existenceBasis()}
	// 方式の単位は作成の綴りで作り、期待状態と更新操作が作成と同じ綴りの名前を示すようにする
	policyUnit := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: s.Unit.Environment}
	policy, ok := r.settingPlanOf(policyUnit)
	if ok && !isOfAbsentEnvironment(policy) {
		panic(fmt.Sprintf("changeplan: creation of %s is paired with %s that is not of an absent environment", s.Unit.String(), policy.Unit.String()))
	}
	if ok {
		change.DesiredState = append(change.DesiredState, configuration.ManagedSetting{Unit: policyUnit, Desired: policy.Desired})
	}
	reviewersUnit := setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: s.Unit.Environment}
	if reviewers, declared := r.settingPlanOf(reviewersUnit); declared && isNoReviewersInAbsentEnvironment(reviewers) {
		change.DesiredState = append(change.DesiredState, configuration.ManagedSetting{Unit: reviewersUnit, Desired: reviewers.Desired})
	}
	for _, variable := range r.Settings {
		if isDeclaredAbsentInAbsentEnvironment(variable) && variable.Unit.Environment.EqualFold(s.Unit.Environment) {
			change.DesiredState = append(change.DesiredState, configuration.ManagedSetting{Unit: variable.Unit, Desired: variable.Desired})
		}
	}
	if !ok {
		change.Operations = []UpdateOperation{{Kind: EnvironmentCreation, Unit: s.Unit, PreconditionBases: existence}}
		return change
	}
	desired := policy.Desired.DeploymentBranchPolicy()
	change.Operations = []UpdateOperation{{Kind: EnvironmentCreationWithMode, Unit: s.Unit, Mode: desired.Mode, PreconditionBases: existence}}
	if desired.Mode != setting.DeploymentBranchPolicySelected {
		return change
	}
	created := PreconditionBasis{Unit: s.Unit, AnticipatedValue: AnticipateValue(setting.Present()), AnticipatedEnvironmentIdentity: CreatedByPrecedingOperation()}
	change.Operations = append(change.Operations, deploymentPatternOperations(policyUnit, created, emptySelected(), nil, desired.Patterns)...)
	return change
}

// requiredReviewersChangeOf は、担当者の集合の変更予定 s を、同じ名前の Environment の存在の計画 existence と組んだ論理的な設定変更にして返す。
// 作成との分け方と基準は Environments の設計の論理的な設定変更と更新操作に従う。
// 作成する Environment の空集合の宣言は、更新操作を持たずに作成の期待状態に加わるため (creationOf)、偽を返す。
// 対にならない形は reconcile と judgeApplicability が作らないため、不変条件の破れとして panic する。
func requiredReviewersChangeOf(s, existence SettingPlan, ok bool) (LogicalChange, bool) {
	desiredState := []configuration.ManagedSetting{{Unit: s.Unit, Desired: s.Desired}}
	switch {
	case ok && existence.Result == Match && s.Result == Mismatch:
		bases := []PreconditionBasis{existence.existenceBasis(), {Unit: s.Unit, AnticipatedValue: AnticipateUnchanged(s.Observed)}}
		return LogicalChange{DesiredState: desiredState, Operations: []UpdateOperation{{Kind: RequiredReviewersUpdate, Unit: s.Unit, Value: s.Desired, PreconditionBases: bases}}}, true
	case ok && existence.IsEnvironmentCreation() && isNoReviewersInAbsentEnvironment(s):
		return LogicalChange{}, false
	case ok && existence.IsEnvironmentCreation() && isOfAbsentEnvironment(s):
		bases := []PreconditionBasis{{Unit: s.Unit, AnticipatedValue: AnticipateValue(setting.ReviewerSetValue(setting.ReviewerSetOf(nil)))}}
		return LogicalChange{DesiredState: desiredState, Operations: []UpdateOperation{{Kind: RequiredReviewersUpdate, Unit: s.Unit, Value: s.Desired, PreconditionBases: bases}}}, true
	}
	panic(fmt.Sprintf("changeplan: %s has no environment plan to pair with", s.Unit.String()))
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
		operation := variableOperationOf(s, AnticipateUnchanged(s.Observed))
		operation.PreconditionBases = append([]PreconditionBasis{existence.existenceBasis()}, operation.PreconditionBases...)
		return operation, true
	case ok && existence.IsEnvironmentCreation() && isDeclaredAbsentInAbsentEnvironment(s):
		return UpdateOperation{}, false
	case ok && existence.IsEnvironmentCreation() && isOfAbsentEnvironment(s):
		return variableOperationOf(s, AnticipateValue(setting.Absent())), true
	}
	panic(fmt.Sprintf("changeplan: %s has no environment plan to pair with", s.Unit.String()))
}

// deploymentBranchPolicyOperations は、存在する Environment のデプロイ元制限の不一致 s を実現する更新操作を、実行順に返す。
// environment は、その Environment の存在と ID の基準である。
// 方式が selected でない期待値は、個別パターンを持っていても加える更新操作を並べない。宣言していない要素を送らないため。
func deploymentBranchPolicyOperations(s SettingPlan, environment PreconditionBasis) []UpdateOperation {
	observed, _ := s.Observed.Value()
	before, after := observed.DeploymentBranchPolicy(), s.Desired.DeploymentBranchPolicy()
	if before.Mode == setting.DeploymentBranchPolicySelected && after.Mode == setting.DeploymentBranchPolicySelected {
		return deploymentPatternOperations(s.Unit, environment, before, before.PatternsNotIn(after), after.PatternsNotIn(before))
	}
	operations := []UpdateOperation{{
		Kind: DeploymentBranchPolicyModeUpdate, Unit: s.Unit, Mode: after.Mode,
		PreconditionBases: []PreconditionBasis{environment, {Unit: s.Unit, AnticipatedValue: AnticipateUnchanged(s.Observed)}},
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
func deploymentPatternOperations(unit setting.Unit, environment PreconditionBasis, from setting.DeploymentBranchPolicy, remove, add []setting.DeploymentPattern) []UpdateOperation {
	patterns := slices.Clone(from.Patterns)
	var operations []UpdateOperation
	next := func(kind UpdateOperationKind, pattern setting.DeploymentPattern) {
		base := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: patterns})
		operations = append(operations, UpdateOperation{
			Kind: kind, Unit: unit, Pattern: pattern,
			PreconditionBases: []PreconditionBasis{environment, {Unit: unit, AnticipatedValue: AnticipateValue(base)}},
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
func (s SettingPlan) existenceBasis() PreconditionBasis {
	return PreconditionBasis{Unit: s.Unit, AnticipatedValue: AnticipateUnchanged(s.Observed), AnticipatedEnvironmentIdentity: s.EnvironmentIdentity}
}

// isOfAbsentEnvironment は、s が、属する Environment が存在しないことを理由に比較不能な、方式か変数の計画かを返す。
func isOfAbsentEnvironment(s SettingPlan) bool {
	return s.Result == NotComparable && s.Observed.UnavailableReason() == github.UnavailableEnvironmentAbsent
}

// isNoReviewersInAbsentEnvironment は、s が、属する Environment が存在しないことを理由に比較不能な、担当者がいないことを宣言した担当者の集合の計画かを返す。
// 作られたばかりの Environment は担当者の保護ルールを持たないことを確かめているため (Environments の設計の論理的な設定変更と更新操作)、作成だけで期待値が成り立つ。
func isNoReviewersInAbsentEnvironment(s SettingPlan) bool {
	return s.Unit.Key == setting.EnvironmentRequiredReviewers && isOfAbsentEnvironment(s) && s.Desired.Equal(setting.ReviewerSetValue(setting.ReviewerSetOf(nil)))
}

// isDeclaredAbsentInAbsentEnvironment は、s が、属する Environment が存在しないことを理由に比較不能な、存在しないことを宣言した変数の計画かを返す。
func isDeclaredAbsentInAbsentEnvironment(s SettingPlan) bool {
	return s.Unit.Key == setting.EnvironmentVariable && isOfAbsentEnvironment(s) && s.Desired.Equal(setting.Absent())
}
