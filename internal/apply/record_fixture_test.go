package apply

import (
	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
)

// completedRecord は、change のすべての更新操作が成功し、期待状態との一致を確認できた記録。
func completedRecord(change changeplan.LogicalChange) changeRecord {
	operations := make([]operationRecord, len(change.Operations))
	for i := range operations {
		operations[i] = operationRecord{precondition: preconditionHolds, update: github.UpdateSucceeded}
	}
	return changeRecord{change: change, operations: operations, confirmation: desiredStateMatches}
}

// started は、変更前提が成立して開始し、update に終わった更新操作の記録。
func started(update github.UpdateOutcome) operationRecord {
	return operationRecord{precondition: preconditionHolds, update: update}
}
