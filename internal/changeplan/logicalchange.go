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
// Environment そのものの期待値は存在することで、値を持たないため Value をゼロ値にする。
type DesiredSetting struct {
	Unit  setting.Unit
	Value setting.Value
}

// UpdateOperation は、論理的な設定変更を構成する更新操作について、承認の対象になる内容。
// 送る内容のフィールドは Kind ごとに決まり、その種類が送らないフィールドはゼロ値にする。
type UpdateOperation struct {
	Kind UpdateOperationKind
	// Unit は、この更新操作が更新要求を送る先の管理の単位。
	Unit setting.Unit
	// Value は、SettingUpdate が送る設定の値。
	Value setting.Value
	// Mode は、EnvironmentCreationWithMode と DeploymentBranchPolicyModeUpdate が送るデプロイ元制限の方式。
	Mode setting.DeploymentBranchPolicyMode
	// Pattern は、DeploymentPatternAddition と DeploymentPatternRemoval が加えるか取り除く個別パターン。
	Pattern setting.DeploymentPattern
	// RelevantSettings は、この更新操作の変更前提の基準。変わっていたことを報告するときに優先する順に並べる。
	RelevantSettings []RelevantSetting
}

// UpdateOperationKind は、更新操作の種類。送る更新要求の形と、Environment を作るかどうかで分ける。
// EnvironmentCreationWithMode と DeploymentBranchPolicyModeUpdate は同じ形の要求を送るが、作成かどうかで変更前提の基準と表示が変わるため分ける。
type UpdateOperationKind int

// ゼロ値をいずれかの種類と取り違えないよう 1 から始める。
const (
	// SettingUpdate は、リポジトリ設定の値をその更新エンドポイントへ送る。
	SettingUpdate UpdateOperationKind = iota + 1
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
)

