package apply

import (
	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// changeRecord は、承認した論理的な設定変更を実行して得た、変更結果を導く事実。
// change だけを持つ値は、先行する異常によって何も確かめずに止めた論理的な設定変更を表す。
type changeRecord struct {
	change       changeplan.LogicalChange
	precondition preconditionCheck
	// update は updateStarted が真のときだけ意味を持つ。偽のときのゼロ値は、開始しなかった更新操作の結果ではない。
	update                   github.UpdateOutcome
	confirmation             desiredStateConfirmation
	unexpectedOutsideChanges []unexpectedOutsideChange
	// cause は、変更前提の確認、更新操作、更新後の観測のうち、失敗した操作の error。
	cause error
}

// changeResult は、論理的な設定変更の変更結果。
type changeResult int

const (
	notAttempted changeResult = iota
	failed
	completed
)

// preconditionCheck は、更新直前に変更前提を確かめた結果。
// 未実行の理由はこの値から導く。preconditionNotChecked は先行する異常による停止、preconditionHolds 以外の残りの値は変更前提の不成立に当たる。
// 理由を別のフィールドで持つと、前提は成立したのに理由は停止、のような矛盾した値を作れるため。
type preconditionCheck int

const (
	preconditionNotChecked preconditionCheck = iota
	preconditionUnconfirmed
	preconditionHolds
	preconditionIdentityMismatch
	preconditionCurrentValueChanged
)

// desiredStateConfirmation は、更新後の観測で期待状態を確かめた結果。
// desiredStateUnconfirmed と desiredStateRepositoryChanged は、どちらも確認失敗を理由で分けたもの。
type desiredStateConfirmation int

const (
	desiredStateNotChecked desiredStateConfirmation = iota
	desiredStateMatches
	desiredStateMismatch
	desiredStateUnconfirmed
	desiredStateRepositoryChanged
)

// unexpectedOutsideChange は、変更対象外の設定の予期しない変化。
type unexpectedOutsideChange struct {
	Key           setting.Key
	Before, After bool
}

// updateStarted は、更新操作を開始したかを返す。
// executeChange は変更前提が成立したときに限り、必ず更新操作を一度開始するため、前提の成立と同値になる。
func (r changeRecord) updateStarted() bool {
	return r.precondition == preconditionHolds
}

// result は、変更結果を導く。
func (r changeRecord) result() changeResult {
	switch {
	case !r.updateStarted():
		return notAttempted
	case r.update == github.UpdateSucceeded && r.confirmation == desiredStateMatches && len(r.unexpectedOutsideChanges) == 0:
		return completed
	default:
		return failed
	}
}
