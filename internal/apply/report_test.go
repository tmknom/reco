package apply

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks QR-002
func TestUnavailableReasonText(t *testing.T) {
	cases := []struct {
		reason github.UnavailableReason
		want   string
	}{
		{reason: github.UnavailableActionsDisabled, want: "GitHub Actions is disabled"},
		{reason: github.UnavailableNotSelected, want: `allowed_actions is not "selected"`},
		{reason: github.UnavailablePublicRepository, want: "the repository is public"},
		{reason: github.UnavailableEnvironmentAbsent, want: "the environment does not exist"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := unavailableReasonText(tc.reason); got != tc.want {
				t.Errorf("unavailableReasonText() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("undefined reason panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("unavailableReasonText() did not panic for an undefined reason")
			}
		}()
		_ = unavailableReasonText(0)
	})

	// 判定できない名前は、宣言が効いていないのではなく reco が確かめられなかったことを示すため、confirmationText が not in effect と示さない
	t.Run("undetermined environment name panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("unavailableReasonText() did not panic for an undetermined environment name")
			}
		}()
		_ = unavailableReasonText(github.UnavailableEnvironmentNameUndetermined)
	})

}

//trace:checks FR-013 INV-005
func TestChangesError(t *testing.T) {
	target := mustReference(t, "owner/repo")

	t.Run("all complete have no error", func(t *testing.T) {
		records := []changeRecord{completedRecord(deleteBranchOnMergeChange()), completedRecord(allowAutoMergeChange())}
		if err := changesError(records, target); err != nil {
			t.Errorf("changesError() = %v, want nil", err)
		}
	})

	t.Run("not attempted without a cause is an error naming the target", func(t *testing.T) {
		records := []changeRecord{{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionIdentityMismatch}}}}
		err := changesError(records, target)
		if err == nil || !strings.Contains(err.Error(), "owner/repo") {
			t.Errorf("changesError() = %v, want an error containing the target", err)
		}
	})

	t.Run("failed wraps the cause without repeating the target it names", func(t *testing.T) {
		cause := fmt.Errorf("update repository owner/repo: %w", github.ErrForbidden)
		records := []changeRecord{{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateRejected}}, cause: cause}}
		err := changesError(records, target)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("changesError() = %v, want it to wrap %v", err, github.ErrForbidden)
		}
		if n := strings.Count(err.Error(), "owner/repo"); n != 1 {
			t.Errorf("changesError() = %q, want the target once, got %d", err, n)
		}
	})

	t.Run("failed change after a complete one is the error naming the failed setting", func(t *testing.T) {
		cause := fmt.Errorf("update repository owner/repo: %w", github.ErrForbidden)
		records := []changeRecord{
			completedRecord(deleteBranchOnMergeChange()),
			{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateRejected}}, cause: cause},
		}
		err := changesError(records, target)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("changesError() = %v, want it to wrap %v", err, github.ErrForbidden)
		}
		if err == nil || !strings.Contains(err.Error(), string(setting.AllowAutoMerge)) {
			t.Errorf("changesError() = %v, want it to name %q", err, setting.AllowAutoMerge)
		}
	})

	t.Run("failed change of two updates without a cause names both settings", func(t *testing.T) {
		records := []changeRecord{toSelectedRecord(desiredStateMismatch,
			started(github.UpdateSucceeded), operationRecord{precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.AllowedActions}})}
		err := changesError(records, target)
		want := "actions.permissions.allowed_actions + actions.permissions.selected_actions"
		if err == nil || !strings.Contains(err.Error(), "owner/repo") || !strings.Contains(err.Error(), want) {
			t.Errorf("changesError() = %v, want an error containing the target and %q", err, want)
		}
	})

	t.Run("stopped change after a failed one adds no error", func(t *testing.T) {
		cause := fmt.Errorf("update repository owner/repo: %w", github.ErrForbidden)
		records := []changeRecord{
			{change: deleteBranchOnMergeChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateRejected}}, cause: cause},
			{change: allowAutoMergeChange()},
		}
		err := changesError(records, target)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("changesError() = %v, want it to wrap %v", err, github.ErrForbidden)
		}
		if err == nil || strings.Contains(err.Error(), string(setting.AllowAutoMerge)) {
			t.Errorf("changesError() = %v, want it not to name the stopped %q", err, setting.AllowAutoMerge)
		}
	})
}

