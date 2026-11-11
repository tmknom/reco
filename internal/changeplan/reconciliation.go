package changeplan

import (
	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
)

// ReconciliationResult は、期待状態と観測結果の照合結果。
type ReconciliationResult int

const (
	Match ReconciliationResult = iota
	Mismatch
	NotComparable
)

// reconcile は、期待値が明示された設定 (管理範囲内) だけを観測結果と照合する。
// 期待値が省略された設定は管理範囲外のため、照合結果を作らない。
func reconcile(desired configuration.General, observed github.General) []SettingPlan {
	var settings []SettingPlan
	if desired.AllowAutoMerge != nil {
		settings = append(settings, newSettingPlan("general.allow_auto_merge", observed.AllowAutoMerge, *desired.AllowAutoMerge))
	}
	return settings
}

func newSettingPlan(setting string, current, desired bool) SettingPlan {
	result := Match
	if current != desired {
		result = Mismatch
	}
	return SettingPlan{Setting: setting, Current: current, Desired: desired, Result: result}
}
