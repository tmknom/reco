package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// LogicalChange は、承認の対象になる論理的な設定変更。
// 実行は承認した変更内容だけを使い、設定宣言を読み直さずに変更前提と期待状態を確かめる。
type LogicalChange struct {
	// DesiredState は期待状態。まとめた管理設定の期待値を、管理設定ごとに一つずつ、計画の並びで持つ。一つ以上を持つ。
	DesiredState []DesiredSetting
	// Operations は構成する更新操作を実行順に並べたもの。一つ以上を持つ。
	Operations []UpdateOperation
	// SideEffectSettings は、変更に伴う副作用として提示した設定のうち reco が観測するもの。
	// 承認した変化のため、同じ更新エンドポイントの設定でも変更対象外の設定に数えない。
	SideEffectSettings []setting.Unit
}

// DesiredSetting は、期待状態を構成する一つの管理設定の期待値。
type DesiredSetting struct {
	Unit  setting.Unit
	Value setting.Value
}

// UpdateOperation は、論理的な設定変更を構成する更新操作について、承認の対象になる内容。
// 送る内容のフィールドは Kind ごとに決まり、その種類が送らないフィールドはゼロ値にする。
type UpdateOperation struct {
	Kind UpdateOperationKind
	// Unit は、この更新操作が更新要求を送る先の管理の単位。MergeMethodsUpdate では、送る管理設定の先頭である。
	Unit setting.Unit
	// Value は、SettingUpdate が送る設定の値と、Repository Variable と Environment Variable の作成と値の変更が送る変数の値と、Ruleset の作成と値の変更が送る Ruleset の値。
	Value setting.Value
	// Mode は、EnvironmentCreationWithMode と DeploymentBranchPolicyModeUpdate が送るデプロイ元制限の方式。
	Mode setting.DeploymentBranchPolicyMode
	// Pattern は、DeploymentPatternAddition と DeploymentPatternRemoval が加えるか取り除く個別パターン。
	Pattern setting.DeploymentPattern
	// Settings は、MergeMethodsUpdate が一つの更新要求で送る管理設定とその値を、計画の並びで持つ。
	Settings []DesiredSetting
	// RelevantSettings は、この更新操作の変更前提の基準。変わっていたことを報告するときに優先する順に並べる。
	RelevantSettings []RelevantSetting
}

// UpdateOperationKind は、更新操作の種類。送る更新要求の形と、関連リソースを作るかどうかで分ける。
// EnvironmentCreationWithMode と DeploymentBranchPolicyModeUpdate は同じ形の要求を送るが、作成かどうかで変更前提の基準、表示、失敗の示し方が変わるため分ける。
type UpdateOperationKind int

// ゼロ値をいずれかの種類と取り違えないよう 1 から始める。
const (
	// SettingUpdate は、リポジトリ設定の値をその更新エンドポイントへ送る。
	SettingUpdate UpdateOperationKind = iota + 1
	// MergeMethodsUpdate は、マージ方式の値を一つの更新要求でまとめて送る。
	MergeMethodsUpdate
	// EnvironmentCreation は、デプロイ元制限を送らずに Environment を作る。
	EnvironmentCreation
	// EnvironmentCreationWithMode は、デプロイ元制限の方式を送って Environment を作る。
	EnvironmentCreationWithMode
	// DeploymentBranchPolicyModeUpdate は、存在する Environment のデプロイ元制限の方式を送る。
	DeploymentBranchPolicyModeUpdate
	// DeploymentPatternAddition は、個別パターンを一つ加える。
	DeploymentPatternAddition
	// DeploymentPatternRemoval は、個別パターンを一つ取り除く。
	DeploymentPatternRemoval
	// RepositoryVariableCreation は、Repository Variable を名前と値で作る。
	RepositoryVariableCreation
	// RepositoryVariableValueUpdate は、存在する Repository Variable の値だけを変える。
	RepositoryVariableValueUpdate
	// RepositoryVariableDeletion は、存在する Repository Variable を消す。値を送らない。
	RepositoryVariableDeletion
	// EnvironmentVariableCreation は、Environment Variable を名前と値で作る。宛先が Environment の名前を含むため Repository Variable と分ける。
	EnvironmentVariableCreation
	// EnvironmentVariableValueUpdate は、存在する Environment Variable の値だけを変える。
	EnvironmentVariableValueUpdate
	// EnvironmentVariableDeletion は、存在する Environment Variable を消す。値を送らない。
	EnvironmentVariableDeletion
	// RulesetCreation は、Ruleset を名前と値で作る。
	RulesetCreation
	// RulesetValueUpdate は、存在する Ruleset の値を置き換える。
	RulesetValueUpdate
)

