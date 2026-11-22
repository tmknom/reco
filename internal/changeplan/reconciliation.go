package changeplan

import (
	"cmp"
	"slices"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// ReconciliationResult は、期待値と観測結果の照合結果。
type ReconciliationResult int

const (
	Match ReconciliationResult = iota
	Mismatch
	NotComparable
)

// reconcile は、期待値が明示された設定 (管理範囲内) だけを観測結果と照合する。
// 期待値が省略された設定は管理範囲外のため、照合結果を作らず、観測値も読まない。
// 並びは設定の領域の並び (YAML 設定仕様の章の並び) で、同じ領域の中では setting.RepositorySettingUnits の並び、Repository Variable と Environment は宣言の並びである。
// Environment Variable は、設定の領域の並びではなく、属する Environment の計画の直後に宣言の並びで置く (planOrderArea)。
// 宣言に名前のない Repository Variable、Environment、Environment Variable は、観測結果にあっても計画に現れない。
func reconcile(desired configuration.DesiredState, observed github.ObservationResult) []SettingPlan {
	var settings []SettingPlan
	for _, key := range setting.RepositorySettingUnits() {
		if d, ok := desired.Value(key); ok {
			unit := setting.Unit{Key: key}
			settings = append(settings, newSettingPlan(unit, observed.Setting(unit), d))
		}
	}
	for _, v := range desired.RepositoryVariables {
		unit := setting.Unit{Key: setting.RepositoryVariable, Variable: v.Name}
		settings = append(settings, newSettingPlan(unit, observed.Setting(unit), v.Desired))
	}
	for _, e := range desired.Environments {
		settings = append(settings, environmentPlans(e, observed.Environments.Lookup(e.Name))...)
	}
	slices.SortStableFunc(settings, func(a, b SettingPlan) int { return cmp.Compare(planOrderArea(a.Unit.Key), planOrderArea(b.Unit.Key)) })
	return settings
}

// planOrderArea は、変更計画の並びで key の管理の単位を置く設定の領域を返す。
// Environment Variable は Environments の領域に置き、environmentPlans が返す Environment ごとの並びを保つ。
// 並びの理由は Environments の設計の論理的な設定変更と更新操作に従う。
func planOrderArea(key setting.Key) setting.Area {
	if key == setting.EnvironmentVariable {
		return setting.AreaEnvironments
	}
	return key.Area()
}

// managedAreas は、管理範囲内の管理設定が属する設定の領域を一度ずつ返す。
// Repository Variable を宣言していれば Repository Variables を、Environment を宣言していれば Environments を、
// どれかの Environment に変数を宣言していれば Environment Variables を含む。
// 観測は領域の並びによらず決まった順に読むため、並びに意味はない。
// reconcile と同じ宣言で管理設定を決めるため、照合する設定の領域は必ず観測される。
func managedAreas(desired configuration.DesiredState) []setting.Area {
	var areas []setting.Area
	for _, key := range setting.RepositorySettingUnits() {
		if _, ok := desired.Value(key); ok && !slices.Contains(areas, key.Area()) {
			areas = append(areas, key.Area())
		}
	}
	if len(desired.RepositoryVariables) > 0 {
		areas = append(areas, setting.AreaRepositoryVariables)
	}
	if len(desired.Environments) > 0 {
		areas = append(areas, setting.AreaEnvironments)
	}
	if slices.ContainsFunc(desired.Environments, func(e configuration.EnvironmentDeclaration) bool { return len(e.Variables) > 0 }) {
		areas = append(areas, setting.AreaEnvironmentVariables)
	}
	return areas
}

// newSettingPlan は、観測値があれば期待値と比べ、観測不能なら Equal に渡さずに比較不能とする。
func newSettingPlan(unit setting.Unit, observed github.SettingObservation[setting.Value], desired setting.Value) SettingPlan {
	result := NotComparable
	if v, ok := observed.Value(); ok {
		result = Mismatch
		if v.Equal(desired) {
			result = Match
		}
	}
	return SettingPlan{Unit: unit, Observed: observed, Desired: desired, Result: result}
}
