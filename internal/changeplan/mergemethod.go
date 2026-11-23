package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// mergeMethodsLeftDisabled は、管理設定の計画 s がマージ方式の確認済みの差分で、同じリポジトリの管理設定の計画 settings (s を含む) と
// 計画時の観測結果 observed から求めた更新後のマージ方式の値がすべて無効になるとき、無効のまま残るマージ方式を YAML 設定仕様の表の並びで返す (General の設計の適用可能性と副作用)。
// 当たらなければ nil を返す。有効にする差分は、s 自身の期待値が有効のため当たらない。
func mergeMethodsLeftDisabled(s SettingPlan, settings []SettingPlan, observed github.ObservationResult) []setting.Key {
	if !slices.Contains(setting.MergeMethods(), s.Unit.Key) || s.Result != Mismatch {
		return nil
	}
	var leftDisabled []setting.Key
	for _, key := range setting.MergeMethods() {
		unit := setting.Unit{Key: key}
		if i := slices.IndexFunc(settings, func(p SettingPlan) bool { return p.Unit.Same(unit) }); i >= 0 {
			if settings[i].Desired.Bool() {
				return nil
			}
			continue
		}
		if v, ok := observed.Setting(unit).Value(); ok && v.Bool() {
			return nil
		}
		leftDisabled = append(leftDisabled, key)
	}
	if len(leftDisabled) == 0 {
		panic(fmt.Sprintf("changeplan: every merge method is declared false with %s", s.Unit.Key))
	}
	return leftDisabled
}

// mergeMethodsChange は、同じリポジトリの管理設定の計画 settings のうちマージ方式の変更予定を、計画の並びで、一つの更新要求で送る一つの論理的な設定変更にまとめる
// (General の設計の論理的な設定変更と更新操作)。
// 基準は、まとめた各マージ方式の計画時の観測値だけで、宣言していないマージ方式と一致したマージ方式は含めない。
// マージ方式の変更予定がなければ送るものがないため、不変条件の破れとして panic する。
func mergeMethodsChange(settings []SettingPlan) LogicalChange {
	var desired []DesiredSetting
	var relevant []RelevantSetting
	for _, s := range settings {
		if !slices.Contains(setting.MergeMethods(), s.Unit.Key) || !s.IsPlannedChange() {
			continue
		}
		desired = append(desired, DesiredSetting{Unit: s.Unit, Value: s.Desired})
		relevant = append(relevant, RelevantSetting{Unit: s.Unit, AnticipatedValue: s.Observed})
	}
	if len(desired) == 0 {
		panic("changeplan: no merge method change to bundle")
	}
	return LogicalChange{
		DesiredState: desired,
		Operations:   []UpdateOperation{{Kind: MergeMethodsUpdate, Unit: desired[0].Unit, Settings: desired, RelevantSettings: relevant}},
	}
}
