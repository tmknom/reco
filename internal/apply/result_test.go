package apply

import (
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestChangeRecord_MayHaveChangedState は、更新後に観測するかを決める、GitHub の状態を変えた可能性のある更新操作の規則を確かめる。
//
//trace:checks FR-012
func TestChangeRecord_MayHaveChangedState(t *testing.T) {
	succeeded := operationRecord{precondition: preconditionHolds, update: github.UpdateSucceeded}
	rejected := operationRecord{precondition: preconditionHolds, update: github.UpdateRejected}
	cases := []struct {
		name       string
		operations []operationRecord
		want       bool
	}{
		{name: "no record has not changed the state", want: false},
		{
			// 開始しなかった記録の update はゼロ値の UpdateUnknown で、成否不明ではない
			name:       "first precondition not holding has not changed the state",
			operations: []operationRecord{{precondition: preconditionCurrentValueChanged, changedSetting: setting.AllowedActions}},
			want:       false,
		},
		{name: "only a rejected update has not changed the state", operations: []operationRecord{rejected}, want: false},
		{name: "unknown update may have changed the state", operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateUnknown}}, want: true},
		{name: "succeeded update then a not started one may have changed the state", operations: []operationRecord{succeeded, {precondition: preconditionUnconfirmed}}, want: true},
		{name: "succeeded update then a rejected one may have changed the state", operations: []operationRecord{succeeded, rejected}, want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := changeRecord{change: toSelectedChange(), operations: tc.operations}
			if got := record.mayHaveChangedState(); got != tc.want {
				t.Errorf("mayHaveChangedState() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-013
func TestChangeRecord_Result(t *testing.T) {
	cases := []struct {
		name   string
		record changeRecord
		want   changeResult
	}{
		{name: "zero value is not attempted", record: changeRecord{}, want: notAttempted},
		{
			name: "success, match and no other setting change is complete",
			record: changeRecord{
				change:     allowAutoMergeChange(),
				operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateMatches,
			},
			want: completed,
		},
		{
			name: "two updates whose first precondition does not hold is not attempted",
			record: changeRecord{
				change:     toSelectedChange(),
				operations: []operationRecord{{precondition: preconditionCurrentValueChanged, changedSetting: setting.AllowedActions}},
			},
			want: notAttempted,
		},
		{
			// 更新をすでに開始しているため、後続を開始しなかったことは未実行にならない。
			// 更新後の観測が期待状態と一致しても、必要な更新操作の成功を確認していないため完了にならない
			name: "two updates whose second precondition does not hold after the first succeeds is failed even with a match",
			record: changeRecord{
				change: toSelectedChange(),
				operations: []operationRecord{
					{precondition: preconditionHolds, update: github.UpdateSucceeded},
					{precondition: preconditionCurrentValueChanged, changedSetting: setting.AllowedActions},
				},
				confirmation: desiredStateMatches,
			},
			want: failed,
		},
		{
			// executeChange は後続の記録を必ず積むため作らない形。完了に全更新操作の成功が要るという定義を、記録の数でも固定する
			name: "two updates with a record only for the succeeded first and a match is failed",
			record: changeRecord{
				change:       toSelectedChange(),
				operations:   []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}},
				confirmation: desiredStateMatches,
			},
			want: failed,
		},
		{
			name: "two updates both succeeded with a match and no other setting change is complete",
			record: changeRecord{
				change: toSelectedChange(),
				operations: []operationRecord{
					{precondition: preconditionHolds, update: github.UpdateSucceeded},
					{precondition: preconditionHolds, update: github.UpdateSucceeded},
				},
				confirmation: desiredStateMatches,
			},
			want: completed,
		},
		{
			name: "two updates with the second unknown and a match is failed",
			record: changeRecord{
				change: toSelectedChange(),
				operations: []operationRecord{
					{precondition: preconditionHolds, update: github.UpdateSucceeded},
					{precondition: preconditionHolds, update: github.UpdateUnknown},
				},
				confirmation: desiredStateMatches,
			},
			want: failed,
		},
		{
			name:   "success and mismatch is failed",
			record: changeRecord{operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateMismatch},
			want:   failed,
		},
		{
			name: "success, match and other setting change is failed",
			record: changeRecord{
				operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateMatches,
				unexpectedOutsideChanges: []unexpectedOutsideChange{{Key: setting.DeleteBranchOnMerge, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
			},
			want: failed,
		},
		{
			name:   "unknown outcome with a match is failed",
			record: changeRecord{operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateUnknown}}, confirmation: desiredStateMatches},
			want:   failed,
		},
		{
			name:   "rejected update is failed",
			record: changeRecord{operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateRejected}}},
			want:   failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.record.result(); got != tc.want {
				t.Errorf("result() = %v, want %v", got, tc.want)
			}
		})
	}
}