// RelevantSetting は、変更前提で比べる関係する現在値の設定と、その承認時に予定した値。
type RelevantSetting struct {
	Unit setting.Unit
	// AnticipatedValue は、承認時に予定した値。関連リソースそのものでは、存在すること (値を伴う) か存在しないことである。
	AnticipatedValue AnticipatedValue
	// AnticipatedEnvironmentIdentity は、Environment が存在すると予定したときに、存在に加えて確かめる同一性の基準。ほかではゼロ値。
	AnticipatedEnvironmentIdentity EnvironmentIdentity
	// AnticipatedRulesetIdentity は、Ruleset が存在すると予定したときに、存在と値に加えて確かめる同一性の基準。ほかではゼロ値。
	AnticipatedRulesetIdentity RulesetIdentity
}

// Units は、c が変える管理設定の管理の単位を、期待状態の並びで返す。
func (c LogicalChange) Units() []setting.Unit {
	units := make([]setting.Unit, len(c.DesiredState))
	for i, desired := range c.DesiredState {
		units[i] = desired.Unit
	}
	return units
}

// SettingsToChange は、ドメインモデルの「論理的な設定変更が変える設定」を返す。変更対象外の設定から除く範囲の一つである (ExcludedFromOutsideSettings)。
// Units の管理の単位に、その期待値が含む状態の設定を足したもの。
func (c LogicalChange) SettingsToChange() []setting.Unit {
	units := c.Units()
	if slices.Contains(units, setting.Unit{Key: setting.AllowedActions}) {
		units = append(units, setting.Unit{Key: setting.ActionsEnabled})
	}
	return units
}

// ExcludedFromOutsideSettings は、更新エンドポイントで reco が観測する設定のうち、変更対象外の設定から除くものを返す。
// 論理的な設定変更が変える設定と、副作用として提示した設定である (ドメインモデルの変更対象外の設定)。
func (c LogicalChange) ExcludedFromOutsideSettings() []setting.Unit {
	return append(c.SettingsToChange(), c.SideEffectSettings...)
}

// CreatesEnvironment は、この更新操作が Environment の作成かを返す。
func (o UpdateOperation) CreatesEnvironment() bool {
	return o.Kind == EnvironmentCreation || o.Kind == EnvironmentCreationWithMode
}

// Endpoint は、この更新操作が更新要求を送る更新エンドポイントを、設定の識別から引けるときだけ、それと真を返す。
// 関連リソースの更新操作は、更新エンドポイントが名前ごとに別で、設定の識別だけでは引けないため偽を返す。
// 更新要求を持たない設定は UpdateEndpointOf が panic する。
// 未定義の種類は宛先が決まらないため、不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (o UpdateOperation) Endpoint() (github.UpdateEndpoint, bool) {
	switch o.Kind {
	case SettingUpdate, MergeMethodsUpdate:
		return github.UpdateEndpointOf(o.Unit.Key), true
	case EnvironmentCreation, EnvironmentCreationWithMode, DeploymentBranchPolicyModeUpdate,
		DeploymentPatternAddition, DeploymentPatternRemoval,
		RepositoryVariableCreation, RepositoryVariableValueUpdate, RepositoryVariableDeletion,
		EnvironmentVariableCreation, EnvironmentVariableValueUpdate, EnvironmentVariableDeletion,
		RulesetCreation, RulesetValueUpdate:
		return 0, false
	}
	panic(fmt.Sprintf("changeplan: undefined kind %d of an update operation", int(o.Kind)))
}

