package changeplan

import (
	"fmt"

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
// 期待値が省略された設定は管理範囲外のため、照合結果を作らない。
// setting.GeneralKeys の並びが Plan.Settings の並びになる。
func reconcile(desired configuration.DesiredState, observed github.General) []SettingPlan {
	var settings []SettingPlan
	for _, key := range setting.GeneralKeys() {
		if d := desiredValue(key, desired.General); d != nil {
			settings = append(settings, newSettingPlan(key, ObservedValue(key, observed), *d))
		}
	}
	return settings
}

// desiredValue は、key の設定の期待値を返す。期待値が省略されていれば nil を返す。
// key は setting.GeneralKeys に由来するため、未知の設定は不変条件の破れとして panic する。
func desiredValue(key setting.Key, desired configuration.General) *bool {
	switch key {
	case setting.DeleteBranchOnMerge:
		return desired.DeleteBranchOnMerge
	case setting.AllowAutoMerge:
		return desired.AllowAutoMerge
	default:
		panic(fmt.Sprintf("unknown setting %q", key))
	}
}

// ObservedValue は、key の設定の観測値を返す。未知の設定は不変条件の破れとして panic する。
func ObservedValue(key setting.Key, observed github.General) bool {
	switch key {
	case setting.DeleteBranchOnMerge:
		return observed.DeleteBranchOnMerge
	case setting.AllowAutoMerge:
		return observed.AllowAutoMerge
	default:
		panic(fmt.Sprintf("unknown setting %q", key))
	}
}

func compare(observed, desired bool) ReconciliationResult {
	if observed != desired {
		return Mismatch
	}
	return Match
}

func newSettingPlan(key setting.Key, observed, desired bool) SettingPlan {
	return SettingPlan{Key: key, Observed: observed, Desired: desired, Result: compare(observed, desired)}
}
