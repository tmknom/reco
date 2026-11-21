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
// 並びは設定の領域の並び (YAML 設定仕様の章の並び) で、同じ領域の中では setting.RepositorySettingUnits の並び、Environment は宣言の並びである。
// 宣言に名前のない Environment は、観測結果にあっても計画に現れない。
func reconcile(desired configuration.DesiredState, observed github.ObservationResult) []SettingPlan {
	var settings []SettingPlan
	for _, key := range setting.RepositorySettingUnits() {
		if d, ok := desired.Value(key); ok {
			settings = append(settings, newSettingPlan(key, observed.Setting(setting.Unit{Key: key}), d))
		}
	}
	for _, e := range desired.Environments {
		settings = append(settings, environmentPlans(e, observed.Environments.Lookup(e.Name))...)
	}
	slices.SortStableFunc(settings, func(a, b SettingPlan) int { return cmp.Compare(a.Unit.Key.Area(), b.Unit.Key.Area()) })
	return settings
}

// managedAreas は、管理範囲内の管理設定が属する設定の領域を一度ずつ返す。Environment を宣言していれば Environments を含む。
// 観測は領域の並びによらず決まった順に読むため、並びに意味はない。
// reconcile と同じ宣言で管理設定を決めるため、照合する設定の領域は必ず観測される。
func managedAreas(desired configuration.DesiredState) []setting.Area {
	var areas []setting.Area
	for _, key := range setting.RepositorySettingUnits() {
		if _, ok := desired.Value(key); ok && !slices.Contains(areas, key.Area()) {
			areas = append(areas, key.Area())
		}
	}
	if len(desired.Environments) > 0 {
		areas = append(areas, setting.AreaEnvironments)
	}
	return areas
}

// newSettingPlan は、観測値があれば期待値と比べ、観測不能なら Equal に渡さずに比較不能とする。
func newSettingPlan(key setting.Key, observed github.SettingObservation[setting.Value], desired setting.Value) SettingPlan {
	result := NotComparable
	if v, ok := observed.Value(); ok {
		result = Mismatch
		if v.Equal(desired) {
			result = Match
		}
	}
	return SettingPlan{Unit: setting.Unit{Key: key}, Observed: observed, Desired: desired, Result: result}
}
