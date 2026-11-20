package apply

import (
	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
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
		confirmation, operationReason := confirmOperation(operation, observed)
		if confirmation == desiredStateMismatch {
			return desiredStateMismatch, 0
		}
		if confirmation == desiredStateUnavailable && reason == 0 {
			reason = operationReason
		}
	}
	if reason != 0 {
		return desiredStateUnavailable, reason
	}
	return desiredStateMatches, 0
}

// confirmOperation は、一つの更新操作の期待値を observed と比べ、一致、不一致、理由を伴う観測不能のいずれかを返す。
// Environment の作成の期待値は、存在することと、方式を送ったときはその方式である（ドメインモデルの関連リソースと存在）。
func confirmOperation(operation changeplan.UpdateOperation, observed github.ObservationResult) (desiredStateConfirmation, github.UnavailableReason) {
	var current github.SettingObservation[setting.Value]
	if operation.CreatesEnvironment() {
		environment := observed.Environments.Lookup(operation.Unit.Environment)
		_, exists := environment.Existing()
		switch {
		case environment.Absent():
			return desiredStateMismatch, 0
		case !exists:
			return desiredStateUnavailable, environment.UnavailableReason()
		case !operation.SetsDeploymentBranchPolicy():
			return desiredStateMatches, 0
		}
		current = environment.DeploymentBranchPolicy()
	} else {
		current = observed.Setting(operation.Unit)
	}
	value, ok := current.Value()
	switch {
	case !ok:
		return desiredStateUnavailable, current.UnavailableReason()
	case !value.Equal(operation.Desired):
		return desiredStateMismatch, 0
	default:
		return desiredStateMatches, 0
	}
}
