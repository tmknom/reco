package changeplan

import (
	"slices"

	"github.com/tmknom/reco/internal/setting"
)

// resumedSettings は、s が Actions を有効にする変更予定のとき、再開する設定を返す。当たらなければ nil を返す。
func resumedSettings(s SettingPlan, settings []SettingPlan) []setting.Key {
	var resumed []setting.Key
	switch {
	case s.Unit.Key == setting.ActionsEnabled && s.Result == Mismatch && s.Desired.Bool():
		resumed = []setting.Key{setting.AllowedActions, setting.SelectedActions, setting.SHAPinningRequired}
	case s.EnablesActionsByPolicy():
		resumed = []setting.Key{setting.SHAPinningRequired}
	}
	resumed = slices.DeleteFunc(resumed, func(key setting.Key) bool {
		return slices.ContainsFunc(settings, func(other SettingPlan) bool { return other.Unit.Key == key })
	})
	if len(resumed) == 0 {
		return nil
	}
	return resumed
}
