package apply

import (
	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// repositoryRecord は、一つの対象リポジトリについて、承認した論理的な設定変更の記録を実行順に並べたもの。
// 変更不要の対象リポジトリは記録を持たない。
type repositoryRecord struct {
	target  repository.Reference
	changes []changeRecord
}

// repositoryResult は、対象リポジトリごとに変更結果の並びを集約した区別。変更結果ではない。
// 変更結果は論理的な設定変更にだけ定まり、変更不要の対象リポジトリは変更結果を持たないため。
type repositoryResult int

const (
	repositoryNoChange repositoryResult = iota
	repositoryCompleted
	repositoryNotCompleted
	// repositoryStopped は、先行する対象リポジトリの異常によって、どの論理的な設定変更にも要求を送らなかったこと。
	repositoryStopped
)

// result は、対象リポジトリの区別を導く。
// 実行を始めた対象リポジトリは、最初の論理的な設定変更の最初の更新操作の変更前提を必ず確かめる (確認不能を含む) ため、
// 停止は最初の記録が変更前提を確かめていないことで分かる。停止の理由を別のフィールドに持つと、要求を送ったのに停止、のような矛盾した値を作れる。
func (r repositoryRecord) result() repositoryResult {
	switch {
	case len(r.changes) == 0:
		return repositoryNoChange
	case allCompleted(r.changes):
		return repositoryCompleted
	case len(r.changes[0].operations) == 0:
		return repositoryStopped
	default:
		return repositoryNotCompleted
	}
}

func allCompleted(records []changeRecord) bool {
	for _, record := range records {
		if record.result() != completed {
			return false
		}
	}
	return true
}

// changeRecord は、承認した論理的な設定変更を実行して得た、変更結果を導く事実。
// change だけを持つ値は、先行する異常によって何も確かめずに止めた論理的な設定変更を表す。
type changeRecord struct {
	change changeplan.LogicalChange
	// operations は、変更前提を確かめた更新操作の記録を、change.Operations の先頭から並べたもの。
	// 変更前提を確かめなかった更新操作の記録は持たない。
	operations   []operationRecord
	confirmation desiredStateConfirmation
	// desiredUnavailableReason は confirmation が desiredStateUnavailable のときだけ意味を持つ。
	desiredUnavailableReason github.UnavailableReason
	unexpectedOutsideChanges []unexpectedOutsideChange
	// cause は、変更前提の確認、更新操作、更新後の観測とその前の待機のうち、失敗した操作の error。
	cause error
}

// operationRecord は、論理的な設定変更を構成する一つの更新操作について、変更前提の確認結果と更新操作の結果。
type operationRecord struct {
	precondition preconditionCheck
	// changedSetting は precondition が preconditionCurrentValueChanged のときだけ意味を持つ、変わっていた関係する現在値の設定。
	// 副作用として提示した設定や、先行する更新操作が変えた設定が変わっていた場合は、この更新操作が変える設定と異なる。
	changedSetting setting.Key
	// update は started が真のときだけ意味を持つ。偽のときのゼロ値は、開始しなかった更新操作の結果ではない。
	update github.UpdateOutcome
}

// started は、この更新操作を開始したかを返す。
// 変更前提が成立したときに限り、必ず更新操作を一度開始するため、前提の成立と同値になる。
func (o operationRecord) started() bool {
	return o.precondition == preconditionHolds
}

// changeResult は、論理的な設定変更の変更結果。
type changeResult int

const (
	notAttempted changeResult = iota
	failed
	completed
)

// operation は、i 番目の更新操作の記録を返す。変更前提を確かめなかった更新操作は、ゼロ値 (preconditionNotChecked) を返す。
func (r changeRecord) operation(i int) operationRecord {
	if i < len(r.operations) {
		return r.operations[i]
	}
	return operationRecord{}
}

// updateStarted は、論理的な設定変更の更新操作を一つでも開始したかを返す。
// 更新操作は先頭から順に開始するため、先頭の更新操作を開始したかと同値になる。
func (r changeRecord) updateStarted() bool {
	return r.operation(0).started()
}

// updatesSucceeded は、論理的な設定変更のすべての更新操作が成功したかを返す。
// 開始しなかった更新操作が残っていれば、成功していない。
func (r changeRecord) updatesSucceeded() bool {
	if len(r.operations) != len(r.change.Operations) {
		return false
	}
	for _, operation := range r.operations {
		if operation.update != github.UpdateSucceeded {
			return false
		}
	}
	return true
}

// mayHaveChangedState は、GitHub の状態を変えた可能性のある更新操作 (成功か成否不明) があるかを返す。
func (r changeRecord) mayHaveChangedState() bool {
	for _, operation := range r.operations {
		if operation.started() && operation.update != github.UpdateRejected {
			return true
		}
	}
	return false
}

// hasUnknownOutcome は、開始した更新操作に成否不明があるかを返す。
func (r changeRecord) hasUnknownOutcome() bool {
	for _, operation := range r.operations {
		if operation.started() && operation.update == github.UpdateUnknown {
			return true
		}
	}
	return false
}

// result は、変更結果を導く。
func (r changeRecord) result() changeResult {
	switch {
	case !r.updateStarted():
		return notAttempted
	case r.updatesSucceeded() && r.confirmation == desiredStateMatches && len(r.unexpectedOutsideChanges) == 0:
		return completed
	default:
		return failed
	}
}