//trace:checks FR-013
func TestPossibleCause(t *testing.T) {
	cases := []struct {
		name   string
		record changeRecord
		want   bool
	}{
		{
			name:   "allow_auto_merge mismatch after a successful update has a possible cause",
			record: changeRecord{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateMismatch},
			want:   true,
		},
		{
			name:   "mismatch of another setting has no possible cause",
			record: changeRecord{change: deleteBranchOnMergeChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateMismatch},
			want:   false,
		},
		{
			name:   "mismatch after an unknown outcome has no possible cause",
			record: changeRecord{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateUnknown}}, confirmation: desiredStateMismatch},
			want:   false,
		},
		{
			name:   "unconfirmed state after a successful update has no possible cause",
			record: changeRecord{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateUnconfirmed},
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := possibleCause(tc.record); (got != "") != tc.want {
				t.Errorf("possibleCause() = %q, want present = %v", got, tc.want)
			}
		})
	}
}

func TestNotAttemptedReason_StartedUpdatePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("notAttemptedReason() did not panic for a record whose precondition holds")
		}
	}()
	notAttemptedReason(changeRecord{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionHolds}}})
}

// TestWriteChangeReport_SingleChange は、論理的な設定変更が1件のとき、リポジトリの行に結果と理由を書く表示を確かめる。
//
//trace:checks FR-013 QR-002
func TestWriteChangeReport_SingleChange(t *testing.T) {
	target := mustReference(t, "owner/repo")
	change := allowAutoMergeChange()
	outside := []unexpectedOutsideChange{{Setting: string(setting.DeleteBranchOnMerge), Before: "false", After: "true"}}
	rollbackNote := "Note: Some changes may already have been applied. No automatic rollback was performed.\n"

	cases := []struct {
		name   string
		record changeRecord
		want   string
	}{
		{
			name:   "complete",
			record: completedRecord(change),
			want:   "  owner/repo: updated\n\nApply complete.\nRepositories updated: 1\n",
		},
		{
			name:   "not attempted because the repository ID differs",
			record: changeRecord{change: change, operations: []operationRecord{{precondition: preconditionIdentityMismatch}}},
			want:   "  owner/repo: not attempted\n    Precondition: repository ID differs from the approved repository\n",
		},
		{
			name:   "not attempted because the current value changed",
			record: changeRecord{change: change, operations: []operationRecord{{precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.AllowAutoMerge}}}},
			want:   "  owner/repo: not attempted\n    Precondition: general.allow_auto_merge changed since approval\n",
		},
		{
			name:   "not attempted because the value shown as a side effect changed names that setting rather than allowed_actions",
			record: changeRecord{change: selectedToAllChange(), operations: []operationRecord{{precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.SelectedActions}}}},
			want:   "  owner/repo: not attempted\n    Precondition: actions.permissions.selected_actions changed since approval\n",
		},
		{
			name:   "not attempted because the precondition is not confirmed",
			record: changeRecord{change: change, operations: []operationRecord{{precondition: preconditionUnconfirmed}}, cause: github.ErrNotFound},
			want:   "  owner/repo: not attempted\n    Precondition: not confirmed (read failed)\n",
		},
		{
			name:   "rejected update",
			record: changeRecord{change: change, operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateRejected}}, cause: github.ErrForbidden},
			want: "  owner/repo: failed\n    Update request: rejected\n    Desired state: not checked\n\n" +
				rollbackNote,
		},
		{
			name:   "unknown outcome with a matching state",
			record: changeRecord{change: change, operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateUnknown}}, confirmation: desiredStateMatches},
			want: "  owner/repo: failed\n    Update request: unknown\n" +
				"    Desired state: matches (update outcome still unknown)\n\nNote: No automatic retry was performed.\n",
		},
		{
			name:   "success with an unconfirmed state",
			record: changeRecord{change: change, operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateUnconfirmed},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not confirmed (read failed)\n\n" +
				rollbackNote,
		},
		{
			name:   "success with a changed repository ID",
			record: changeRecord{change: change, operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateIdentityMismatch},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not confirmed (repository ID changed)\n\n" +
				rollbackNote,
		},
		{
			name: "success with an unavailable desired value shows that the declaration is not in effect with the reason",
			record: changeRecord{
				change: allowedActionsChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}},
				confirmation: desiredStateUnavailable, desiredUnavailableReason: github.UnavailableActionsDisabled,
			},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not in effect (GitHub Actions is disabled)\n\n" +
				rollbackNote,
		},
		{
			name:   "success with a mismatch shows the possible cause after the desired state",
			record: changeRecord{change: change, operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateMismatch},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: does not match\n" +
				"    Possible cause: auto-merge is not available for private repositories on GitHub Free, where GitHub may accept the update without applying it\n\n" +
				rollbackNote,
		},
		{
			name: "other setting change",
			record: changeRecord{
				change: change, operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateMatches, unexpectedOutsideChanges: outside,
			},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: matches\n" +
				"    Other settings: general.delete_branch_on_merge changed false -> true (cause not determined)\n\n" +
				rollbackNote,
		},
		{
			name: "other setting change with permission level values",
			record: changeRecord{
				change: canApprovePullRequestReviewsChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateMatches,
				unexpectedOutsideChanges: []unexpectedOutsideChange{{
					Setting: string(setting.DefaultWorkflowPermissions),
					Before:  "read",
					After:   "write",
				}},
			},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: matches\n" +
				"    Other settings: actions.workflow.default_workflow_permissions changed read -> write (cause not determined)\n\n" +
				rollbackNote,
		},
		{
			// 完了済みの操作と未開始の操作を示す。未実行にしない
			name: "second update not started because the policy is not the value expected after the first",
			record: toSelectedRecord(desiredStateMismatch,
				started(github.UpdateSucceeded), operationRecord{precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.AllowedActions}}),
			want: "  owner/repo: failed\n" + updateRequestRows("succeeded", "not started") +
				"    Precondition: actions.permissions.allowed_actions is not the value expected after the preceding update\n" +
				"    Desired state: does not match\n\n" + rollbackNote,
		},
		{
			name:   "second update not started because the read before it failed",
			record: toSelectedRecord(desiredStateMismatch, started(github.UpdateSucceeded), operationRecord{precondition: preconditionUnconfirmed}),
			want: "  owner/repo: failed\n" + updateRequestRows("succeeded", "not started") +
				"    Precondition: not confirmed (read failed)\n    Desired state: does not match\n\n" + rollbackNote,
		},
		{
			// 宣言した3項目が GitHub の戻す値と同じなら、後続を開始しなくても一致を観測し得る。必要な更新操作の成功を確認していないため failed のまま
			name:   "second update not started with a matching state stays failed",
			record: toSelectedRecord(desiredStateMatches, started(github.UpdateSucceeded), operationRecord{precondition: preconditionUnconfirmed}),
			want: "  owner/repo: failed\n" + updateRequestRows("succeeded", "not started") +
				"    Precondition: not confirmed (read failed)\n    Desired state: matches\n\n" + rollbackNote,
		},
		{
			name:   "rejected first update shows the second not started without a precondition",
			record: toSelectedRecord(desiredStateNotChecked, started(github.UpdateRejected)),
			want:   "  owner/repo: failed\n" + updateRequestRows("rejected", "not started") + "    Desired state: not checked\n\n" + rollbackNote,
		},
		{
			name:   "unknown first update has the retry note",
			record: toSelectedRecord(desiredStateMismatch, started(github.UpdateUnknown)),
			want: "  owner/repo: failed\n" + updateRequestRows("unknown", "not started") + "    Desired state: does not match\n\n" +
				"Note: No automatic retry was performed.\n",
		},
		{
			name:   "unknown second update after the first succeeded has the retry note",
			record: toSelectedRecord(desiredStateMatches, started(github.UpdateSucceeded), started(github.UpdateUnknown)),
			want: "  owner/repo: failed\n" + updateRequestRows("succeeded", "unknown") +
				"    Desired state: matches (update outcome still unknown)\n\nNote: No automatic retry was performed.\n",
		},
		{
			name:   "both updates succeeded with a mismatch shows both update requests",
			record: toSelectedRecord(desiredStateMismatch, started(github.UpdateSucceeded), started(github.UpdateSucceeded)),
			want:   "  owner/repo: failed\n" + updateRequestRows("succeeded", "succeeded") + "    Desired state: does not match\n\n" + rollbackNote,
		},
		{
			name:   "first precondition of two updates not holding is not attempted without update requests",
			record: toSelectedRecord(desiredStateNotChecked, operationRecord{precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.AllowedActions}}),
			want:   "  owner/repo: not attempted\n    Precondition: actions.permissions.allowed_actions changed since approval\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeChangeReport(&out, []repositoryRecord{{target: target, changes: []changeRecord{tc.record}}}); err != nil {
				t.Fatalf("writeChangeReport() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangeReport() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}

	t.Run("write failure is returned", func(t *testing.T) {
		err := writeChangeReport(failingWriter{}, []repositoryRecord{{target: target, changes: []changeRecord{completedRecord(change)}}})
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangeReport() error = %v, want %v", err, errWrite)
		}
	})
}

