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

// reconcile は、管理範囲の管理設定 (期待値が明示された管理の単位) だけを観測結果と照合する。
// 期待値が省略された設定は管理範囲外のため、照合結果を作らず、観測値も読まない。
// 並びは設定の領域の並び (YAML 設定仕様の章の並び) で、同じ領域の中では管理設定の列挙の並び (configuration.DesiredState.ManagedSettings) である。
// Environment Variable は、設定の領域の並びではなく、属する Environment の計画の直後に宣言の並びで置く (planOrderArea)。
// 宣言に名前のない Repository Variable、Environment、Environment Variable は、観測結果にあっても計画に現れない。
func reconcile(desired configuration.DesiredState, observed github.ObservationResult) []SettingPlan {
	var settings []SettingPlan
	for _, managed := range desired.ManagedSettings() {
		plan := newSettingPlan(managed.Unit, observed.Setting(managed.Unit), managed.Desired)
		plan.EnvironmentIdentity = environmentIdentityOf(managed.Unit, observed)
		settings = append(settings, plan)
	}
	slices.SortStableFunc(settings, func(a, b SettingPlan) int { return cmp.Compare(planOrderArea(a.Unit.Key), planOrderArea(b.Unit.Key)) })
	return settings
}

// planOrderArea は、変更計画の並びで key の管理の単位を置く設定の領域を返す。
// Environment Variable は Environments の領域に置き、管理設定の列挙が並べる Environment ごとの並びを保つ。
// 並びの理由は Environments の設計の論理的な設定変更と更新操作に従う。
func planOrderArea(key setting.Key) setting.Area {
	if key == setting.EnvironmentVariable {
		return setting.AreaEnvironments
	}
	return key.Area()
}

// managedAreas は、管理範囲内の管理設定が属する設定の領域を一度ずつ返す。
// 観測は領域の並びによらず決まった順に読むため、並びに意味はない。
// reconcile と同じ管理設定の列挙から決めるため、照合する設定の領域は必ず観測される。
func managedAreas(desired configuration.DesiredState) []setting.Area {
	var areas []setting.Area
	for _, managed := range desired.ManagedSettings() {
		if area := managed.Unit.Key.Area(); !slices.Contains(areas, area) {
			areas = append(areas, area)
		}
	}
	return areas
}

// managedEnvironments は、管理範囲内の管理設定が名指す Environment の名前を、管理設定の列挙の並びで一度ずつ返す。
// reconcile と同じ管理設定の列挙から決めるため、照合で引く Environment は必ず読まれる。
func managedEnvironments(desired configuration.DesiredState) []setting.EnvironmentName {
	var names []setting.EnvironmentName
	for _, managed := range desired.ManagedSettings() {
		if managed.Unit.Key.HasEnvironmentName() && !slices.Contains(names, managed.Unit.Environment) {
			names = append(names, managed.Unit.Environment)
		}
	}
	return names
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

// environmentIdentityOf は、Environment そのものの管理の単位 unit について、計画時に存在を観測した Environment の ID を同一性の基準にして返す。
// ほかの管理の単位と、存在すると観測しなかった Environment では、基準がないゼロ値を返す。
func environmentIdentityOf(unit setting.Unit, observed github.ObservationResult) EnvironmentIdentity {
	if unit.Key != setting.EnvironmentResource {
		return EnvironmentIdentity{}
	}
	environment, ok := observed.Environments.Lookup(unit.Environment).Existing()
	if !ok {
		return EnvironmentIdentity{}
	}
	return ObservedEnvironmentIdentity(environment.ID)
}
