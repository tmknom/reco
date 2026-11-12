package apply

import (
	"context"
	"errors"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// executeChange は、承認した論理的な設定変更を実行し、変更結果を導く事実を返す。
// 更新直前に観測して変更前提を確かめ、成立したときだけ更新要求を一度だけ送る。再送はしない。
func executeChange(ctx context.Context, client *github.Client, p changeplan.Plan, change changeplan.LogicalChange) changeRecord {
	var record changeRecord
	before, err := client.ObserveRepository(ctx, p.Target)
	if err != nil {
		record.cause = err
		return record
	}
	record.precondition = checkPrecondition(p.RepositoryID, change, before)
	if record.precondition != preconditionHolds {
		return record
	}

	record.update, record.cause = client.UpdateAllowAutoMerge(ctx, p.Target, change.Desired)
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
		record.unexpectedUnmanagedChanges = unexpectedUnmanagedChanges(before.General, after.General)
	}
	return record
}

// checkPrecondition は、承認した変更を更新直前の観測と照らし、更新してよいかを判定する。
// 別のリポジトリの値を承認時の値と比べても意味がないため、同一性を現在値より先に確かめる。
func checkPrecondition(approvedID repository.ID, change changeplan.LogicalChange, observed github.ObservationResult) preconditionCheck {
	if observed.RepositoryID != approvedID {
		return preconditionIdentityMismatch
	}
	if change.ObservedIn(observed.General) != change.Observed {
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

// unexpectedUnmanagedChanges は、更新操作の影響を受け得る未管理設定について、更新直前と更新後の観測値の差を返す。
func unexpectedUnmanagedChanges(before, after github.General) []unexpectedUnmanagedChange {
	var changes []unexpectedUnmanagedChange
	if before.DeleteBranchOnMerge != after.DeleteBranchOnMerge {
		changes = append(changes, unexpectedUnmanagedChange{
			Key:    setting.DeleteBranchOnMerge,
			Before: before.DeleteBranchOnMerge,
			After:  after.DeleteBranchOnMerge,
		})
	}
	return changes
}