// TestWriteChangeReport_MultipleChanges は、論理的な設定変更が2件以上のときの集約規則と注記の規則を確かめる。
//
//trace:checks FR-013 INV-005 QR-002
func TestWriteChangeReport_MultipleChanges(t *testing.T) {
	target := mustReference(t, "owner/repo")
	stopped := changeRecord{change: allowAutoMergeChange()}
	rollbackNote := "Note: Some changes may already have been applied. No automatic rollback was performed.\n"

	cases := []struct {
		name    string
		records []changeRecord
		want    string
	}{
		{
			name:    "all complete are reported as the repository updated",
			records: []changeRecord{completedRecord(deleteBranchOnMergeChange()), completedRecord(allowAutoMergeChange())},
			want:    "  owner/repo: updated\n\nApply complete.\nRepositories updated: 1\n",
		},
		{
			name: "complete then failed shows each setting without a repository result and the rollback note",
			records: []changeRecord{
				completedRecord(deleteBranchOnMergeChange()),
				{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateRejected}}, cause: github.ErrForbidden},
			},
			want: "  owner/repo\n" +
				"    general.delete_branch_on_merge: updated\n" +
				"    general.allow_auto_merge: failed\n      Update request: rejected\n      Desired state: not checked\n\n" +
				rollbackNote,
		},
		{
			name: "failed then stopped shows the stop reason",
			records: []changeRecord{
				{
					change: deleteBranchOnMergeChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateSucceeded}}, confirmation: desiredStateMatches,
					unexpectedOutsideChanges: []unexpectedOutsideChange{{Setting: string(setting.AllowAutoMerge), Before: "false", After: "true"}},
				},
				stopped,
			},
			want: "  owner/repo\n" +
				"    general.delete_branch_on_merge: failed\n" +
				"      Update request: succeeded\n" +
				"      Desired state: matches\n" +
				"      Other settings: general.allow_auto_merge changed false -> true (cause not determined)\n" +
				"    general.allow_auto_merge: not attempted\n" +
				"      Stopped: a preceding change did not complete\n" +
				"\n" +
				rollbackNote,
		},
		{
			name:    "precondition failure then stopped has no note because no update started",
			records: []changeRecord{{change: deleteBranchOnMergeChange(), operations: []operationRecord{{precondition: preconditionIdentityMismatch}}}, stopped},
			want: "  owner/repo\n" +
				"    general.delete_branch_on_merge: not attempted\n      Precondition: repository ID differs from the approved repository\n" +
				"    general.allow_auto_merge: not attempted\n      Stopped: a preceding change did not complete\n",
		},
		{
			name:    "complete then precondition failure has the rollback note",
			records: []changeRecord{completedRecord(deleteBranchOnMergeChange()), {change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.AllowAutoMerge}}}}},
			want: "  owner/repo\n" +
				"    general.delete_branch_on_merge: updated\n" +
				"    general.allow_auto_merge: not attempted\n      Precondition: general.allow_auto_merge changed since approval\n\n" +
				rollbackNote,
		},
		{
			name: "complete then unknown outcome has the retry note",
			records: []changeRecord{
				completedRecord(deleteBranchOnMergeChange()),
				{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionHolds, update: github.UpdateUnknown}}, confirmation: desiredStateMatches},
			},
			want: "  owner/repo\n" +
				"    general.delete_branch_on_merge: updated\n" +
				"    general.allow_auto_merge: failed\n      Update request: unknown\n" +
				"      Desired state: matches (update outcome still unknown)\n\n" +
				"Note: No automatic retry was performed.\n",
		},
		{
			// 変更結果を二つの管理設定の行に分けず、論理的な設定変更の全体について一つ示す
			name: "change of two updates is shown as one change named by both settings",
			records: []changeRecord{
				completedRecord(allowAutoMergeChange()),
				toSelectedRecord(desiredStateMismatch, started(github.UpdateSucceeded), started(github.UpdateRejected)),
				{change: defaultWorkflowPermissionsChange()},
			},
			want: "  owner/repo\n" +
				"    general.allow_auto_merge: updated\n" +
				"    actions.permissions.allowed_actions + actions.permissions.selected_actions: failed\n" +
				"      Update request (actions.permissions.allowed_actions): succeeded\n" +
				"      Update request (actions.permissions.selected_actions): rejected\n" +
				"      Desired state: does not match\n" +
				"    actions.workflow.default_workflow_permissions: not attempted\n" +
				"      Stopped: a preceding change did not complete\n\n" +
				rollbackNote,
		},
		{
			name: "change of two updates stopped by a preceding change shows the stop reason",
			records: []changeRecord{
				{change: allowAutoMergeChange(), operations: []operationRecord{started(github.UpdateRejected)}, cause: github.ErrForbidden},
				{change: toSelectedChange()},
			},
			want: "  owner/repo\n" +
				"    general.allow_auto_merge: failed\n      Update request: rejected\n      Desired state: not checked\n" +
				"    actions.permissions.allowed_actions + actions.permissions.selected_actions: not attempted\n" +
				"      Stopped: a preceding change did not complete\n\n" +
				rollbackNote,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeChangeReport(&out, []repositoryRecord{{target: target, changes: tc.records}}); err != nil {
				t.Fatalf("writeChangeReport() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangeReport() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}
}

// TestWriteChangeReport_Repositories は、選択したすべての対象リポジトリを宣言順に示す規則、変更不要と停止の行、
// Apply complete. の条件と数え方、対象リポジトリをまたぐ注記の判定を確かめる。期待値は CLI 仕様の apply の表示例から導く。
//
//trace:checks FR-013 INV-005 QR-002
func TestWriteChangeReport_Repositories(t *testing.T) {
	a, b, c := mustReference(t, "example/project-a"), mustReference(t, "example/project-b"), mustReference(t, "example/project-c")
	rollbackNote := "Note: Some changes may already have been applied. No automatic rollback was performed.\n"
	const stoppedByRepository = "    Stopped: a preceding repository did not complete\n"
	mismatched := changeRecord{change: deleteBranchOnMergeChange(), operations: []operationRecord{started(github.UpdateSucceeded)}, confirmation: desiredStateMismatch}
	rejected := changeRecord{change: allowAutoMergeChange(), operations: []operationRecord{started(github.UpdateRejected)}, cause: github.ErrForbidden}
	valueChanged := changeRecord{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.AllowAutoMerge}}}}
	stoppedAutoMerge := changeRecord{change: allowAutoMergeChange()}
	stoppedBranch := changeRecord{change: deleteBranchOnMergeChange()}

	cases := []struct {
		name    string
		records []repositoryRecord
		want    string
	}{
		{
			name: "complete, no change and complete report two repositories updated",
			records: []repositoryRecord{
				{target: a, changes: []changeRecord{completedRecord(allowAutoMergeChange())}},
				{target: b},
				{target: c, changes: []changeRecord{completedRecord(defaultWorkflowPermissionsChange())}},
			},
			want: "  example/project-a: updated\n  example/project-b: no change\n  example/project-c: updated\n\n" +
				"Apply complete.\nRepositories updated: 2\n",
		},
		{
			name: "no change after a failure stays no change and the repository after it is stopped",
			records: []repositoryRecord{
				{target: a, changes: []changeRecord{mismatched}},
				{target: b},
				{target: c, changes: []changeRecord{stoppedAutoMerge}},
			},
			want: "  example/project-a: failed\n    Update request: succeeded\n    Desired state: does not match\n" +
				"  example/project-b: no change\n" +
				"  example/project-c: not attempted\n" + stoppedByRepository + "\n" +
				rollbackNote,
		},
		{
			// 最後の対象リポジトリだけで Apply complete. を判定する実装を失敗させる
			name: "failure followed only by no change is not reported as apply complete",
			records: []repositoryRecord{
				{target: a, changes: []changeRecord{mismatched}},
				{target: b},
			},
			want: "  example/project-a: failed\n    Update request: succeeded\n    Desired state: does not match\n" +
				"  example/project-b: no change\n\n" +
				rollbackNote,
		},
		{
			name: "stopped repository with two changes is shown as one line with the reason",
			records: []repositoryRecord{
				{target: a, changes: []changeRecord{rejected}},
				{target: b, changes: []changeRecord{stoppedBranch, stoppedAutoMerge}},
			},
			want: "  example/project-a: failed\n    Update request: rejected\n    Desired state: not checked\n" +
				"  example/project-b: not attempted\n" + stoppedByRepository + "\n" +
				rollbackNote,
		},
		{
			name: "stop by a preceding change and by a preceding repository are worded differently",
			records: []repositoryRecord{
				{target: a, changes: []changeRecord{{change: deleteBranchOnMergeChange(), operations: []operationRecord{started(github.UpdateRejected)}}, stoppedAutoMerge}},
				{target: b, changes: []changeRecord{stoppedAutoMerge}},
			},
			want: "  example/project-a\n" +
				"    general.delete_branch_on_merge: failed\n      Update request: rejected\n      Desired state: not checked\n" +
				"    general.allow_auto_merge: not attempted\n      Stopped: a preceding change did not complete\n" +
				"  example/project-b: not attempted\n" + stoppedByRepository + "\n" +
				rollbackNote,
		},
		{
			name: "complete, failed and stopped is not reported as apply complete",
			records: []repositoryRecord{
				{target: a, changes: []changeRecord{completedRecord(allowAutoMergeChange())}},
				{target: b, changes: []changeRecord{rejected}},
				{target: c, changes: []changeRecord{stoppedAutoMerge}},
			},
			want: "  example/project-a: updated\n" +
				"  example/project-b: failed\n    Update request: rejected\n    Desired state: not checked\n" +
				"  example/project-c: not attempted\n" + stoppedByRepository + "\n" +
				rollbackNote,
		},
		{
			// 完了以外を含む対象リポジトリは更新を開始していないが、先に完了した対象リポジトリの変更が残る
			name: "precondition failure after a completed repository has the rollback note",
			records: []repositoryRecord{
				{target: a, changes: []changeRecord{completedRecord(allowAutoMergeChange())}},
				{target: b, changes: []changeRecord{valueChanged}},
				{target: c, changes: []changeRecord{stoppedAutoMerge}},
			},
			want: "  example/project-a: updated\n" +
				"  example/project-b: not attempted\n    Precondition: general.allow_auto_merge changed since approval\n" +
				"  example/project-c: not attempted\n" + stoppedByRepository + "\n" +
				rollbackNote,
		},
		{
			name: "precondition failure in the first repository then a stopped one has no note because no update started",
			records: []repositoryRecord{
				{target: a, changes: []changeRecord{valueChanged}},
				{target: b, changes: []changeRecord{stoppedAutoMerge}},
			},
			want: "  example/project-a: not attempted\n    Precondition: general.allow_auto_merge changed since approval\n" +
				"  example/project-b: not attempted\n" + stoppedByRepository,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeChangeReport(&out, tc.records); err != nil {
				t.Fatalf("writeChangeReport() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangeReport() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}
}

// TestRepositoriesError は、宣言順で最初の完了以外の変更結果だけを error にし、停止した対象リポジトリの error を足さない規則を確かめる。
//
//trace:checks FR-013 INV-005
func TestRepositoriesError(t *testing.T) {
	first, second, third := mustReference(t, "owner/first"), mustReference(t, "owner/second"), mustReference(t, "owner/third")
	rejected := changeRecord{change: allowAutoMergeChange(), operations: []operationRecord{started(github.UpdateRejected)}, cause: github.ErrForbidden}
	valueChanged := changeRecord{change: allowAutoMergeChange(), operations: []operationRecord{{precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.AllowAutoMerge}}}}

	t.Run("complete and no change have no error", func(t *testing.T) {
		records := []repositoryRecord{{target: first, changes: []changeRecord{completedRecord(allowAutoMergeChange())}}, {target: second}}
		if err := repositoriesError(records); err != nil {
			t.Errorf("repositoriesError() = %v, want nil", err)
		}
	})

	t.Run("failure after no change wraps its cause and does not name the stopped repository", func(t *testing.T) {
		records := []repositoryRecord{
			{target: first},
			{target: second, changes: []changeRecord{rejected}},
			{target: third, changes: []changeRecord{{change: deleteBranchOnMergeChange()}}},
		}
		err := repositoriesError(records)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("repositoriesError() = %v, want it to wrap %v", err, github.ErrForbidden)
		}
		if err == nil || !strings.Contains(err.Error(), string(setting.AllowAutoMerge)) {
			t.Errorf("repositoriesError() = %v, want it to name %q", err, setting.AllowAutoMerge)
		}
		if err != nil && strings.Contains(err.Error(), string(setting.DeleteBranchOnMerge)) {
			t.Errorf("repositoriesError() = %v, want it not to name the stopped %q", err, setting.DeleteBranchOnMerge)
		}
	})

	t.Run("precondition failure without a cause names its own repository", func(t *testing.T) {
		records := []repositoryRecord{
			{target: first, changes: []changeRecord{completedRecord(allowAutoMergeChange())}},
			{target: second, changes: []changeRecord{valueChanged}},
		}
		err := repositoriesError(records)
		if err == nil || !strings.Contains(err.Error(), second.String()) || strings.Contains(err.Error(), first.String()) {
			t.Errorf("repositoriesError() = %v, want an error naming %s and not %s", err, second, first)
		}
	})
}

// toSelectedRecord は、toSelectedChange の更新操作の記録が operations の記録。
func toSelectedRecord(confirmation desiredStateConfirmation, operations ...operationRecord) changeRecord {
	return changeRecord{change: toSelectedChange(), operations: operations, confirmation: confirmation}
}

// updateRequestRows は、toSelectedChange の2つの更新操作の結果を、リポジトリの行の下の字下げで示す行。
func updateRequestRows(first, second string) string {
	return "    Update request (actions.permissions.allowed_actions): " + first + "\n" +
		"    Update request (actions.permissions.selected_actions): " + second + "\n"
}

// TestWriteChangeReport_Environments は、Environment の作成と方式の変更の変更結果の表示を確かめる。期待値は CLI 仕様の apply の表示例から導く。
//
//trace:checks FR-013 QR-002
func TestWriteChangeReport_Environments(t *testing.T) {
	target := mustReference(t, "owner/repo")
	creation := creationChange(t, "production", modeValue(setting.DeploymentBranchPolicyProtectedBranches))
	staging := modeChange(t, "staging", observedEnvironment(7, "Staging", setting.DeploymentBranchPolicyAll), setting.DeploymentBranchPolicyProtectedBranches)
	succeeded := []operationRecord{started(github.UpdateSucceeded)}
	rollbackNote := "Note: Some changes may already have been applied. No automatic rollback was performed.\n"

	cases := []struct {
		name    string
		records []changeRecord
		want    string
	}{
		{
			name: "creation with an environment created since approval names the environment",
			records: []changeRecord{{change: creation, operations: []operationRecord{{
				precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "production")},
			}}}},
			want: "  owner/repo: not attempted\n    Precondition: environments.production changed since approval\n",
		},
		{
			name: "mode change with only the mode changed names the mode",
			records: []changeRecord{{change: staging, operations: []operationRecord{{
				precondition: preconditionCurrentValueChanged, changedSetting: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: mustEnvironmentName(t, "staging")},
			}}}},
			want: "  owner/repo: not attempted\n    Precondition: environments.staging.deployment_branch_policy changed since approval\n",
		},
		{
			name: "creation completed and a mode change with a changed protection are named by the declared spelling",
			records: []changeRecord{
				completedRecord(creation),
				{
					change: staging, operations: succeeded, confirmation: desiredStateMatches,
					unexpectedOutsideChanges: []unexpectedOutsideChange{{Setting: "environments.staging.wait_timer", Before: "30", After: "none"}},
				},
			},
			want: "  owner/repo\n" +
				"    environments.production + environments.production.deployment_branch_policy: updated\n" +
				"    environments.staging.deployment_branch_policy: failed\n" +
				"      Update request: succeeded\n" +
				"      Desired state: matches\n" +
				"      Other settings: environments.staging.wait_timer changed 30 -> none (cause not determined)\n\n" +
				rollbackNote,
		},
		{
			name:    "mode change of an environment absent after the update is not in effect",
			records: []changeRecord{{change: staging, operations: succeeded, confirmation: desiredStateUnavailable, desiredUnavailableReason: github.UnavailableEnvironmentAbsent}},
			want:    "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not in effect (the environment does not exist)\n\n" + rollbackNote,
		},
		{
			name:    "creation whose existence cannot be determined after the update is not confirmed",
			records: []changeRecord{{change: creation, operations: succeeded, confirmation: desiredStateUnavailable, desiredUnavailableReason: github.UnavailableEnvironmentNameUndetermined}},
			want:    "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not confirmed (environment existence unknown)\n\n" + rollbackNote,
		},
		{
			name:    "creation absent after the update does not match",
			records: []changeRecord{{change: creationChange(t, "production", setting.Value{}), operations: succeeded, confirmation: desiredStateMismatch}},
			want:    "  owner/repo: failed\n    Update request: succeeded\n    Desired state: does not match\n\n" + rollbackNote,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeChangeReport(&out, []repositoryRecord{{target: target, changes: tc.records}}); err != nil {
				t.Fatalf("writeChangeReport() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangeReport() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}
}

// TestWriteChangeReport_DeploymentPatterns は、個別パターンを一つずつ取り除き、加える論理的な設定変更の変更結果の表示を確かめる。
// 期待値は CLI 仕様の apply の表示例から導く。
//
//trace:checks FR-013 QR-002
func TestWriteChangeReport_DeploymentPatterns(t *testing.T) {
	target := mustReference(t, "owner/repo")
	main, release, version := branchPattern("main"), branchPattern("release/*"), tagPattern("v*")
	approvedAll := observedEnvironment(7, "production", setting.DeploymentBranchPolicyAll)
	existence := setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "production")}
	policy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: mustEnvironmentName(t, "production")}
	succeeded := started(github.UpdateSucceeded)
	changed := func(unit setting.Unit) operationRecord {
		return operationRecord{precondition: preconditionCurrentValueChanged, changedSetting: unit}
	}
	rollbackNote := "Note: Some changes may already have been applied. No automatic rollback was performed.\n"

	cases := []struct {
		name    string
		records []changeRecord
		want    string
	}{
		{
			name: "addition rejected after the mode update shows each addition with a different outcome on its own line",
			records: []changeRecord{{
				change:       deploymentPatternsChange(t, "production", approvedAll, selectedPolicy(main, release, version)),
				operations:   []operationRecord{succeeded, succeeded, started(github.UpdateRejected)},
				confirmation: desiredStateMismatch,
			}},
			want: "  owner/repo: failed\n" +
				"    Update request (environments.production.deployment_branch_policy): succeeded\n" +
				"    Update request (patterns to add [branch \"main\"]): succeeded\n" +
				"    Update request (patterns to add [branch \"release/*\"]): rejected\n" +
				"    Update request (patterns to add [tag \"v*\"]): not started\n" +
				"    Desired state: does not match\n\n" + rollbackNote,
		},
		{
			name: "additions not started after removals show one line for each direction and the precondition after the additions",
			records: []changeRecord{{
				change: deploymentPatternsChange(t, "production",
					selectedEnvironment(7, "production", []setting.DeploymentPattern{branchPattern("feature/*"), branchPattern("old/*")}, 11, 12), selectedPolicy(release, version)),
				operations:   []operationRecord{succeeded, succeeded, changed(policy)},
				confirmation: desiredStateMismatch,
			}},
			want: "  owner/repo: failed\n" +
				"    Update request (patterns to remove [branch \"feature/*\", branch \"old/*\"]): succeeded\n" +
				"    Update request (patterns to add [branch \"release/*\", tag \"v*\"]): not started\n" +
				"    Precondition: environments.production.deployment_branch_policy is not the value expected after the preceding update\n" +
				"    Desired state: does not match\n\n" + rollbackNote,
		},
		{
			name: "removal not started by the precondition shows the precondition after its removal line and before the additions",
			records: []changeRecord{{
				change: deploymentPatternsChange(t, "production",
					selectedEnvironment(7, "production", []setting.DeploymentPattern{branchPattern("a"), branchPattern("b"), branchPattern("c")}, 1, 2, 3),
					selectedPolicy(branchPattern("d"), branchPattern("e"))),
				operations:   []operationRecord{succeeded, changed(policy)},
				confirmation: desiredStateMismatch,
			}},
			want: "  owner/repo: failed\n" +
				"    Update request (patterns to remove [branch \"a\"]): succeeded\n" +
				"    Update request (patterns to remove [branch \"b\", branch \"c\"]): not started\n" +
				"    Precondition: environments.production.deployment_branch_policy is not the value expected after the preceding update\n" +
				"    Update request (patterns to add [branch \"d\", branch \"e\"]): not started\n" +
				"    Desired state: does not match\n\n" + rollbackNote,
		},
		{
			name: "addition of unknown outcome after a creation is named by the environment and its policy once",
			records: []changeRecord{
				completedRecord(allowAutoMergeChange()),
				{
					change:       creationWithPatternsChange(t, "production", selectedPolicy(main, version)),
					operations:   []operationRecord{succeeded, started(github.UpdateUnknown)},
					confirmation: desiredStateMismatch,
				},
			},
			want: "  owner/repo\n" +
				"    general.allow_auto_merge: updated\n" +
				"    environments.production + environments.production.deployment_branch_policy: failed\n" +
				"      Update request (environments.production): succeeded\n" +
				"      Update request (patterns to add [branch \"main\"]): unknown\n" +
				"      Update request (patterns to add [tag \"v*\"]): not started\n" +
				"      Desired state: does not match\n\n" +
				"Note: No automatic retry was performed.\n",
		},
		{
			name: "environment not as expected before a later update after a creation names the environment",
			records: []changeRecord{{
				change:       creationWithPatternsChange(t, "production", selectedPolicy(main, version)),
				operations:   []operationRecord{succeeded, succeeded, changed(existence)},
				confirmation: desiredStateMismatch,
			}},
			want: "  owner/repo: failed\n" +
				"    Update request (environments.production): succeeded\n" +
				"    Update request (patterns to add [branch \"main\"]): succeeded\n" +
				"    Update request (patterns to add [tag \"v*\"]): not started\n" +
				"    Precondition: environments.production is not the environment expected after the preceding update\n" +
				"    Desired state: does not match\n\n" + rollbackNote,
		},
		{
			name: "existing environment changed before a later update is shown as changed since approval",
			records: []changeRecord{{
				change: deploymentPatternsChange(t, "production",
					selectedEnvironment(7, "production", []setting.DeploymentPattern{branchPattern("feature/*")}, 11), selectedPolicy(release)),
				operations:   []operationRecord{succeeded, changed(existence)},
				confirmation: desiredStateMismatch,
			}},
			want: "  owner/repo: failed\n" +
				"    Update request (patterns to remove [branch \"feature/*\"]): succeeded\n" +
				"    Update request (patterns to add [branch \"release/*\"]): not started\n" +
				"    Precondition: environments.production changed since approval\n" +
				"    Desired state: does not match\n\n" + rollbackNote,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeChangeReport(&out, []repositoryRecord{{target: target, changes: tc.records}}); err != nil {
				t.Fatalf("writeChangeReport() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangeReport() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}

	t.Run("undefined direction of a pattern update panics", func(t *testing.T) {
		change := deploymentPatternsChange(t, "production", selectedEnvironment(7, "production", []setting.DeploymentPattern{main}, 10), selectedPolicy(main, release, version))
		change.Operations[0].DeploymentPatternChange.Direction = changeplan.RemoveDeploymentPattern + 1
		defer func() {
			if recover() == nil {
				t.Error("updateRequestLines() did not panic")
			}
		}()
		updateRequestLines(changeRecord{change: change, operations: []operationRecord{succeeded, succeeded}})
	})
}
