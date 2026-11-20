package apply

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// updateIntervalDelay は、更新要求の間隔として待つ時間。観測し直しの待ち時間 (reobservationDelays) とは理由の違う待機のため、別に持つ。
const updateIntervalDelay = 1 * time.Second

// updateInterval は、一回の実行で更新操作を開始したかを持ち、更新要求の間隔を待つ。
// 実行の中の2つ目以降の更新操作の前だけで待つため、論理的な設定変更と対象リポジトリをまたいで同じ値を使う。
type updateInterval struct {
	updateStarted bool
}

func (u *updateInterval) wait(ctx context.Context, after func(time.Duration) <-chan time.Time) error {
	if !u.updateStarted {
		return nil
	}
	return waitFor(ctx, after, updateIntervalDelay)
}

// executeRepositories は、承認した論理的な設定変更を、対象リポジトリの宣言順に実行する。
// INV-005 は一回の実行の全体を範囲にするため、完了以外の変更結果が出たら、残りの対象リポジトリにも要求を一つも送らず、
// change だけを持つ記録 (先行する異常による停止) を並べる。変更不要の対象リポジトリは、停止のあとでも記録を持たないまま並ぶ。
// 戻り値は repositories と同じ長さ、同じ並びになる。
func executeRepositories(ctx context.Context, client *github.Client, repositories []repositoryChanges, after func(time.Duration) <-chan time.Time) []repositoryRecord {
	interval := &updateInterval{}
	records := make([]repositoryRecord, len(repositories))
	stopped := false
	for i, r := range repositories {
		records[i].target = r.plan.Target
		if stopped {
			for _, change := range r.changes {
				records[i].changes = append(records[i].changes, changeRecord{change: change})
			}
			continue
		}
		records[i].changes = executeChanges(ctx, client, r.plan, r.changes, after, interval)
		stopped = !allCompleted(records[i].changes)
	}
	return records
}

// executeChanges は、承認した論理的な設定変更を順に実行する。
// INV-005 のため、完了以外の変更結果が出たら、残りには要求を一つも送らず、
// change だけを持つ記録 (先行する異常による停止) を返す。戻り値は changes と同じ長さ、同じ並びになる。
func executeChanges(ctx context.Context, client *github.Client, r changeplan.RepositoryPlan, changes []changeplan.LogicalChange, after func(time.Duration) <-chan time.Time, interval *updateInterval) []changeRecord {
	records := make([]changeRecord, len(changes))
	stopped := false
	for i, change := range changes {
		if stopped {
			records[i] = changeRecord{change: change}
			continue
		}
		records[i] = executeChange(ctx, client, r, change, after, interval)
		stopped = records[i].result() != completed
	}
	return records
}

// executeChange は、承認した論理的な設定変更を実行し、変更結果を導く事実を返す。
// 更新操作を順に、直前に観測して変更前提を確かめ、成立したときだけ更新要求を一度だけ送る。再送はしない。
// INV-005 のため、先行する更新操作が成功したときだけ後続の更新操作の変更前提を確かめる。
func executeChange(ctx context.Context, client *github.Client, r changeplan.RepositoryPlan, change changeplan.LogicalChange, after func(time.Duration) <-chan time.Time, interval *updateInterval) changeRecord {
	record := changeRecord{change: change}
	areas := observationAreas(change)
	// before[i] は change.Operations[i] の直前の観測。変更対象外の設定の更新前の値を与える。
	var before []github.ObservationResult
	for i, operation := range change.Operations {
		observed, checked, err := observeBeforeUpdate(ctx, client, r, change, i, areas, after, interval)
		record.operations = append(record.operations, checked)
		if err != nil {
			record.cause = err
		}
		if !checked.started() {
			break
		}
		before = append(before, observed)
		// 更新要求の結果によらず、送ったあとの次の更新操作は間隔を待つ。
		interval.updateStarted = true
		var updateErr error
		record.operations[i].update, updateErr = sendUpdate(ctx, client, r.Target, operation, observed)
		record.cause = errors.Join(record.cause, updateErr)
		if record.operations[i].update != github.UpdateSucceeded {
			break
		}
	}

	// 更新後の観測は、GitHub の状態を変えた可能性のある更新操作 (成功か成否不明) があるときに行う。
	// 先行する更新操作が成功していれば、後続を開始しなかったか拒否されたときも、先行する更新操作の更新エンドポイントの設定を比べるために観測する。
	if !record.mayHaveChangedState() {
		return record
	}
	// 観測し直すのは、すべての更新操作が成功したときだけにする。
	// それ以外は完了を確認できないことが決まっており、観測し直しても判断を変えないため、一度だけ観測する。
	var retryDelays []time.Duration
	if record.updatesSucceeded() {
		retryDelays = reobservationDelays[:]
	}
	observed, confirmation, reason, err := observeAfterUpdate(ctx, client, r, change, areas, retryDelays, after)
	record.confirmation = confirmation
	record.desiredUnavailableReason = reason
	if err != nil {
		record.cause = errors.Join(record.cause, err)
		return record
	}
	// ID が変わった観測は別のリポジトリの値のため、更新直前の値と比べない。
	if record.confirmation != desiredStateIdentityMismatch {
		record.unexpectedOutsideChanges = unexpectedOutsideChanges(change, before, observed)
	}
	return record
}

