package apply

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// executeChanges は、承認した論理的な設定変更を順に実行する。
// INV-005 のため、完了以外の変更結果が出たら、残りには要求を一つも送らず、
// change だけを持つ記録 (先行する異常による停止) を返す。戻り値は changes と同じ長さ、同じ並びになる。
func executeChanges(ctx context.Context, client *github.Client, p changeplan.Plan, changes []changeplan.LogicalChange, after func(time.Duration) <-chan time.Time) []changeRecord {
	records := make([]changeRecord, len(changes))
	stopped := false
	for i, change := range changes {
		if stopped {
			records[i] = changeRecord{change: change}
			continue
		}
		records[i] = executeChange(ctx, client, p, change, after)
		stopped = records[i].result() != completed
	}
	return records
}

// executeChange は、承認した論理的な設定変更を実行し、変更結果を導く事実を返す。
// 更新直前に観測して変更前提を確かめ、成立したときだけ更新要求を一度だけ送る。再送はしない。
func executeChange(ctx context.Context, client *github.Client, p changeplan.Plan, change changeplan.LogicalChange, after func(time.Duration) <-chan time.Time) changeRecord {
	update, ok := updateOperationOf(change.Key)
	if !ok {
		// executableChanges が承認を求める前に確かめた不変条件の破れのため、要求を送る前に止める。
		panic("apply: no update operation for " + string(change.Key))
	}
	record := changeRecord{change: change}
	before, err := client.ObserveRepository(ctx, p.Target, change.Key.Area())
	if err != nil {
		record.precondition = preconditionUnconfirmed
		record.cause = err
		return record
	}
	record.precondition = checkPrecondition(p.RepositoryID, change, before)
	if record.precondition != preconditionHolds {
		return record
	}

	record.update, record.cause = update.send(ctx, client, p.Target, change.Desired)
	if record.update == github.UpdateRejected {
		return record
	}

	// 更新が成功したときは確認のため、成否不明のときは診断のために観測する。
	// 診断の観測は、成否が判明しない以上、観測し直しても判断を変えないため一度だけにする。
	var retryDelays []time.Duration
	if record.update == github.UpdateSucceeded {
		retryDelays = confirmationRetryDelays[:]
	}
	observed, confirmation, err := observeAfterUpdate(ctx, client, p, change, retryDelays, after)
	record.confirmation = confirmation
	if err != nil {
		record.cause = errors.Join(record.cause, err)
		return record
	}
	// ID が変わった観測は別のリポジトリの値のため、更新直前の値と比べない。
	if record.confirmation != desiredStateIdentityMismatch {
		record.unexpectedOutsideChanges = unexpectedOutsideChanges(update.endpoint, change, before, observed)
	}
	return record
}

// confirmationRetryDelays は、期待状態と一致しないときに観測し直す前に待つ時間を、観測し直す順に並べたもの。
var confirmationRetryDelays = [...]time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

// observeAfterUpdate は、更新後の観測を行い、期待状態の確認結果と、それを与えた観測を返す。
// GitHub は正常応答のあとすぐには更新を反映しないことがあるため、期待状態と一致しないあいだは、
// retryDelays の時間ずつ待って、retryDelays を使い切るまで観測し直す。
func observeAfterUpdate(ctx context.Context, client *github.Client, p changeplan.Plan, change changeplan.LogicalChange, retryDelays []time.Duration, after func(time.Duration) <-chan time.Time) (github.ObservationResult, desiredStateConfirmation, error) {
	for i := 0; ; i++ {
		observed, err := client.ObserveRepository(ctx, p.Target, change.Key.Area())
		if err != nil {
			return github.ObservationResult{}, desiredStateUnconfirmed, err
		}
		confirmation := confirmDesiredState(p.RepositoryID, change, observed)
		if confirmation != desiredStateMismatch || i == len(retryDelays) {
			return observed, confirmation, nil
		}
		if err := waitFor(ctx, after, retryDelays[i]); err != nil {
			return github.ObservationResult{}, desiredStateUnconfirmed, fmt.Errorf("wait before reading %s again after the update: %w", p.Target, err)
		}
	}
}

func waitFor(ctx context.Context, after func(time.Duration) <-chan time.Time, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-after(d):
		return nil
	}
}

// observedValue は、key の設定の観測値を返す。
// 更新操作を持つ設定と、その更新エンドポイントで観測する設定は、どれも取得条件を持たず常に観測値になるため、
// 観測不能は不変条件の破れとして panic する。
func observedValue(o github.ObservationResult, key setting.Key) setting.Value {
	v, ok := o.Value(key).Value()
	if !ok {
		panic("apply: no observed value of " + string(key))
	}
	return v
}

// checkPrecondition は、承認した変更を更新直前の観測と照らし、更新してよいかを判定する。
// 別のリポジトリの値を承認時の値と比べても意味がないため、同一性を現在値より先に確かめる。
func checkPrecondition(approvedID repository.ID, change changeplan.LogicalChange, observed github.ObservationResult) preconditionCheck {
	if observed.RepositoryID != approvedID {
		return preconditionIdentityMismatch
	}
	if !observedValue(observed, change.Key).Equal(change.Observed) {
		return preconditionCurrentValueChanged
	}
	return preconditionHolds
}

// confirmDesiredState は、更新後の観測が期待状態と一致するかを判定する。
func confirmDesiredState(approvedID repository.ID, change changeplan.LogicalChange, observed github.ObservationResult) desiredStateConfirmation {
	if observed.RepositoryID != approvedID {
		return desiredStateIdentityMismatch
	}
	if observedValue(observed, change.Key).Equal(change.Desired) {
		return desiredStateMatches
	}
	return desiredStateMismatch
}

// unexpectedOutsideChanges は、change の更新操作が更新要求を送る endpoint で観測する設定のうち change が変えないものについて、
// 更新直前と更新後の観測値の差を返す。
// 観測は設定の領域の単位で読むが、同じ設定の領域でも別の更新エンドポイントの設定は更新操作の影響を受けないため比べない。
func unexpectedOutsideChanges(endpoint updateEndpoint, change changeplan.LogicalChange, before, after github.ObservationResult) []unexpectedOutsideChange {
	var changes []unexpectedOutsideChange
	for _, key := range endpoint.observedSettings() {
		if key == change.Key {
			continue
		}
		b, a := observedValue(before, key), observedValue(after, key)
		if !b.Equal(a) {
			changes = append(changes, unexpectedOutsideChange{Key: key, Before: b, After: a})
		}
	}
	return changes
}
