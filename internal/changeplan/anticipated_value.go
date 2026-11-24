package changeplan

import (
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// AnticipatedValue は、承認時に予定した値 (ドメインモデルの承認と変更前提)。
// 設定の観測結果と同じ形をとるが、観測で確かめた事実ではなく承認の対象になる見込みのため、観測結果の型と分ける。
// 同じ型にすると、予定と更新直前の観測結果を取り違えて比べてもコンパイルが通るため。
type AnticipatedValue struct {
	basis github.SettingObservation[setting.Value]
}

// AnticipateValue は、値が value であることを予定する。先行する更新操作による予定どおりの変化を反映した値などに使う。
func AnticipateValue(value setting.Value) AnticipatedValue {
	return AnticipatedValue{basis: github.Observed(value)}
}

// AnticipateUnchanged は、計画時の観測結果 observed から変わらないことを予定する。
// 観測不能だった設定は、同じ理由で観測不能のままであることを予定する。
func AnticipateUnchanged(observed github.SettingObservation[setting.Value]) AnticipatedValue {
	return AnticipatedValue{basis: observed}
}

// Value は、値を予定したときだけ、その値と真を返す。
func (a AnticipatedValue) Value() (setting.Value, bool) {
	return a.basis.Value()
}

// UnavailableReason は、観測不能のままであることを予定したときの理由を返す。値を予定したとき、およびゼロ値では、不変条件の破れとして panic する。
func (a AnticipatedValue) UnavailableReason() github.UnavailableReason {
	return a.basis.UnavailableReason()
}
