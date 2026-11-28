package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// SideEffect は、変更予定に添える変更に伴う副作用で、GitHub が変える未管理設定か、Environment の削除で一緒に消える観測した変数と Secret と、
// それに起きる変化を表す。
type SideEffect struct {
	Kind SideEffectKind
	// Observed は、Kind が reco の観測する設定に及ぶときだけ持つ、その設定の計画時の設定の観測結果。
	// 観測値なら変わる前の値を、観測不能なら失われる可能性だけを表す。観測しない設定に及ぶ副作用ではゼロ値。
	// 一緒に消える変数と Secret の副作用もゼロ値で、消える一覧は計画の削除で失われるもの (SettingPlan.EnvironmentLoss) が持つ。
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
	// push protection が知らない値なら、Observed は観測不能で、無効になる可能性を表す。
	SideEffectPushProtectionDisabled
	// SideEffectPreventSelfReviewDiscarded は、担当者をすべて外す変更で、Environment の自己承認の禁止が失われること。Observed は失われる前の値の真。
	SideEffectPreventSelfReviewDiscarded
	// SideEffectEnvironmentVariablesDeleted は、Environment の削除で、その Environment Variable が名前と値ごと消えること。
	// 消える変数の一覧は SettingPlan.EnvironmentLoss が持つ。
	SideEffectEnvironmentVariablesDeleted
	// SideEffectEnvironmentSecretsDeleted は、Environment の削除で、その Environment Secret が消えること。
	// 消える Secret の一覧は SettingPlan.EnvironmentLoss が持つ。
	SideEffectEnvironmentSecretsDeleted
)

// SettingExpectedUnaffected は、副作用を示さなかった設定と、その計画時の設定の観測結果。
// Key が Environment の名前を含む形なら、変更予定と同じ Environment の中の設定を指す。
type SettingExpectedUnaffected struct {
	Key      setting.Key
	Observed github.SettingObservation[setting.Value]
}

// ObservedSetting は、副作用が及ぶ設定のうち、reco が観測する設定の形を返す。Environment の名前を含む形は、変更予定と同じ Environment の中の設定を指す。
// Dependency graph は観測しないため偽を返し、その副作用は変更前提にも更新後の比較にも含めない。
// 一緒に消える変数と Secret は設定の識別を持たないため偽を返す。変更前提では、計画時の一覧を AnticipatedEnvironmentLoss で確かめる。
// 未定義の種類は不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (k SideEffectKind) ObservedSetting() (setting.Key, bool) {
	switch k {
	case SideEffectSelectedActionsDiscarded:
		return setting.SelectedActions, true
	case SideEffectSecurityUpdatesDisabled:
		return setting.DependabotSecurityUpdates, true
	case SideEffectPushProtectionDisabled:
		return setting.RepositoryPushProtection, true
	case SideEffectPreventSelfReviewDiscarded:
		return setting.EnvironmentPreventSelfReview, true
	case SideEffectDependencyGraphEnabled, SideEffectDependencyGraphDisabled, SideEffectEnvironmentVariablesDeleted, SideEffectEnvironmentSecretsDeleted:
		return "", false
	}
	panic(fmt.Sprintf("changeplan: undefined side effect kind %d", int(k)))
}

// sideEffects は、s の変更予定に伴う副作用と、副作用を示さなかった設定を、設定ごとの設計の遷移表と設定の依存関係に従って返す。
// settings は s と同じリポジトリの管理設定の計画で、宣言した設定の変化は管理設定の変更として扱い、どちらにも数えない。
// 比較不能な変更予定は、前提の設定では適用不能 (非公開リポジトリか知らない値の Secret scanning) に限られ、更新しないため副作用を添えない。
// Environment の削除は、計画時に読んだ削除で失われるもの (SettingPlan.EnvironmentLoss) から副作用を決める。
func sideEffects(s SettingPlan, observed github.ObservationResult, settings []SettingPlan) ([]SideEffect, []SettingExpectedUnaffected) {
	if s.Unit.Key == setting.AllowedActions {
		return selectedActionsSideEffects(s, observed), nil
	}
	if s.Unit.Key == setting.EnvironmentReviewerSet {
		return preventSelfReviewSideEffects(s, observed, settings)
	}
	if s.Result != Mismatch {
		return nil, nil
	}
	if s.EnvironmentLoss != nil {
		return environmentLossSideEffects(*s.EnvironmentLoss), nil
	}
	var effects []SideEffect
	if s.Unit.Key == setting.DependabotAlerts {
		effects = append(effects, dependencyGraphSideEffect(s))
	}
	dependentEffects, unaffected := dependentSideEffects(s, observed, settings)
	return append(effects, dependentEffects...), unaffected
}

