package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// protectionRuleSetting は、保護ルールの設定 (Environments の設計の設定と管理の単位) の一つで、その設定だけを送る更新操作の種類と、作成直後の値を持つ。
// 種類ごとの属性を一か所に置き、作成への吸収、更新操作、変更前提の基準が同じ定義から決まるようにする。
type protectionRuleSetting struct {
	key     setting.Key
	kind    UpdateOperationKind
	created setting.Value
}

// environmentChangeOf は、Environment の変更予定 s を、それを含む論理的な設定変更にして返す。
// Environment そのものの変更予定は、削除なら削除の、そうでなければ作成の論理的な設定変更にする。存在しないことを宣言した Environment は中の設定の計画を持たない。
// 存在しない Environment のデプロイ元制限の変更予定は、同じ名前の作成の論理的な設定変更にまとめ、単独の論理的な設定変更にしないため偽を返す。
// 更新操作の並びと基準は Environments の設計の論理的な設定変更と更新操作に従う。
// 対にならない形（作成のない存在しないことを理由にした方式、存在が一致でない方式の不一致）と適用可能な比較不能の存在は、
// reconcile と judgeApplicability が作らないため、不変条件の破れとして panic する。
func (r RepositoryPlan) environmentChangeOf(s SettingPlan) (LogicalChange, bool) {
	if s.IsEnvironmentDeletion() {
		return environmentDeletionOf(s), true
	}
	if s.Unit.Key == setting.EnvironmentResource {
		return r.creationOf(s), true
	}
	existence, ok := r.settingPlanOf(setting.Unit{Key: setting.EnvironmentResource, Environment: s.Unit.Environment})
	if p, isProtectionRule := protectionRuleSettingOf(s.Unit.Key); isProtectionRule {
		return r.protectionRuleSettingChangeOf(s, p, existence, ok)
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

// environmentDeletionOf は、Environment の削除の変更予定 s を、存在しないことを期待状態にし、値を送らない削除の更新操作一つからなる論理的な設定変更にして返す。
// 基準は一つで、計画時の存在と ID に加えて、承認の前に示した削除で失われるものを載せる。示した内容と比べる内容を同じ値から取るため。
// 削除で失われるもののない削除は、失われる値を示せずに適用不能のはずで (judgeApplicability)、ここに届けば不変条件の破れとして panic する。
func environmentDeletionOf(s SettingPlan) LogicalChange {
	if s.EnvironmentLoss == nil {
		panic(fmt.Sprintf("changeplan: deletion of %s has no loss shown before approval", s.Unit.String()))
	}
	basis := s.existenceBasis()
	basis.AnticipatedEnvironmentLoss = s.EnvironmentLoss
	return LogicalChange{
		DesiredState: []configuration.ManagedSetting{{Unit: s.Unit, Desired: s.Desired}},
		Operations:   []UpdateOperation{{Kind: EnvironmentDeletion, Unit: s.Unit, PreconditionBases: []PreconditionBasis{basis}}},
	}
}

// creationOf は、Environment の作成の変更予定 s と、同じ名前のデプロイ元制限の変更予定と、同じ名前の Environment の作成直後の値と同じ保護ルールの設定の宣言と、
// 同じ名前の Environment の中で存在しないことを宣言した変数の変更予定を、一つにまとめた論理的な設定変更を返す。
// 期待状態は、Environment、方式、担当者の集合、自己承認の禁止、デプロイの待機時間、存在しないことを宣言した変数 (計画の並び) の順である。
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
	for _, p := range protectionRuleSettings() {
		unit := setting.Unit{Key: p.key, Environment: s.Unit.Environment}
		if plan, declared := r.settingPlanOf(unit); declared && isOfAbsentEnvironment(plan) && plan.Desired.Equal(p.created) {
			change.DesiredState = append(change.DesiredState, configuration.ManagedSetting{Unit: unit, Desired: plan.Desired})
		}
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

// protectionRuleSettingChangeOf は、保護ルールの設定 p の変更予定 s を、同じ名前の Environment の存在の計画 existence と組み、
// その設定だけを送る更新操作からなる論理的な設定変更にして返す。作成との分け方と基準は Environments の設計の論理的な設定変更と更新操作に従う。
// 作成直後の値と同じ宣言は、更新操作を持たずに作成の期待状態に加わるため (creationOf)、偽を返す。
// 担当者をすべて外す変更と重なる自己承認の禁止は、担当者の論理的な設定変更に加わるため (withPreventSelfReviewOfRemoval)、偽を返す。
// 対にならない形は reconcile と judgeApplicability が作らないため、不変条件の破れとして panic する。
func (r RepositoryPlan) protectionRuleSettingChangeOf(s SettingPlan, p protectionRuleSetting, existence SettingPlan, ok bool) (LogicalChange, bool) {
	switch {
	case ok && existence.Result == Match && s.Result == Mismatch:
		if s.Unit.Key == setting.EnvironmentPreventSelfReview && r.removesEveryReviewer(s) {
			return LogicalChange{}, false
		}
		change := protectionRuleSettingChange(p, s, existence.existenceBasis(), PreconditionBasis{Unit: s.Unit, AnticipatedValue: AnticipateUnchanged(s.Observed)})
		if s.Unit.Key == setting.EnvironmentReviewerSet && s.Desired.ReviewerSet().HasNoReviewers() {
			return r.withPreventSelfReviewOfRemoval(change, s), true
		}
		return change, true
	case ok && existence.IsEnvironmentCreation() && isOfAbsentEnvironment(s) && s.Desired.Equal(p.created):
		return LogicalChange{}, false
	case ok && existence.IsEnvironmentCreation() && isOfAbsentEnvironment(s):
		// 作成直後の値を観測できたことで、作成した Environment の存在もあわせて確かめる
		return protectionRuleSettingChange(p, s, PreconditionBasis{Unit: s.Unit, AnticipatedValue: AnticipateValue(p.created)}), true
	}
	panic(fmt.Sprintf("changeplan: %s has no environment plan to pair with", s.Unit.String()))
}

// protectionRuleSettingChange は、保護ルールの設定 p の変更予定 s の期待値だけを送る更新操作一つからなる論理的な設定変更を、基準 bases で返す。
func protectionRuleSettingChange(p protectionRuleSetting, s SettingPlan, bases ...PreconditionBasis) LogicalChange {
	return LogicalChange{
		DesiredState: []configuration.ManagedSetting{{Unit: s.Unit, Desired: s.Desired}},
		Operations:   []UpdateOperation{{Kind: p.kind, Unit: s.Unit, Value: s.Desired, PreconditionBases: bases}},
	}
}

// removesEveryReviewer は、自己承認の禁止の不一致 s が、同じ Environment の担当者をすべて外す変更と重なるかを返す。
// 重なるとき、担当者をすべて外す更新が自己承認の禁止を偽にし、期待値 (宣言できるのは偽だけ) に届く。
// 担当者がいないことを宣言して一致したまま自己承認の禁止だけが違う形は、観測から生じず、送れば GitHub が拒むため、不変条件の破れとして panic する。
func (r RepositoryPlan) removesEveryReviewer(s SettingPlan) bool {
	reviewers, declared := r.settingPlanOf(setting.Unit{Key: setting.EnvironmentReviewerSet, Environment: s.Unit.Environment})
	if !declared || !reviewers.Desired.ReviewerSet().HasNoReviewers() {
		return false
	}
	if reviewers.Result != Mismatch {
		panic(fmt.Sprintf("changeplan: %s differs while %s declares no reviewers and matches", s.Unit.String(), reviewers.Unit.String()))
	}
	return true
}

// withPreventSelfReviewOfRemoval は、担当者をすべて外す論理的な設定変更 change に、GitHub が同時に偽にする同じ Environment の自己承認の禁止を、
// 計画時の値のまま変わっていないことを確かめる基準として加えて返す。宣言の有無によらず加え、承認のあとに外部主体が変えた値を示さないまま失わせないようにする。
func (r RepositoryPlan) withPreventSelfReviewOfRemoval(change LogicalChange, s SettingPlan) LogicalChange {
	operation := &change.Operations[0]
	operation.PreconditionBases = append(operation.PreconditionBases, sideEffectBases(s)...)
	change.SideEffectSettings = sideEffectSettingsOf(s)
	declared, ok := r.settingPlanOf(setting.Unit{Key: setting.EnvironmentPreventSelfReview, Environment: s.Unit.Environment})
	if !ok {
		return change
	}
	operation.PreconditionBases = append(operation.PreconditionBases, PreconditionBasis{Unit: declared.Unit, AnticipatedValue: AnticipateUnchanged(declared.Observed)})
	if declared.Result == Mismatch {
		change.DesiredState = append(change.DesiredState, configuration.ManagedSetting{Unit: declared.Unit, Desired: declared.Desired})
	}
	return change
}

// protectionRuleSettings は、保護ルールの設定を計画の並び (YAML 設定仕様の表の並び) で返す。
// 作成直後の値は、本文のない作成も方式だけの作成も required_reviewers と wait_timer の保護ルールを持たない Environment を作ると確かめたことから決まる
// (Environments の設計の論理的な設定変更と更新操作)。
// 呼び出し側が要素を書き換えても共有されないよう、呼び出すたびに新しいスライスを返す。
func protectionRuleSettings() []protectionRuleSetting {
	return []protectionRuleSetting{
		{key: setting.EnvironmentReviewerSet, kind: ReviewerSetUpdate, created: setting.ReviewerSetValue(setting.ReviewerSetOf(nil))},
		{key: setting.EnvironmentPreventSelfReview, kind: PreventSelfReviewUpdate, created: setting.BoolValue(false)},
		{key: setting.EnvironmentWaitTimer, kind: WaitTimerUpdate, created: setting.WaitTimerValue(0)},
	}
}

// protectionRuleSettingOf は、key が保護ルールの設定のとき、その設定と真を返す。
func protectionRuleSettingOf(key setting.Key) (protectionRuleSetting, bool) {
	for _, p := range protectionRuleSettings() {
		if p.key == key {
			return p, true
		}
	}
	return protectionRuleSetting{}, false
}

// createdValueOf は、作られたばかりの Environment の、保護ルールの設定 key の値 (作成直後の値) を返す。
// 保護ルールの設定でない key は不変条件の破れとして panic する。
func createdValueOf(key setting.Key) setting.Value {
	p, ok := protectionRuleSettingOf(key)
	if !ok {
		panic(fmt.Sprintf("changeplan: %s has no value of a created environment", key))
	}
	return p.created
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

// isDeclaredAbsentInAbsentEnvironment は、s が、属する Environment が存在しないことを理由に比較不能な、存在しないことを宣言した変数の計画かを返す。
func isDeclaredAbsentInAbsentEnvironment(s SettingPlan) bool {
	return s.Unit.Key == setting.EnvironmentVariable && isOfAbsentEnvironment(s) && s.Desired.Equal(setting.Absent())
}