// LogicalChanges は、選択したすべての対象リポジトリについて、論理的な設定変更を実行順で導く。
// 戻り値は Repositories と同じ長さ、同じ並びで、変更不要のリポジトリの計画の位置は空になる。
// INV-002 のため、先の対象リポジトリを更新したあとではなく、承認を求める前にすべての対象リポジトリを導き切る。
func (p Plan) LogicalChanges() ([][]LogicalChange, error) {
	all := make([][]LogicalChange, len(p.Repositories))
	for i, r := range p.Repositories {
		changes, err := r.LogicalChanges()
		if err != nil {
			return nil, err
		}
		all[i] = changes
	}
	return all, nil
}

// LogicalChanges は、変更予定から論理的な設定変更を、executionOrder の実行順で導く。
// 適用可能でない変更予定があれば、承認の対象にしないため error にする。
// 副作用を伴う変更予定の更新操作は、operationOf が副作用として提示した設定のうち reco が観測するものを必ず基準に載せる。載せずに受け付けると承認の前に示していない値を失わせ得るため。
// 遷移先が selected のポリシーの変更予定と、選択方式の Actions 許可条件の比較不能な変更予定は、ポリシーの位置で一つの論理的な設定変更にまとめ、
// ポリシー、許可条件の順に二つの更新操作で実現する。
// マージ方式の変更予定は、最初のマージ方式の位置で一つの論理的な設定変更にまとめる (mergeMethodsChange)。
// 対にならない形は、設定宣言の組み合わせの規則と観測の取得条件のどちらかが破れたときにしか生じないため、不変条件の破れとして panic する。
// Actions の有効状態と Actions 許可ポリシーの計画が、照合結果によらず同じリポジトリの計画に並ぶ形も、
// 設定宣言の組み合わせの規則が破れたときにしか生じないため panic する。
// 有効状態の変更予定とポリシーの組を実行すると、有効状態の更新操作が無効にする前のポリシーで Actions を有効にし、宣言していない状態を残し得るため。
// 導いた更新操作は、承認のあと先行する変更を送ってから止まらないよう、宛先を決められるかを checkUpdateEndpoint で確かめる。
func (r RepositoryPlan) LogicalChanges() ([]LogicalChange, error) {
	for _, s := range r.Settings {
		if s.IsPlannedChange() && !s.Applicability.IsApplicable() {
			return nil, fmt.Errorf("%s: %s cannot be applied", r.Target, s.Unit.String())
		}
	}
	_, hasEnabled := r.settingPlanOf(setting.Unit{Key: setting.ActionsEnabled})
	if _, hasPolicy := r.settingPlanOf(setting.Unit{Key: setting.AllowedActions}); hasEnabled && hasPolicy {
		panic(fmt.Sprintf("changeplan: plans of %s and %s are in the same repository plan", setting.ActionsEnabled, setting.AllowedActions))
	}
	criteria, paired := r.criteriaFollowingPolicy()
	var changes []LogicalChange
	mergeMethodsBundled := false
	for _, s := range r.executionOrder() {
		switch {
		case !s.IsPlannedChange():
			continue
		case slices.Contains(setting.MergeMethods(), s.Unit.Key):
			if !mergeMethodsBundled {
				changes = append(changes, mergeMethodsChange(r.Settings))
				mergeMethodsBundled = true
			}
			continue
		case s.Unit.Key == setting.SelectedActions && s.Result == NotComparable:
			if !paired {
				panic(fmt.Sprintf("changeplan: %s is not comparable without a policy change to selected", s.Unit.Key))
			}
			continue
		case s.Unit.Key.Area() == setting.AreaEnvironments:
			if change, ok := r.environmentChangeOf(s); ok {
				changes = append(changes, change)
			}
			continue
		case s.Unit.Key == setting.EnvironmentVariable:
			// Environment の作成とまとめず、変数ごとに一つの論理的な設定変更にする (Environments の設計の論理的な設定変更と更新操作)
			if operation, ok := r.environmentVariableOperationOf(s); ok {
				changes = append(changes, LogicalChange{DesiredState: []DesiredSetting{{Unit: s.Unit, Value: s.Desired}}, Operations: []UpdateOperation{operation}})
			}
			continue
		case s.Unit.Key == setting.RulesetResource:
			changes = append(changes, rulesetChangeOf(s))
			continue
		}
		operation := operationOf(s)
		switch {
		case s.Unit.Key == setting.SHAPinningRequired:
			operation = shaPinningRequiredOperationOf(s)
		case s.Unit.Key == setting.RepositoryVariable:
			operation = variableOperationOf(s, AnticipateUnchanged(s.Observed))
		case s.Unit.Key == setting.RepositoryPushProtection && s.Desired.Bool():
			// GitHub は Secret scanning が無効のあいだ、有効にする更新に成功を応答して値を変えないため、更新直前に有効と確かめる。
			// 計画時に無効だった場合は、先行する Secret scanning の変更が有効にすることが予定どおりの変化である
			operation.RelevantSettings = append(operation.RelevantSettings, RelevantSetting{
				Unit: setting.Unit{Key: setting.SecretScanningEnabled}, AnticipatedValue: AnticipateValue(setting.BoolValue(true)),
			})
		}
		change := LogicalChange{
			DesiredState: []DesiredSetting{{Unit: s.Unit, Value: s.Desired}}, Operations: []UpdateOperation{operation}, SideEffectSettings: sideEffectSettingsOf(s),
		}
		if targetsSelectedPolicy(s) {
			if !paired {
				panic(fmt.Sprintf("changeplan: %s to selected has no not-comparable %s to follow", s.Unit.Key, setting.SelectedActions))
			}
			change.DesiredState = append(change.DesiredState, DesiredSetting{Unit: criteria.Unit, Value: criteria.Desired})
			// 2つ目の承認時に予定した値は、1つ目による予定どおりの変化を反映したポリシーである
			change.Operations = append(change.Operations, UpdateOperation{
				Kind: SettingUpdate, Unit: criteria.Unit, Value: criteria.Desired,
				RelevantSettings: []RelevantSetting{{Unit: s.Unit, AnticipatedValue: AnticipateValue(s.Desired)}},
			})
		}
		changes = append(changes, change)
	}
	for _, change := range changes {
		for _, operation := range change.Operations {
			checkUpdateEndpoint(operation)
		}
	}
	return changes, nil
}

