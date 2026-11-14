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

// completedRecord は、change の更新操作が成功し、期待状態との一致を確認できた記録。
func completedRecord(change changeplan.LogicalChange) changeRecord {
	return changeRecord{change: change, precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMatches}
}

//trace:checks FR-013
//trace:checks INV-005
func TestChangesError(t *testing.T) {
	target := mustReference(t, "owner/repo")

	t.Run("all complete have no error", func(t *testing.T) {
		records := []changeRecord{completedRecord(deleteBranchOnMergeChange()), completedRecord(allowAutoMergeChange())}
		if err := changesError(records, target); err != nil {
			t.Errorf("changesError() = %v, want nil", err)
		}
	})

	t.Run("not attempted without a cause is an error naming the target", func(t *testing.T) {
		records := []changeRecord{{change: allowAutoMergeChange(), precondition: preconditionIdentityMismatch}}
		err := changesError(records, target)
		if err == nil || !strings.Contains(err.Error(), "owner/repo") {
			t.Errorf("changesError() = %v, want an error containing the target", err)
		}
	})

	t.Run("failed wraps the cause without repeating the target it names", func(t *testing.T) {
		cause := fmt.Errorf("update repository owner/repo: %w", github.ErrForbidden)
		records := []changeRecord{{change: allowAutoMergeChange(), precondition: preconditionHolds, update: github.UpdateRejected, cause: cause}}
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
			{change: allowAutoMergeChange(), precondition: preconditionHolds, update: github.UpdateRejected, cause: cause},
		}
		err := changesError(records, target)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("changesError() = %v, want it to wrap %v", err, github.ErrForbidden)
		}
		if err == nil || !strings.Contains(err.Error(), string(setting.AllowAutoMerge)) {
			t.Errorf("changesError() = %v, want it to name %q", err, setting.AllowAutoMerge)
		}
	})

	t.Run("stopped change after a failed one adds no error", func(t *testing.T) {
		cause := fmt.Errorf("update repository owner/repo: %w", github.ErrForbidden)
		records := []changeRecord{
			{change: deleteBranchOnMergeChange(), precondition: preconditionHolds, update: github.UpdateRejected, cause: cause},
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
			record: changeRecord{change: allowAutoMergeChange(), precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMismatch},
			want:   true,
		},
		{
			name:   "mismatch of another setting has no possible cause",
			record: changeRecord{change: deleteBranchOnMergeChange(), precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMismatch},
			want:   false,
		},
		{
			name:   "mismatch after an unknown outcome has no possible cause",
			record: changeRecord{change: allowAutoMergeChange(), precondition: preconditionHolds, update: github.UpdateUnknown, confirmation: desiredStateMismatch},
			want:   false,
		},
		{
			name:   "unconfirmed state after a successful update has no possible cause",
			record: changeRecord{change: allowAutoMergeChange(), precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateUnconfirmed},
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
	notAttemptedReason(changeRecord{change: allowAutoMergeChange(), precondition: preconditionHolds})
}

// TestWriteChangeReport_SingleChange は、論理的な設定変更が1件のとき、リポジトリの行に結果と理由を書く表示を確かめる。
//
//trace:checks FR-013
//trace:checks QR-002
func TestWriteChangeReport_SingleChange(t *testing.T) {
	target := mustReference(t, "owner/repo")
	change := allowAutoMergeChange()
	outside := []unexpectedOutsideChange{{Key: setting.DeleteBranchOnMerge, Before: setting.BoolValue(false), After: setting.BoolValue(true)}}
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
			record: changeRecord{change: change, precondition: preconditionIdentityMismatch},
			want:   "  owner/repo: not attempted\n    Precondition: repository ID differs from the approved repository\n",
		},
		{
			name:   "not attempted because the current value changed",
			record: changeRecord{change: change, precondition: preconditionCurrentValueChanged},
			want:   "  owner/repo: not attempted\n    Precondition: general.allow_auto_merge changed since approval\n",
		},
		{
			name:   "not attempted because the precondition is not confirmed",
			record: changeRecord{change: change, precondition: preconditionUnconfirmed, cause: github.ErrNotFound},
			want:   "  owner/repo: not attempted\n    Precondition: not confirmed (read failed)\n",
		},
		{
			name:   "rejected update",
			record: changeRecord{change: change, precondition: preconditionHolds, update: github.UpdateRejected, cause: github.ErrForbidden},
			want: "  owner/repo: failed\n    Update request: rejected\n    Desired state: not checked\n\n" +
				rollbackNote,
		},
		{
			name:   "unknown outcome with a matching state",
			record: changeRecord{change: change, precondition: preconditionHolds, update: github.UpdateUnknown, confirmation: desiredStateMatches},
			want: "  owner/repo: failed\n    Update request: unknown\n" +
				"    Desired state: matches (update outcome still unknown)\n\nNote: No automatic retry was performed.\n",
		},
		{
			name:   "success with an unconfirmed state",
			record: changeRecord{change: change, precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateUnconfirmed},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not confirmed (read failed)\n\n" +
				rollbackNote,
		},
		{
			name:   "success with a changed repository ID",
			record: changeRecord{change: change, precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateIdentityMismatch},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not confirmed (repository ID changed)\n\n" +
				rollbackNote,
		},
		{
			name:   "success with a mismatch shows the possible cause after the desired state",
			record: changeRecord{change: change, precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMismatch},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: does not match\n" +
				"    Possible cause: auto-merge is not available for private repositories on GitHub Free, where GitHub may accept the update without applying it\n\n" +
				rollbackNote,
		},
		{
			name: "other setting change",
			record: changeRecord{
				change: change, precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMatches, unexpectedOutsideChanges: outside,
			},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: matches\n" +
				"    Other settings: general.delete_branch_on_merge changed false -> true (cause not determined)\n\n" +
				rollbackNote,
		},
		{
			name: "other setting change with permission level values",
			record: changeRecord{
				change: canApprovePullRequestReviewsChange(), precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMatches,
				unexpectedOutsideChanges: []unexpectedOutsideChange{{
					Key:    setting.DefaultWorkflowPermissions,
					Before: setting.PermissionLevelValue(setting.PermissionRead),
					After:  setting.PermissionLevelValue(setting.PermissionWrite),
				}},
			},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: matches\n" +
				"    Other settings: actions.workflow.default_workflow_permissions changed read -> write (cause not determined)\n\n" +
				rollbackNote,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeChangeReport(&out, target, []changeRecord{tc.record}); err != nil {
				t.Fatalf("writeChangeReport() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangeReport() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}

	t.Run("write failure is returned", func(t *testing.T) {
		err := writeChangeReport(failingWriter{}, target, []changeRecord{{change: change}})
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangeReport() error = %v, want %v", err, errWrite)
		}
	})
}

// TestWriteChangeReport_MultipleChanges は、論理的な設定変更が2件以上のときの集約規則と注記の規則を確かめる。
//
//trace:checks FR-013
//trace:checks INV-005
//trace:checks QR-002
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
				{change: allowAutoMergeChange(), precondition: preconditionHolds, update: github.UpdateRejected, cause: github.ErrForbidden},
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
					change: deleteBranchOnMergeChange(), precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMatches,
					unexpectedOutsideChanges: []unexpectedOutsideChange{{Key: setting.AllowAutoMerge, Before: setting.BoolValue(false), After: setting.BoolValue(true)}},
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
			records: []changeRecord{{change: deleteBranchOnMergeChange(), precondition: preconditionIdentityMismatch}, stopped},
			want: "  owner/repo\n" +
				"    general.delete_branch_on_merge: not attempted\n      Precondition: repository ID differs from the approved repository\n" +
				"    general.allow_auto_merge: not attempted\n      Stopped: a preceding change did not complete\n",
		},
		{
			name:    "complete then precondition failure has the rollback note",
			records: []changeRecord{completedRecord(deleteBranchOnMergeChange()), {change: allowAutoMergeChange(), precondition: preconditionCurrentValueChanged}},
			want: "  owner/repo\n" +
				"    general.delete_branch_on_merge: updated\n" +
				"    general.allow_auto_merge: not attempted\n      Precondition: general.allow_auto_merge changed since approval\n\n" +
				rollbackNote,
		},
		{
			name: "complete then unknown outcome has the retry note",
			records: []changeRecord{
				completedRecord(deleteBranchOnMergeChange()),
				{change: allowAutoMergeChange(), precondition: preconditionHolds, update: github.UpdateUnknown, confirmation: desiredStateMatches},
			},
			want: "  owner/repo\n" +
				"    general.delete_branch_on_merge: updated\n" +
				"    general.allow_auto_merge: failed\n      Update request: unknown\n" +
				"      Desired state: matches (update outcome still unknown)\n\n" +
				"Note: No automatic retry was performed.\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeChangeReport(&out, target, tc.records); err != nil {
				t.Fatalf("writeChangeReport() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangeReport() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}
}
