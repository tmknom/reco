package changeplan

import (
	"fmt"
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
// setting.Keys の並びが Plan.Settings の並びになる。
func reconcile(desired configuration.DesiredState, observed github.ObservationResult) []SettingPlan {
	var settings []SettingPlan
	for _, key := range setting.Keys() {
		if d, ok := desiredValue(key, desired); ok {
			settings = append(settings, newSettingPlan(key, ObservedValue(key, observed), d))
		}
	}
	return settings
}

// managedAreas は、管理範囲内の管理設定が属する設定の領域を、setting.Keys の並びで一度ずつ返す。
// reconcile と同じ desiredValue で管理設定を決めるため、照合する設定の領域は必ず観測される。
func managedAreas(desired configuration.DesiredState) []setting.Area {
	var areas []setting.Area
	for _, key := range setting.Keys() {
		if _, ok := desiredValue(key, desired); ok && !slices.Contains(areas, key.Area()) {
			areas = append(areas, key.Area())
		}
	}
	return areas
}

// desiredValue は、key の設定の期待値を返す。期待値が省略されていれば ok は偽になる。
// key は setting.Keys に由来するため、未知の設定は不変条件の破れとして panic する。
// 設定を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func desiredValue(key setting.Key, desired configuration.DesiredState) (SettingValue, bool) {
	switch key {
	case setting.DeleteBranchOnMerge:
		return boolDesiredValue(desired.General.DeleteBranchOnMerge)
	case setting.AllowAutoMerge:
		return boolDesiredValue(desired.General.AllowAutoMerge)
	case setting.DefaultWorkflowPermissions:
		if level := desired.WorkflowPermissions.DefaultWorkflowPermissions; level != nil {
			return PermissionLevelValue(*level), true
		}
		return SettingValue{}, false
	case setting.CanApprovePullRequestReviews:
		return boolDesiredValue(desired.WorkflowPermissions.CanApprovePullRequestReviews)
	}
	panic(fmt.Sprintf("unknown setting %q", key))
}

func boolDesiredValue(b *bool) (SettingValue, bool) {
	if b == nil {
		return SettingValue{}, false
	}
	return BoolValue(*b), true
}

// ObservedValue は、key の設定の観測値を返す。observed は key の設定の領域を読み取った観測の結果に限り、
// 読み取らなかった領域の値は補わずに nil 参照で止まる。未知の設定は不変条件の破れとして panic する。
func ObservedValue(key setting.Key, observed github.ObservationResult) SettingValue {
	switch key {
	case setting.DeleteBranchOnMerge:
		return BoolValue(observed.General.DeleteBranchOnMerge)
	case setting.AllowAutoMerge:
		return BoolValue(observed.General.AllowAutoMerge)
	case setting.DefaultWorkflowPermissions:
		return PermissionLevelValue(observed.WorkflowPermissions.DefaultWorkflowPermissions)
	case setting.CanApprovePullRequestReviews:
		return BoolValue(observed.WorkflowPermissions.CanApprovePullRequestReviews)
	}
	panic(fmt.Sprintf("unknown setting %q", key))
}

func compare(observed, desired SettingValue) ReconciliationResult {
	if observed != desired {
		return Mismatch
	}
	return Match
}

func newSettingPlan(key setting.Key, observed, desired SettingValue) SettingPlan {
	return SettingPlan{Key: key, Observed: observed, Desired: desired, Result: compare(observed, desired)}
}
