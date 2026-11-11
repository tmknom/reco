package changeplan

import (
	"fmt"

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

// SettingAllowAutoMerge は、general.allow_auto_merge の YAML パス。
const SettingAllowAutoMerge = "general.allow_auto_merge"

// reconcile は、期待値が明示された設定 (管理範囲内) だけを観測結果と照合する。
// 期待値が省略された設定は管理範囲外のため、照合結果を作らない。
func reconcile(desired configuration.General, observed github.General) []SettingPlan {
	var settings []SettingPlan
	if desired.AllowAutoMerge != nil {
		settings = append(settings, newSettingPlan(SettingAllowAutoMerge, observedValue(SettingAllowAutoMerge, observed), *desired.AllowAutoMerge))
	}
	return settings
}

// Reconcile は、s の期待状態を observed と照合する。
// 承認した計画から期待状態を取り出すため、設定宣言を読み直さずに照合できる。
func (s SettingPlan) Reconcile(observed github.General) ReconciliationResult {
	return newSettingPlan(s.Setting, s.CurrentIn(observed), s.Desired).Result
}

// CurrentIn は、observed での s の設定の現在値を返す。
func (s SettingPlan) CurrentIn(observed github.General) bool {
	return observedValue(s.Setting, observed)
}

// observedValue は、setting の観測値を返す。
// setting は reconcile が作った計画から来るため、未知の設定は不変条件の破れとして panic する。
func observedValue(setting string, observed github.General) bool {
	switch setting {
	case SettingAllowAutoMerge:
		return observed.AllowAutoMerge
	default:
		panic(fmt.Sprintf("unknown setting %q", setting))
	}
}

func newSettingPlan(setting string, current, desired bool) SettingPlan {
	result := Match
	if current != desired {
		result = Mismatch
	}
	return SettingPlan{Setting: setting, Current: current, Desired: desired, Result: result}
}
