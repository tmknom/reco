package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// SideEffect は、変更予定に添える変更に伴う副作用で、GitHub が変える未管理設定と、起きる変化を表す。
type SideEffect struct {
	Kind SideEffectKind
	// Observed は、Kind が reco の観測する設定に及ぶときだけ持つ、その設定の計画時の設定の観測結果。
	// 観測値なら変わる前の値を、観測不能なら失われる可能性だけを表す。観測しない設定に及ぶ副作用ではゼロ値。
	Observed github.SettingObservation[setting.Value]
}

// SideEffectKind は、副作用が及ぶ設定と、その設定に起きる変化の組。表示の文言と、変更前提に含めるかはこれで決まる。
type SideEffectKind int

// ゼロ値を定義済みの種類と取り違えないよう 1 から始める。
const (
	// SideEffectSelectedActionsDiscarded は、選択方式の Actions 許可条件が失われること。
	SideEffectSelectedActionsDiscarded SideEffectKind = iota + 1
	// SideEffectDependencyGraphEnabled は、Dependency graph が有効になること。
	SideEffectDependencyGraphEnabled
	// SideEffectDependencyGraphDisabled は、Dependency graph が無効になること。
	SideEffectDependencyGraphDisabled
	// SideEffectSecurityUpdatesDisabled は、Dependabot security updates が無効になること。
	SideEffectSecurityUpdatesDisabled
	// SideEffectPushProtectionDisabled は、リポジトリの push protection が無効になること。
	SideEffectPushProtectionDisabled
)

// SettingExpectedUnaffected は、副作用を示さなかった設定と、その計画時の設定の観測結果。
type SettingExpectedUnaffected struct {
	Key      setting.Key
	Observed github.SettingObservation[setting.Value]
}

// ObservedSetting は、副作用が及ぶ設定のうち reco が観測するものを返す。
// Dependency graph は観測しないため偽を返し、その副作用は変更前提にも更新後の比較にも含めない。
// 未定義の種類は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (k SideEffectKind) ObservedSetting() (setting.Key, bool) {
	switch k {
	case SideEffectSelectedActionsDiscarded:
		return setting.SelectedActions, true
	case SideEffectSecurityUpdatesDisabled:
		return setting.DependabotSecurityUpdates, true
	case SideEffectPushProtectionDisabled:
		return setting.RepositoryPushProtection, true
	case SideEffectDependencyGraphEnabled, SideEffectDependencyGraphDisabled:
		return "", false
	}
	panic(fmt.Sprintf("changeplan: undefined side effect kind %d", int(k)))
}

// sideEffects は、s の変更予定に伴う副作用と、副作用を示さなかった設定を、設定ごとの設計の遷移表と設定の依存関係に従って返す。
// settings は s と同じリポジトリの管理設定の計画で、宣言した設定の変化は管理設定の変更として扱い、どちらにも数えない。
// 比較不能な変更予定は、前提の設定では適用不能 (非公開リポジトリの Secret scanning) に限られ、更新しないため副作用を添えない。
func sideEffects(s SettingPlan, observed github.ObservationResult, settings []SettingPlan) ([]SideEffect, []SettingExpectedUnaffected) {
	if s.Unit.Key == setting.AllowedActions {
		return selectedActionsSideEffects(s, observed), nil
	}
	if s.Result != Mismatch {
		return nil, nil
	}
	var effects []SideEffect
	if s.Unit.Key == setting.DependabotAlerts {
		effects = append(effects, dependencyGraphSideEffect(s))
	}
	dependentEffects, unaffected := dependentSideEffects(s, observed, settings)
	return append(effects, dependentEffects...), unaffected
}

// selectedActionsSideEffects は、Actions 許可ポリシーの変更予定 s が選択方式の Actions 許可条件を失わせるときの副作用を返す。
// 現在のポリシーを観測できないとき (Actions が無効) も、無効にする前が selected だった可能性があるため副作用にする。
func selectedActionsSideEffects(s SettingPlan, observed github.ObservationResult) []SideEffect {
	if s.Desired.AllowedActionsPolicy() == setting.AllowedActionsSelected {
		return nil
	}
	if current, ok := s.Observed.Value(); ok && current.AllowedActionsPolicy() != setting.AllowedActionsSelected {
		return nil
	}
	return []SideEffect{{Kind: SideEffectSelectedActionsDiscarded, Observed: observed.Setting(setting.Unit{Key: setting.SelectedActions})}}
}

// dependencyGraphSideEffect は、Dependabot alerts の不一致 s に伴う Dependency graph の副作用を返す。
// Dependency graph は観測しないため、計画時の値によらず起きる変化だけを示す。
func dependencyGraphSideEffect(s SettingPlan) SideEffect {
	if s.Desired.Bool() {
		return SideEffect{Kind: SideEffectDependencyGraphEnabled}
	}
	return SideEffect{Kind: SideEffectDependencyGraphDisabled}
}

// dependentSideEffects は、前提の設定を無効にする不一致 s に伴う、依存する設定の副作用と、副作用を示さなかった設定を返す。
// 宣言していない依存する設定は、有効なら副作用として示し、無効なら示さずに変更前提で無効のままであることを確かめる。
// 承認のあとに外部主体が有効にした依存する設定を、示さないまま無効にしないため。
// 前提の設定を観測できて依存する設定を観測できない形は、同じ領域の取得から生じないため、不変条件の破れとして panic する。
func dependentSideEffects(s SettingPlan, observed github.ObservationResult, settings []SettingPlan) ([]SideEffect, []SettingExpectedUnaffected) {
	d, ok := dependencyOnPrerequisite(s.Unit.Key)
	if !ok || s.Desired.Bool() || declares(settings, d.Dependent) {
		return nil, nil
	}
	dependent := observed.Setting(setting.Unit{Key: d.Dependent})
	current, ok := dependent.Value()
	if !ok {
		panic(fmt.Sprintf("changeplan: %s is observed without %s", s.Unit.Key, d.Dependent))
	}
	// 無効の値と Equal で比べるため、一時停止中の Dependabot security updates は有効として副作用に示す
	if current.Equal(d.DisabledDependent) {
		return nil, []SettingExpectedUnaffected{{Key: d.Dependent, Observed: dependent}}
	}
	return []SideEffect{{Kind: dependentDisabledSideEffect(d.Dependent), Observed: dependent}}, nil
}

// dependencyOnPrerequisite は、key を前提の設定とする設定の依存関係を返す。前提の設定でなければ偽を返す。
func dependencyOnPrerequisite(key setting.Key) (setting.Dependency, bool) {
	for _, d := range setting.Dependencies() {
		if d.Prerequisite == key {
			return d, true
		}
	}
	return setting.Dependency{}, false
}

// dependentDisabledSideEffect は、前提の設定を無効にする更新操作が依存する設定 dependent を無効にする副作用の種類を返す。
// 設定の依存関係を足して副作用の種類を足し忘れると、無効になる依存する設定を示さずに承認を求め得るため、不変条件の破れとして panic する。
func dependentDisabledSideEffect(dependent setting.Key) SideEffectKind {
	switch dependent {
	case setting.DependabotSecurityUpdates:
		return SideEffectSecurityUpdatesDisabled
	case setting.RepositoryPushProtection:
		return SideEffectPushProtectionDisabled
	default:
		panic(fmt.Sprintf("changeplan: %s has no side effect of being disabled", dependent))
	}
}

// declares は、管理設定の計画 settings に key の計画があるかを返す。
func declares(settings []SettingPlan, key setting.Key) bool {
	return slices.ContainsFunc(settings, func(s SettingPlan) bool { return s.Unit.Key == key })
}
