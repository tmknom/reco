package apply

import (
	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

// desiredStateConfirmation は、更新後の観測で期待状態を確かめた結果。
// desiredStateUnconfirmed、desiredStateIdentityMismatch、desiredStateUnavailable は、どれも確認失敗を理由で分けたもの。
// desiredStateUnavailable は、期待値と比べる設定が取得条件により観測不能だったことを表す。
type desiredStateConfirmation int

const (
	desiredStateNotChecked desiredStateConfirmation = iota
	desiredStateMatches
	desiredStateMismatch
	desiredStateUnconfirmed
	desiredStateIdentityMismatch
	desiredStateUnavailable
)

// confirmDesiredState は、更新後の観測が、すべての更新操作の期待値からなる期待状態と一致するかを判定する。
// 観測できた値に期待値と異なるものがあれば、ほかの設定が観測不能でも不一致にする。
// そうでなく期待値と比べる設定に観測不能があれば、値を確認できていないため確認失敗にし、最初に観測不能だった設定の理由を返す。
func confirmDesiredState(approvedID repository.ID, change changeplan.LogicalChange, observed github.ObservationResult) (desiredStateConfirmation, github.UnavailableReason) {
	if observed.RepositoryID != approvedID {
		return desiredStateIdentityMismatch, 0
	}
	var reason github.UnavailableReason
	for _, operation := range change.Operations {
		current := observed.Setting(operation.Key)
		value, ok := current.Value()
		switch {
		case !ok:
			if reason == 0 {
				reason = current.UnavailableReason()
			}
		case !value.Equal(operation.Desired):
			return desiredStateMismatch, 0
		}
	}
	if reason != 0 {
		return desiredStateUnavailable, reason
	}
	return desiredStateMatches, 0
}
