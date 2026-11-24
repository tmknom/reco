package apply

import (
	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// desiredStateConfirmation は、更新後の観測で期待状態を確かめた結果。
// desiredStateUnconfirmed、desiredStateIdentityMismatch、desiredStateUnavailable は、どれも確認失敗を理由で分けたもの。
// desiredStateUnavailable は、期待値と比べる設定が理由を伴う観測不能だったことを表す。
type desiredStateConfirmation int

const (
	desiredStateNotChecked desiredStateConfirmation = iota
	desiredStateMatches
	desiredStateMismatch
	desiredStateUnconfirmed
	desiredStateIdentityMismatch
	desiredStateUnavailable
)

// unavailableDesiredSetting は、期待状態の確認で期待値と比べる設定が観測不能だった管理設定と、その理由。
// 確認結果の表示は、理由だけでなく管理設定の種類からも文言を選ぶため、二つを組で持つ。
type unavailableDesiredSetting struct {
	unit   setting.Unit
	reason github.UnavailableReason
}

// confirmDesiredState は、更新後の観測が、論理的な設定変更の期待状態と一致するかを判定する。
// 観測できた値に期待値と異なるものがあれば、ほかの設定が観測不能でも不一致にする。
// そうでなく期待値と比べる設定に観測不能があれば、値を確認できていないため確認失敗にし、期待状態の並びで最初に観測不能だった管理設定を返す。
// 期待状態のない論理的な設定変更を一致にすると何も確かめずに完了になるため、不変条件の破れとして panic する。
func confirmDesiredState(approvedID repository.ID, change changeplan.LogicalChange, observed github.ObservationResult) (desiredStateConfirmation, unavailableDesiredSetting) {
	if len(change.DesiredState) == 0 {
		panic("apply: logical change has no desired state to confirm")
	}
	if observed.RepositoryID != approvedID {
		return desiredStateIdentityMismatch, unavailableDesiredSetting{}
	}
	// 不一致は mismatchedDesiredSettings から導き、不一致の確認結果が一致しなかった管理設定を必ず伴うようにする
	if len(mismatchedDesiredSettings(change, observed)) > 0 {
		return desiredStateMismatch, unavailableDesiredSetting{}
	}
	for _, desired := range change.DesiredState {
		if confirmation, reason := confirmDesiredSetting(desired, observed); confirmation == desiredStateUnavailable {
			return desiredStateUnavailable, unavailableDesiredSetting{unit: desired.Unit, reason: reason}
		}
	}
	return desiredStateMatches, unavailableDesiredSetting{}
}

// mismatchedDesiredSettings は、change の期待状態のうち、observed で期待値と異なる値を観測した管理設定を、期待状態の並びで返す。観測不能だった管理設定は含めない。
func mismatchedDesiredSettings(change changeplan.LogicalChange, observed github.ObservationResult) []setting.Unit {
	var mismatched []setting.Unit
	for _, desired := range change.DesiredState {
		if confirmation, _ := confirmDesiredSetting(desired, observed); confirmation == desiredStateMismatch {
			mismatched = append(mismatched, desired.Unit)
		}
	}
	return mismatched
}

// confirmDesiredSetting は、一つの管理設定の期待値を observed と比べ、一致、不一致、理由を伴う観測不能のいずれかを返す。
func confirmDesiredSetting(desired changeplan.DesiredSetting, observed github.ObservationResult) (desiredStateConfirmation, github.UnavailableReason) {
	current := observed.Setting(desired.Unit)
	value, ok := current.Value()
	switch {
	case !ok:
		return desiredStateUnavailable, current.UnavailableReason()
	case !value.Equal(desired.Value):
		return desiredStateMismatch, 0
	default:
		return desiredStateMatches, 0
	}
}
