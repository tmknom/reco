package changeplan

import (
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// SideEffect は、変更予定に添える変更に伴う副作用で、GitHub が値を失わせる未管理設定を表す。
type SideEffect struct {
	Key setting.Key
	// Observed は、その設定の計画時の設定の観測結果。観測値なら失われる値を、観測不能なら失われる可能性だけを表す。
	Observed github.SettingObservation[setting.Value]
}

// sideEffects は、s の変更予定に伴う副作用を、アーキテクチャ設計の遷移表に従って返す。
// 現在のポリシーを観測できないとき (Actions が無効) も、無効にする前が selected だった可能性があるため副作用にする。
func sideEffects(s SettingPlan, observed github.ObservationResult) []SideEffect {
	if s.Key != setting.AllowedActions || s.Desired.AllowedActionsPolicy() == setting.AllowedActionsSelected {
		return nil
	}
	if current, ok := s.Observed.Value(); ok && current.AllowedActionsPolicy() != setting.AllowedActionsSelected {
		return nil
	}
	return []SideEffect{{Key: setting.SelectedActions, Observed: observed.Value(setting.SelectedActions)}}
}
