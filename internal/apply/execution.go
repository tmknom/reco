package apply

import (
	"context"
	"errors"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// executeChanges は、承認した論理的な設定変更を順に実行する。
// INV-005 のため、完了以外の変更結果が出たら、残りには要求を一つも送らず、
// change だけを持つ記録 (先行する異常による停止) を返す。戻り値は changes と同じ長さ、同じ並びになる。
func executeChanges(ctx context.Context, client *github.Client, p changeplan.Plan, changes []changeplan.LogicalChange) []changeRecord {
	records := make([]changeRecord, len(changes))
	stopped := false
	for i, change := range changes {
		if stopped {
			records[i] = changeRecord{change: change}
			continue
		}
		records[i] = executeChange(ctx, client, p, change)
		stopped = records[i].result() != completed
	}
	return records
}

// executeChange は、承認した論理的な設定変更を実行し、変更結果を導く事実を返す。
// 更新直前に観測して変更前提を確かめ、成立したときだけ更新要求を一度だけ送る。再送はしない。
func executeChange(ctx context.Context, client *github.Client, p changeplan.Plan, change changeplan.LogicalChange) changeRecord {
	update, ok := updateOperation(change.Key)
	if !ok {
		// executableChanges が承認を求める前に確かめた不変条件の破れのため、要求を送る前に止める。
		panic("apply: no update operation for " + string(change.Key))
	}
	record := changeRecord{change: change}
	before, err := client.ObserveRepository(ctx, p.Target)
	if err != nil {
		record.precondition = preconditionUnconfirmed
		record.cause = err
		return record
	}
	record.precondition = checkPrecondition(p.RepositoryID, change, before)
	if record.precondition != preconditionHolds {
		return record
	}

	record.update, record.cause = update(client, ctx, p.Target, change.Desired)
	if record.update == github.UpdateRejected {
		return record
	}

	// 更新が成功したときは確認のため、成否不明のときは診断のために観測する。
	after, err := client.ObserveRepository(ctx, p.Target)
	if err != nil {
		record.confirmation = desiredStateUnconfirmed
		record.cause = errors.Join(record.cause, err)
		return record
	}
	record.confirmation = confirmDesiredState(p.RepositoryID, change, after)
	// ID が変わった観測は別のリポジトリの値のため、更新直前の値と比べない。
	if record.confirmation != desiredStateRepositoryChanged {
		record.unexpectedOutsideChanges = unexpectedOutsideChanges(change, before.General, after.General)
	}
	return record
}

// updateOperation は、key の論理的な設定変更を実現する更新操作を返す。なければ ok は偽になる。
// 承認前の判定と実行時の選択が同じ定義を使い、片方だけ設定を足し忘れることがないよう、一つの関数にする。
// 承認前の判定が client を要しないよう、メソッド式で返す。
func updateOperation(key setting.Key) (update func(*github.Client, context.Context, repository.Reference, bool) (github.UpdateOutcome, error), ok bool) {
	switch key {
	case setting.DeleteBranchOnMerge:
		return (*github.Client).UpdateDeleteBranchOnMerge, true
	case setting.AllowAutoMerge:
		return (*github.Client).UpdateAllowAutoMerge, true
	default:
		return nil, false
	}
}

// checkPrecondition は、承認した変更を更新直前の観測と照らし、更新してよいかを判定する。
// 別のリポジトリの値を承認時の値と比べても意味がないため、同一性を現在値より先に確かめる。
func checkPrecondition(approvedID repository.ID, change changeplan.LogicalChange, observed github.ObservationResult) preconditionCheck {
	if observed.RepositoryID != approvedID {
		return preconditionIdentityMismatch
	}
	if changeplan.ObservedValue(change.Key, observed.General) != change.Observed {
		return preconditionCurrentValueChanged
	}
	return preconditionHolds
}

// confirmDesiredState は、更新後の観測が期待状態と一致するかを判定する。
func confirmDesiredState(approvedID repository.ID, change changeplan.LogicalChange, observed github.ObservationResult) desiredStateConfirmation {
	if observed.RepositoryID != approvedID {
		return desiredStateRepositoryChanged
	}
	if change.Reconcile(observed.General) == changeplan.Match {
		return desiredStateMatches
	}
	return desiredStateMismatch
}

// unexpectedOutsideChanges は、change と同じ更新エンドポイントで観測する設定のうち change が変えないものについて、
// 更新直前と更新後の観測値の差を返す。
func unexpectedOutsideChanges(change changeplan.LogicalChange, before, after github.General) []unexpectedOutsideChange {
	var changes []unexpectedOutsideChange
	for _, key := range setting.GeneralKeys() {
		if key == change.Key {
			continue
		}
		b, a := changeplan.ObservedValue(key, before), changeplan.ObservedValue(key, after)
		if b != a {
			changes = append(changes, unexpectedOutsideChange{Key: key, Before: b, After: a})
		}
	}
	return changes
}