// selectedActionsSideEffects は、Actions 許可ポリシーの変更予定 s が選択方式の Actions 許可条件を失わせるときの副作用を返す。
// 現在のポリシーを Actions が無効のために観測できないときも、無効にする前が selected だった可能性があるため副作用にする。
// 知らない値のために観測できないポリシーの変更予定は適用不能で更新しないため、副作用を添えない。
func selectedActionsSideEffects(s SettingPlan, observed github.ObservationResult) []SideEffect {
	if s.Observed.HasUnknownValue() || s.Desired.AllowedActionsPolicy() == setting.AllowedActionsSelected {
		return nil
	}
	if current, ok := s.Observed.Value(); ok && current.AllowedActionsPolicy() != setting.AllowedActionsSelected {
		return nil
	}
	return []SideEffect{{Kind: SideEffectSelectedActionsDiscarded, Observed: observed.Setting(setting.Unit{Key: setting.SelectedActions})}}
}

// preventSelfReviewSideEffects は、担当者の集合の不一致 s が担当者をすべて外すとき、GitHub が同じ Environment の自己承認の禁止を偽にする変化を、
// 副作用か副作用を示さなかった設定として返す。計画時に真なら値が失われる副作用で、偽なら示さずに変更前提で偽のままであることを確かめる。
// 承認のあとに外部主体が真にした値を、示さないまま失わせないため。
// 同じ Environment の自己承認の禁止を宣言していれば、この変化は管理設定の変更か一致で、どちらにも数えない (Environments の設計の適用可能性と副作用)。
// 不一致でない計画は、存在しない Environment (失われる値がない) か、保護ルールのない一致のため、何も返さない。
func preventSelfReviewSideEffects(s SettingPlan, observed github.ObservationResult, settings []SettingPlan) ([]SideEffect, []SettingExpectedUnaffected) {
	if s.Result != Mismatch || !s.Desired.ReviewerSet().HasNoReviewers() {
		return nil, nil
	}
	unit := setting.Unit{Key: setting.EnvironmentPreventSelfReview, Environment: s.Unit.Environment}
	if _, declared := (RepositoryPlan{Settings: settings}).settingPlanOf(unit); declared {
		return nil, nil
	}
	current := observed.Setting(unit)
	if value, ok := current.Value(); ok && value.Bool() {
		return []SideEffect{{Kind: SideEffectPreventSelfReviewDiscarded, Observed: current}}, nil
	}
	return nil, []SettingExpectedUnaffected{{Key: setting.EnvironmentPreventSelfReview, Observed: current}}
}

// environmentLossSideEffects は、Environment の削除が失わせるもの loss のうち、一緒に消える変数と Secret を、計画時に一つ以上を観測した種類だけ、変数、Secret の順に副作用として返す。
// 副作用は一覧を持たず、示す一覧と変更前提で比べる一覧は loss だけが持つ。示す内容と比べる内容を二か所で持たないため。
// 適用不能な削除は loss を持たず、更新しないため副作用を添えない (呼び出し元が loss のある計画に限る)。
func environmentLossSideEffects(loss EnvironmentLoss) []SideEffect {
	var effects []SideEffect
	if len(loss.Variables) > 0 {
		effects = append(effects, SideEffect{Kind: SideEffectEnvironmentVariablesDeleted})
	}
	if len(loss.Secrets) > 0 {
		effects = append(effects, SideEffect{Kind: SideEffectEnvironmentSecretsDeleted})
	}
	return effects
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
// 依存する設定が知らない値で観測不能なら、無効になる可能性を、観測不能の観測結果つきの副作用として示す。値を観測できないことは、副作用がないことを意味しないため。
// 前提の設定を観測できて依存する設定をほかの理由で観測できない形は、同じ領域の取得から生じないため、不変条件の破れとして panic する。
func dependentSideEffects(s SettingPlan, observed github.ObservationResult, settings []SettingPlan) ([]SideEffect, []SettingExpectedUnaffected) {
	d, ok := dependencyOnPrerequisite(s.Unit.Key)
	if !ok || s.Desired.Bool() || declares(settings, d.Dependent) {
		return nil, nil
	}
	dependent := observed.Setting(setting.Unit{Key: d.Dependent})
	if dependent.HasUnknownValue() {
		return []SideEffect{{Kind: dependentDisabledSideEffect(d.Dependent), Observed: dependent}}, nil
	}
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
