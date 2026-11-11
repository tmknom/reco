package apply

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
)

//trace:checks FR-013
func TestChangeReportResult(t *testing.T) {
	cases := []struct {
		name   string
		report changeReport
		want   changeResult
	}{
		{name: "zero value is not attempted", report: changeReport{}, want: notAttempted},
		{
			name:   "success, match and no unmanaged change is complete",
			report: changeReport{precondition: preconditionHolds, update: updateSucceeded, confirmation: desiredStateMatches},
			want:   completed,
		},
		{
			name:   "success and mismatch is failed",
			report: changeReport{precondition: preconditionHolds, update: updateSucceeded, confirmation: desiredStateMismatch},
			want:   failed,
		},
		{
			name: "success, match and unmanaged change is failed",
			report: changeReport{
				precondition: preconditionHolds, update: updateSucceeded, confirmation: desiredStateMatches,
				unmanagedChanges: []unmanagedChange{{Setting: "general.delete_branch_on_merge", Before: false, After: true}},
			},
			want: failed,
		},
		{
			name:   "unknown outcome with a match is failed",
			report: changeReport{precondition: preconditionHolds, update: updateOutcomeUnknown, confirmation: desiredStateMatches},
			want:   failed,
		},
		{
			name:   "failed update is failed",
			report: changeReport{precondition: preconditionHolds, update: updateFailed},
			want:   failed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.report.result(); got != tc.want {
				t.Errorf("result() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-013
func TestChangeReportErr(t *testing.T) {
	target := mustReference(t, "owner/repo")
	change := allowAutoMergeChange()

	t.Run("complete has no error", func(t *testing.T) {
		report := changeReport{precondition: preconditionHolds, update: updateSucceeded, confirmation: desiredStateMatches}
		if err := report.err(target, change); err != nil {
			t.Errorf("err() = %v, want nil", err)
		}
	})

	t.Run("not attempted without a cause is an error naming the target", func(t *testing.T) {
		report := changeReport{precondition: preconditionIdentityMismatch}
		err := report.err(target, change)
		if err == nil || !strings.Contains(err.Error(), "owner/repo") {
			t.Errorf("err() = %v, want an error containing the target", err)
		}
	})

	t.Run("failed wraps the cause without repeating the target it names", func(t *testing.T) {
		cause := fmt.Errorf("update repository owner/repo: %w", github.ErrForbidden)
		report := changeReport{precondition: preconditionHolds, update: updateFailed, cause: cause}
		err := report.err(target, change)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("err() = %v, want it to wrap %v", err, github.ErrForbidden)
		}
		if n := strings.Count(err.Error(), "owner/repo"); n != 1 {
			t.Errorf("err() = %q, want the target once, got %d", err, n)
		}
	})
}

//trace:checks FR-013
//trace:checks QR-002
func TestWriteChangeReport(t *testing.T) {
	target := mustReference(t, "owner/repo")
	change := allowAutoMergeChange()
	unmanaged := []unmanagedChange{{Setting: "general.delete_branch_on_merge", Before: false, After: true}}
	rollbackNote := "Note: Some changes may already have been applied. No automatic rollback was performed.\n"

	cases := []struct {
		name   string
		report changeReport
		want   string
	}{
		{
			name:   "complete",
			report: changeReport{precondition: preconditionHolds, update: updateSucceeded, confirmation: desiredStateMatches},
			want:   "  owner/repo: updated\n\nApply complete.\nRepositories updated: 1\n",
		},
		{
			name:   "not attempted because the repository ID differs",
			report: changeReport{precondition: preconditionIdentityMismatch},
			want:   "  owner/repo: not attempted\n    Precondition: repository ID differs from the approved repository\n",
		},
		{
			name:   "not attempted because the current value changed",
			report: changeReport{precondition: preconditionCurrentValueChanged},
			want:   "  owner/repo: not attempted\n    Precondition: general.allow_auto_merge changed since approval\n",
		},
		{
			name:   "not attempted because the precondition is not confirmed",
			report: changeReport{precondition: preconditionUnconfirmed, cause: github.ErrNotFound},
			want:   "  owner/repo: not attempted\n    Precondition: not confirmed (read failed)\n",
		},
		{
			name:   "rejected update",
			report: changeReport{precondition: preconditionHolds, update: updateFailed, cause: github.ErrForbidden},
			want: "  owner/repo: failed\n    Update request: rejected\n    Desired state: not checked\n\n" +
				rollbackNote,
		},
		{
			name:   "unknown outcome with a matching state",
			report: changeReport{precondition: preconditionHolds, update: updateOutcomeUnknown, confirmation: desiredStateMatches},
			want: "  owner/repo: failed\n    Update request: unknown (response unavailable)\n" +
				"    Desired state: matches (update outcome still unknown)\n\nNote: No automatic retry was performed.\n",
		},
		{
			name:   "success with an unconfirmed state",
			report: changeReport{precondition: preconditionHolds, update: updateSucceeded, confirmation: desiredStateUnconfirmed},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not confirmed (read failed)\n\n" +
				rollbackNote,
		},
		{
			name:   "success with a changed repository ID",
			report: changeReport{precondition: preconditionHolds, update: updateSucceeded, confirmation: desiredStateRepositoryChanged},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not confirmed (repository ID changed)\n\n" +
				rollbackNote,
		},
		{
			name:   "success with a mismatch",
			report: changeReport{precondition: preconditionHolds, update: updateSucceeded, confirmation: desiredStateMismatch},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: does not match\n\n" +
				rollbackNote,
		},
		{
			name: "unmanaged setting change",
			report: changeReport{
				precondition: preconditionHolds, update: updateSucceeded, confirmation: desiredStateMatches, unmanagedChanges: unmanaged,
			},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: matches\n" +
				"    Unmanaged settings: general.delete_branch_on_merge changed false -> true (cause not determined)\n\n" +
				rollbackNote,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeChangeReport(&out, target, change, tc.report); err != nil {
				t.Fatalf("writeChangeReport() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangeReport() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}

	t.Run("write failure is returned", func(t *testing.T) {
		err := writeChangeReport(failingWriter{}, target, change, changeReport{})
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangeReport() error = %v, want %v", err, errWrite)
		}
	})
}