// observeBeforeUpdate は、change の index 番目の更新操作の直前に観測し、変更前提を確かめた記録と、それを与えた観測を返す。
// 更新要求の間隔を観測より前に待つ理由は、アーキテクチャ設計の更新要求の間隔が定める。
// 先行する更新操作の正常応答のあと、GitHub がすぐには更新を反映しないことがあるため、
// 観測が先行する更新操作の直前の状態のままのあいだに限り、reobservationDelays の時間ずつ待って観測し直す。
// その状態は後続の更新操作の基準 (予定どおりの変化) を満たさないため、観測し直すのは変更前提が成立しないときだけになる。
// 観測し直すのは読み取りだけで、返す確認結果は常に最後の観測に対する checkPrecondition の結果である。
func observeBeforeUpdate(ctx context.Context, client *github.Client, r changeplan.RepositoryPlan, change changeplan.LogicalChange, index int, areas []setting.Area, after func(time.Duration) <-chan time.Time, interval *updateInterval) (github.ObservationResult, operationRecord, error) {
	operation := change.Operations[index]
	if err := interval.wait(ctx, after); err != nil {
		return github.ObservationResult{}, operationRecord{precondition: preconditionUnconfirmed}, fmt.Errorf("wait for the interval between update requests before reading %s: %w", r.Target, err)
	}
	for i := 0; ; i++ {
		observed, err := client.ObserveRepository(ctx, r.Target, areas...)
		if err != nil {
			return github.ObservationResult{}, operationRecord{precondition: preconditionUnconfirmed}, err
		}
		check, changed := checkPrecondition(r.RepositoryID, operation, observed)
		if !precedingUpdateNotReflected(r.RepositoryID, change.Operations[:index], observed) || i == len(reobservationDelays) {
			return observed, operationRecord{precondition: check, changedSetting: changed}, nil
		}
		if err := waitFor(ctx, after, reobservationDelays[i]); err != nil {
			return github.ObservationResult{}, operationRecord{precondition: preconditionUnconfirmed}, fmt.Errorf("wait before reading %s again before the next update: %w", r.Target, err)
		}
	}
}

// sendUpdate は、変更前提を確かめた更新操作の更新要求を一度だけ送る。observed は、その変更前提を確かめた更新直前の観測である。
func sendUpdate(ctx context.Context, client *github.Client, target repository.Reference, operation changeplan.UpdateOperation, observed github.ObservationResult) (github.UpdateOutcome, error) {
	if !operation.TargetsEnvironment() {
		return client.Update(ctx, target, operation.Unit.Key, operation.Desired)
	}
	destination := environmentDestination(operation.Unit.Environment, observed)
	if !operation.SetsDeploymentBranchPolicy() {
		return client.CreateEnvironment(ctx, target, destination)
	}
	return client.UpdateEnvironmentDeploymentBranchPolicy(ctx, target, destination, operation.Desired.DeploymentBranchPolicy().Mode)
}

// environmentDestination は、宣言の名前 name の Environment の更新要求の宛先の綴りを返す（アーキテクチャ設計の未管理設定を保全する更新方式）。
func environmentDestination(name setting.EnvironmentName, observed github.ObservationResult) string {
	if existing, ok := observed.Environments.Lookup(name).Existing(); ok {
		return existing.Name
	}
	return name.String()
}

// reobservationDelays は、GitHub の反映の遅れを待って観測し直す前に待つ時間を、観測し直す順に並べたもの。
var reobservationDelays = [...]time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}

// observationAreas は、change の更新直前と更新後の観測で読む設定の領域を返す。
// 変更前提で比べる設定と期待値と比べる設定の領域を読まないと、観測結果にその値がないため、更新操作の設定と基準の設定から導く。
func observationAreas(change changeplan.LogicalChange) []setting.Area {
	var areas []setting.Area
	for _, operation := range change.Operations {
		areas = appendArea(areas, operation.Unit.Key.Area())
		for _, relevant := range operation.RelevantSettings {
			areas = appendArea(areas, relevant.Unit.Key.Area())
		}
	}
	return areas
}

func appendArea(areas []setting.Area, area setting.Area) []setting.Area {
	if slices.Contains(areas, area) {
		return areas
	}
	return append(areas, area)
}

// observeAfterUpdate は、更新後の観測を行い、期待状態の確認結果とその観測不能の理由、それを与えた観測を返す。
// GitHub は正常応答のあとすぐには更新を反映しないことがあるため、期待状態と一致しないあいだと、期待値と比べる設定が観測不能のあいだは、
// retryDelays の時間ずつ待って、retryDelays を使い切るまで観測し直す。
func observeAfterUpdate(ctx context.Context, client *github.Client, r changeplan.RepositoryPlan, change changeplan.LogicalChange, areas []setting.Area, retryDelays []time.Duration, after func(time.Duration) <-chan time.Time) (github.ObservationResult, desiredStateConfirmation, github.UnavailableReason, error) {
	for i := 0; ; i++ {
		observed, err := client.ObserveRepository(ctx, r.Target, areas...)
		if err != nil {
			return github.ObservationResult{}, desiredStateUnconfirmed, 0, err
		}
		confirmation, reason := confirmDesiredState(r.RepositoryID, change, observed)
		if (confirmation != desiredStateMismatch && confirmation != desiredStateUnavailable) || i == len(retryDelays) {
			return observed, confirmation, reason, nil
		}
		if err := waitFor(ctx, after, retryDelays[i]); err != nil {
			return github.ObservationResult{}, desiredStateUnconfirmed, 0, fmt.Errorf("wait before reading %s again after the update: %w", r.Target, err)
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