// RelevantSetting は、変更前提で比べる関係する現在値の設定と、その承認時に予定した値。
// AnticipatedValue と AnticipatedEnvironment は、Unit.Key の形に応じてどちらか一方だけを持つ。
type RelevantSetting struct {
	Unit setting.Unit
	// AnticipatedValue は、承認時に予定した値。
	AnticipatedValue github.SettingObservation[setting.Value]
	// AnticipatedEnvironment は、承認時に予定した存在と ID。
	// Environment そのものは値を持たないため、存在の観測結果を承認時に予定した値にする。
	AnticipatedEnvironment github.EnvironmentObservation
	// CreatedByPrecedingOperation は、承認時に予定した存在が、同じ論理的な設定変更の先行する更新操作による作成であること。
	// 真のとき AnticipatedEnvironment は持たない。承認の時点で ID が定まらないため。
	CreatedByPrecedingOperation bool
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

// LogicalChanges は、変更予定から論理的な設定変更を、executionOrder の実行順で導く。
// 適用可能でない変更予定があれば、承認の対象にしないため error にする。
// 副作用を伴う変更予定の更新操作は、operationOf が副作用として提示した設定のうち reco が観測するものを必ず基準に載せる。載せずに受け付けると承認の前に示していない値を失わせ得るため。
// 遷移先が selected のポリシーの変更予定と、選択方式の Actions 許可条件の比較不能な変更予定は、ポリシーの位置で一つの論理的な設定変更にまとめ、
// ポリシー、許可条件の順に二つの更新操作で実現する。
// 対にならない形は、設定宣言の組み合わせの規則と観測の取得条件のどちらかが破れたときにしか生じないため、不変条件の破れとして panic する。
// Actions の有効状態と Actions 許可ポリシーの計画が、照合結果によらず同じリポジトリの計画に並ぶ形も、
// 設定宣言の組み合わせの規則が破れたときにしか生じないため panic する。
// 有効状態の変更予定とポリシーの組を実行すると、有効状態の更新操作が無効にする前のポリシーで Actions を有効にし、宣言していない状態を残し得るため。
func (r RepositoryPlan) LogicalChanges() ([]LogicalChange, error) {
	for _, s := range r.Settings {
		if s.IsPlannedChange() && s.Applicability != Applicable {
			return nil, fmt.Errorf("%s: %s cannot be applied", r.Target, s.Unit.String())
		}
	}
	// Repository Variable の変更予定を実現する更新操作がないため、承認を求める前に止める
	for _, s := range r.Settings {
		if s.IsPlannedChange() && s.Unit.Key == setting.RepositoryVariable {
			return nil, fmt.Errorf("%s: %s cannot be applied: applying repository variable changes is not supported", r.Target, s.Unit.String())
		}
	}
	_, hasEnabled := r.settingPlanOf(setting.Unit{Key: setting.ActionsEnabled})
	if _, hasPolicy := r.settingPlanOf(setting.Unit{Key: setting.AllowedActions}); hasEnabled && hasPolicy {
		panic(fmt.Sprintf("changeplan: plans of %s and %s are in the same repository plan", setting.ActionsEnabled, setting.AllowedActions))
	}
	criteria, paired := r.criteriaFollowingPolicy()
	var changes []LogicalChange
	for _, s := range r.executionOrder() {
		switch {
		case !s.IsPlannedChange():
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
		}
		operation := operationOf(s)
		switch {
		case s.Unit.Key == setting.SHAPinningRequired:
			operation = shaPinningRequiredOperationOf(s)
		case s.Unit.Key == setting.RepositoryPushProtection && s.Desired.Bool():
			// GitHub は Secret scanning が無効のあいだ、有効にする更新に成功を応答して値を変えないため、更新直前に有効と確かめる。
			// 計画時に無効だった場合は、先行する Secret scanning の変更が有効にすることが予定どおりの変化である
			operation.RelevantSettings = append(operation.RelevantSettings, RelevantSetting{
				Unit: setting.Unit{Key: setting.SecretScanningEnabled}, AnticipatedValue: github.Observed(setting.BoolValue(true)),
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
				RelevantSettings: []RelevantSetting{{Unit: s.Unit, AnticipatedValue: github.Observed(s.Desired)}},
			})
		}
		changes = append(changes, change)
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
// Environment の名前は、GitHub が同じ Environment とみなす名前どうしを同じ単位にするため、EqualFold で比べる。
// 変数の名前は英小文字を含まないため、== で比べれば足りる。
func (r RepositoryPlan) settingPlanOf(unit setting.Unit) (SettingPlan, bool) {
	for _, s := range r.Settings {
		if s.Unit.Key == unit.Key && s.Unit.Environment.EqualFold(unit.Environment) && s.Unit.Variable == unit.Variable {
			return s, true
		}
	}
	return SettingPlan{}, false
}

// operationOf は、変更予定 s の設定の値を送る更新操作を返す。
// 基準は、s の計画時の観測結果、副作用として提示した設定のうち reco が観測するものの計画時の観測結果、副作用を示さなかった設定の計画時の観測結果である。
func operationOf(s SettingPlan) UpdateOperation {
	relevant := []RelevantSetting{{Unit: s.Unit, AnticipatedValue: s.Observed}}
	for _, effect := range s.SideEffects {
		if key, ok := effect.Kind.ObservedSetting(); ok {
			relevant = append(relevant, RelevantSetting{Unit: setting.Unit{Key: key}, AnticipatedValue: effect.Observed})
		}
	}
	for _, unshown := range s.SettingsWithoutSideEffect {
		relevant = append(relevant, RelevantSetting{Unit: setting.Unit{Key: unshown.Key}, AnticipatedValue: unshown.Observed})
	}
	return UpdateOperation{Kind: SettingUpdate, Unit: s.Unit, Value: s.Desired, RelevantSettings: relevant}
}

// shaPinningRequiredOperationOf は、sha_pinning_required の変更予定 s の更新操作を返す。
// 更新要求は必須の enabled に true を送るため、Actions が有効であることを基準に載せ、無効のリポジトリへ送って Actions を有効にしないようにする。
// 計画時に Actions が無効で観測不能だった値は基準に載せない（ドメインモデルの承認と変更前提）。
func shaPinningRequiredOperationOf(s SettingPlan) UpdateOperation {
	var relevant []RelevantSetting
	if s.Result == Mismatch {
		relevant = append(relevant, RelevantSetting{Unit: s.Unit, AnticipatedValue: s.Observed})
	}
	relevant = append(relevant, RelevantSetting{Unit: setting.Unit{Key: setting.ActionsEnabled}, AnticipatedValue: github.Observed(setting.BoolValue(true))})
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
