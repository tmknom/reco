package changeplan

import (
	"slices"

	"github.com/tmknom/reco/internal/setting"
)

// previousSettings は、s が Actions を有効にする変更予定のとき、無効にする前の値のまま再開する設定を返す。当たらなければ nil を返す。
// GitHub は無効にする前の Actions 許可ポリシー、選択方式の Actions 許可条件、sha_pinning_required を残しているため、それらの値で再開する。
// ポリシーで有効にする変更予定では、ポリシーは宣言した値になり、選択方式の Actions 許可条件は宣言した値になるか副作用として示すため、sha_pinning_required だけが当たる。
// 同じリポジトリの管理設定 settings にある設定は、再開のあとで同じ変更計画が宣言した値に変えるため除く。
func previousSettings(s SettingPlan, settings []SettingPlan) []setting.Key {
	var resumed []setting.Key
	switch {
	case s.Key == setting.ActionsEnabled && s.Result == Mismatch && s.Desired.Bool():
		resumed = []setting.Key{setting.AllowedActions, setting.SelectedActions, setting.SHAPinningRequired}
	case s.EnablesActionsByPolicy():
		resumed = []setting.Key{setting.SHAPinningRequired}
	}
	resumed = slices.DeleteFunc(resumed, func(key setting.Key) bool {
		return slices.ContainsFunc(settings, func(other SettingPlan) bool { return other.Key == key })
	})
	if len(resumed) == 0 {
		return nil
	}
	return resumed
}