// executionOrder は、管理設定の計画を論理的な設定変更の実行順に並べて返す。基本は計画の並び (変更予定の表示順) である。
// 前提の設定と依存する設定の組ごとに、前提の設定の期待値が無効のときだけ、依存する設定の計画をその直前に移す。
// 先に前提の設定を無効にすると、GitHub が依存する設定も無効にし、その変更前提が成り立たなくなるため。
// 前提の設定がすでに無効で一致しているなら、依存する設定の変更予定は、有効にする向きが適用不能でこの並びまで届かず、
// 無効にする向きはすでに無効のため変更予定にならない。
func (r RepositoryPlan) executionOrder() []SettingPlan {
	ordered := r.Settings
	for _, d := range settingDependencies() {
		prerequisite := slices.IndexFunc(ordered, func(s SettingPlan) bool { return s.Unit.Key == d.prerequisite })
		dependent := slices.IndexFunc(ordered, func(s SettingPlan) bool { return s.Unit.Key == d.dependent })
		if prerequisite < 0 || dependent < 0 || ordered[prerequisite].Desired.Bool() {
			continue
		}
		moved := ordered[dependent]
		ordered = slices.Insert(slices.Delete(slices.Clone(ordered), dependent, dependent+1), prerequisite, moved)
	}
	return ordered
}

// criteriaFollowingPolicy は、遷移先が selected のポリシーの変更予定に続けて更新する、選択方式の Actions 許可条件の比較不能な変更予定を返す。
// 二つが対で計画にあるときだけ真を返す。
func (r RepositoryPlan) criteriaFollowingPolicy() (SettingPlan, bool) {
	policy, hasPolicy := r.settingPlanOf(setting.Unit{Key: setting.AllowedActions})
	criteria, hasCriteria := r.settingPlanOf(setting.Unit{Key: setting.SelectedActions})
	if !hasPolicy || !hasCriteria || !policy.IsPlannedChange() || !targetsSelectedPolicy(policy) || criteria.Result != NotComparable {
		return SettingPlan{}, false
	}
	return criteria, true
}

// settingPlanOf は、管理の単位 unit の計画を返す。管理範囲になければ偽を返す。
func (r RepositoryPlan) settingPlanOf(unit setting.Unit) (SettingPlan, bool) {
	for _, s := range r.Settings {
		if s.Unit.Same(unit) {
			return s, true
		}
	}
	return SettingPlan{}, false
}

