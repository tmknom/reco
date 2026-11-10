package reco

import (
	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
)

// reconciliationResult は、期待状態と観測結果の照合結果。
type reconciliationResult int

const (
	match reconciliationResult = iota
	mismatch
	notComparable
)

// reconcile は、期待値が明示された設定 (管理範囲内) だけを観測結果と照合する。
// 期待値が省略された設定は管理範囲外のため、照合結果を作らない。
func reconcile(desired configuration.General, observed github.General) []settingPlan {
	var settings []settingPlan
	if desired.AllowAutoMerge != nil {
		settings = append(settings, newSettingPlan("general.allow_auto_merge", observed.AllowAutoMerge, *desired.AllowAutoMerge))
	}
	return settings
}

func newSettingPlan(setting string, current, desired bool) settingPlan {
	result := match
	if current != desired {
		result = mismatch
	}
	return settingPlan{setting: setting, current: current, desired: desired, result: result}
}
