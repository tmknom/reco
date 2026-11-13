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
// 次の表の並びが Plan.Settings の並びになる。
func reconcile(desired configuration.DesiredState, observed github.General) []SettingPlan {
	var settings []SettingPlan
	for _, s := range []struct {
		key     setting.Key
		desired *bool
	}{
		{setting.DeleteBranchOnMerge, desired.General.DeleteBranchOnMerge},
		{setting.AllowAutoMerge, desired.General.AllowAutoMerge},
	} {
		if s.desired != nil {
			settings = append(settings, newSettingPlan(s.key, observedValue(s.key, observed), *s.desired))
		}
	}
	return settings
}

// observedValue は、key の設定の観測値を返す。
// key は reconcile が管理範囲から作った計画に由来するため、未知の設定は不変条件の破れとして panic する。
func observedValue(key setting.Key, observed github.General) bool {
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