// operationOf は、変更予定 s の設定の値を送る更新操作を返す。
// 基準は、s の計画時の観測結果、副作用として提示した設定のうち reco が観測するものの計画時の観測結果、副作用を示さなかった設定の計画時の観測結果である。
func operationOf(s SettingPlan) UpdateOperation {
	relevant := []RelevantSetting{{Unit: s.Unit, AnticipatedValue: AnticipateUnchanged(s.Observed)}}
	for _, effect := range s.SideEffects {
		if key, ok := effect.Kind.ObservedSetting(); ok {
			relevant = append(relevant, RelevantSetting{Unit: setting.Unit{Key: key}, AnticipatedValue: AnticipateUnchanged(effect.Observed)})
		}
	}
	for _, unshown := range s.SettingsWithoutSideEffect {
		relevant = append(relevant, RelevantSetting{Unit: setting.Unit{Key: unshown.Key}, AnticipatedValue: AnticipateUnchanged(unshown.Observed)})
	}
	return UpdateOperation{Kind: SettingUpdate, Unit: s.Unit, Value: s.Desired, RelevantSettings: relevant}
}

// shaPinningRequiredOperationOf は、sha_pinning_required の変更予定 s の更新操作を返す。
// 更新要求は必須の enabled に true を送るため、Actions が有効であることを基準に載せ、無効のリポジトリへ送って Actions を有効にしないようにする。
// 計画時に Actions が無効で観測不能だった値は基準に載せない（ドメインモデルの承認と変更前提）。
func shaPinningRequiredOperationOf(s SettingPlan) UpdateOperation {
	var relevant []RelevantSetting
	if s.Result == Mismatch {
		relevant = append(relevant, RelevantSetting{Unit: s.Unit, AnticipatedValue: AnticipateUnchanged(s.Observed)})
	}
	relevant = append(relevant, RelevantSetting{Unit: setting.Unit{Key: setting.ActionsEnabled}, AnticipatedValue: AnticipateValue(setting.BoolValue(true))})
	return UpdateOperation{Kind: SettingUpdate, Unit: s.Unit, Value: s.Desired, RelevantSettings: relevant}
}

// sideEffectSettingsOf は、変更予定 s の副作用として提示した設定のうち reco が観測するものを返す。
func sideEffectSettingsOf(s SettingPlan) []setting.Unit {
	var units []setting.Unit
	for _, effect := range s.SideEffects {
		if key, ok := effect.Kind.ObservedSetting(); ok {
			units = append(units, setting.Unit{Key: key})
		}
	}
	return units
}

func targetsSelectedPolicy(s SettingPlan) bool {
	return s.Unit.Key == setting.AllowedActions && s.Desired.AllowedActionsPolicy() == setting.AllowedActionsSelected
}

// checkUpdateEndpoint は、operation の更新要求の宛先を承認の前に決められるかを確かめ、決められなければ panic する。どれも、承認のあと先行する変更を送ってから止まらないようにするため。
// マージ方式は一つずつ送る更新要求を持たず、まとめた更新操作でだけ送るため、一つの設定の更新操作なら panic する。
// Ruleset の値の変更は宛先が計画時に観測した ID で決まるため、AnticipatedRulesetID を引けることを確かめる。
// Endpoint の戻り値を使わないのは、引けることの検査だけを目的とするため。最後に引くのは、マージ方式の破れを宛先の破れより具体的な理由で示すため。
func checkUpdateEndpoint(operation UpdateOperation) {
	switch {
	case operation.Kind == SettingUpdate && slices.Contains(setting.MergeMethods(), operation.Unit.Key):
		panic("changeplan: " + operation.Unit.String() + " is a merge method updated on its own")
	case operation.Kind == MergeMethodsUpdate && (len(operation.Settings) == 0 || slices.ContainsFunc(operation.Settings, func(s DesiredSetting) bool {
		return !slices.Contains(setting.MergeMethods(), s.Unit.Key)
	})):
		panic("changeplan: merge methods update of " + operation.Unit.String() + " does not send only merge methods")
	case operation.Kind == RulesetValueUpdate:
		operation.AnticipatedRulesetID()
	}
	operation.Endpoint()
}
