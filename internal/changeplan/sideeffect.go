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
)

// SettingWithoutSideEffect は、副作用を示さなかった設定と、その計画時の設定の観測結果。
type SettingWithoutSideEffect struct {
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
	case SideEffectDependencyGraphEnabled, SideEffectDependencyGraphDisabled:
		return "", false
	}
	panic(fmt.Sprintf("changeplan: undefined side effect kind %d", int(k)))
}

// sideEffects は、s の変更予定に伴う副作用と、副作用を示さなかった設定を、アーキテクチャ設計の遷移表と依存の表に従って返す。
// settings は s と同じリポジトリの管理設定の計画で、宣言した設定の変化は管理設定の変更として扱い、どちらにも数えない。
func sideEffects(s SettingPlan, observed github.ObservationResult, settings []SettingPlan) ([]SideEffect, []SettingWithoutSideEffect) {
	if s.Unit.Key == setting.AllowedActions {
		return selectedActionsSideEffects(s, observed), nil
	}
	if s.Unit.Key == setting.DependabotAlerts {
		return dependabotAlertsSideEffects(s, observed, settings)
	}
	return nil, nil
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

// dependabotAlertsSideEffects は、Dependabot alerts の変更予定 s に伴う副作用と、副作用を示さなかった設定を返す。
// 無効にすると、GitHub は Dependency graph と、有効だった Dependabot security updates も無効にする。
// Dependency graph は観測しないため、計画時の値によらず起きる変化だけを示す。
// 宣言していない Dependabot security updates は、有効なら副作用として示し、無効なら示さずに変更前提で無効のままであることを確かめる。
// 承認のあとに外部主体が有効にした Dependabot security updates を、示さないまま無効にしないため。
func dependabotAlertsSideEffects(s SettingPlan, observed github.ObservationResult, settings []SettingPlan) ([]SideEffect, []SettingWithoutSideEffect) {
	if !s.IsPlannedChange() {
		return nil, nil
	}
	if s.Desired.Bool() {
		return []SideEffect{{Kind: SideEffectDependencyGraphEnabled}}, nil
	}
	effects := []SideEffect{{Kind: SideEffectDependencyGraphDisabled}}
	if declares(settings, setting.DependabotSecurityUpdates) {
		return effects, nil
	}
	updates := observed.Setting(setting.Unit{Key: setting.DependabotSecurityUpdates})
	if observed.Dependabot.SecurityUpdates.Enabled() {
		return append(effects, SideEffect{Kind: SideEffectSecurityUpdatesDisabled, Observed: updates}), nil
	}
	return effects, []SettingWithoutSideEffect{{Key: setting.DependabotSecurityUpdates, Observed: updates}}
}

// declares は、管理設定の計画 settings に key の計画があるかを返す。
func declares(settings []SettingPlan, key setting.Key) bool {
	return slices.ContainsFunc(settings, func(s SettingPlan) bool { return s.Unit.Key == key })
}
