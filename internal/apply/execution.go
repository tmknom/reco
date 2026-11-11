package apply

import (
	"context"
	"errors"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
)

// settingDeleteBranchOnMerge は、観測するが管理しない設定の YAML パス。
const settingDeleteBranchOnMerge = "general.delete_branch_on_merge"

// executeChange は、承認した変更を実行し、その結果を返す。
// 更新直前に再取得して前提を確かめ、成立したときだけ更新要求を一度だけ送る。再送はしない。
func executeChange(ctx context.Context, client *github.Client, p changeplan.Plan, change changeplan.SettingPlan) changeReport {
	var report changeReport
	before, err := client.FetchRepository(ctx, p.Target)
	if err != nil {
		report.cause = err
		return report
	}
	report.precondition = checkPrecondition(p.RepositoryID, change, before)
	if report.precondition != preconditionHolds {
		return report
	}

	updateErr := client.UpdateAllowAutoMerge(ctx, p.Target, change.Desired)
	report.update = updateOutcomeOf(updateErr)
	report.cause = updateErr
	if report.update == updateFailed {
		return report
	}

	// 更新が成功したときは確認のため、成否不明のときは診断のために再取得する。
	after, err := client.FetchRepository(ctx, p.Target)
	if err != nil {
		report.confirmation = desiredStateUnconfirmed
		report.cause = errors.Join(report.cause, err)
		return report
	}
	report.confirmation = confirmDesiredState(p.RepositoryID, change, after)
	// ID が変わった観測は別のリポジトリの値のため、更新直前の値と比べない。
	if report.confirmation != desiredStateRepositoryChanged {
		report.unmanagedChanges = unmanagedChanges(before.General, after.General)
	}
	return report
}

// checkPrecondition は、承認した変更を更新直前の観測と照らし、更新してよいかを判定する。
// 別のリポジトリの値を承認時の値と比べても意味がないため、同一性を現在値より先に確かめる。
func checkPrecondition(approvedID int64, change changeplan.SettingPlan, observed github.Repository) preconditionCheck {
	if observed.ID != approvedID {
		return preconditionIdentityMismatch
	}
	if change.CurrentIn(observed.General) != change.Current {
		return preconditionCurrentValueChanged
	}
	return preconditionHolds
}

// updateOutcomeOf は、更新操作の error から更新の成否を分類する。
// 成否不明は、ほかの失敗と重なっていても成否不明として扱う。
func updateOutcomeOf(err error) updateOutcome {
	switch {
	case err == nil:
		return updateSucceeded
	case errors.Is(err, github.ErrUpdateOutcomeUnknown):
		return updateOutcomeUnknown
	default:
		return updateFailed
	}
}

// confirmDesiredState は、更新後の観測が期待状態と一致するかを判定する。
func confirmDesiredState(approvedID int64, change changeplan.SettingPlan, observed github.Repository) desiredStateConfirmation {
	if observed.ID != approvedID {
		return desiredStateRepositoryChanged
	}
	if change.Reconcile(observed.General) == changeplan.Match {
		return desiredStateMatches
	}
	return desiredStateMismatch
}

// unmanagedChanges は、管理しない設定の更新前後の差を返す。
func unmanagedChanges(before, after github.General) []unmanagedChange {
	var changes []unmanagedChange
	if before.DeleteBranchOnMerge != after.DeleteBranchOnMerge {
		changes = append(changes, unmanagedChange{
			Setting: settingDeleteBranchOnMerge,
			Before:  before.DeleteBranchOnMerge,
			After:   after.DeleteBranchOnMerge,
		})
	}
	return changes
}
