package apply

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-013
func TestChangeError(t *testing.T) {
	target := mustReference(t, "owner/repo")
	change := allowAutoMergeChange()

	t.Run("complete has no error", func(t *testing.T) {
		record := changeRecord{precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMatches}
		if err := changeError(record, target, change); err != nil {
			t.Errorf("changeError() = %v, want nil", err)
		}
	})

	t.Run("not attempted without a cause is an error naming the target", func(t *testing.T) {
		record := changeRecord{precondition: preconditionIdentityMismatch}
		err := changeError(record, target, change)
		if err == nil || !strings.Contains(err.Error(), "owner/repo") {
			t.Errorf("changeError() = %v, want an error containing the target", err)
		}
	})

	t.Run("failed wraps the cause without repeating the target it names", func(t *testing.T) {
		cause := fmt.Errorf("update repository owner/repo: %w", github.ErrForbidden)
		record := changeRecord{precondition: preconditionHolds, update: github.UpdateRejected, cause: cause}
		err := changeError(record, target, change)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("changeError() = %v, want it to wrap %v", err, github.ErrForbidden)
		}
		if n := strings.Count(err.Error(), "owner/repo"); n != 1 {
			t.Errorf("changeError() = %q, want the target once, got %d", err, n)
		}
	})
}

//trace:checks FR-013
func TestPossibleCause(t *testing.T) {
	succeededMismatch := changeRecord{precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMismatch}
	cases := []struct {
		name   string
		record changeRecord
		key    setting.Key
		want   bool
	}{
		{name: "allow_auto_merge mismatch after a successful update has a possible cause", record: succeededMismatch, key: setting.AllowAutoMerge, want: true},
		{name: "mismatch of another setting has no possible cause", record: succeededMismatch, key: setting.DeleteBranchOnMerge, want: false},
		{
			name:   "mismatch after an unknown outcome has no possible cause",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateUnknown, confirmation: desiredStateMismatch},
			key:    setting.AllowAutoMerge, want: false,
		},
		{
			name:   "unconfirmed state after a successful update has no possible cause",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateUnconfirmed},
			key:    setting.AllowAutoMerge, want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := possibleCause(tc.record, tc.key) != ""; got != tc.want {
				t.Errorf("possibleCause(%q) = %q, want present = %v", tc.key, possibleCause(tc.record, tc.key), tc.want)
			}
		})
	}
}

//trace:checks FR-013
//trace:checks QR-002
func TestWriteChangeReport(t *testing.T) {
	target := mustReference(t, "owner/repo")
	change := allowAutoMergeChange()
	unmanaged := []unexpectedUnmanagedChange{{Key: setting.DeleteBranchOnMerge, Before: false, After: true}}
	rollbackNote := "Note: Some changes may already have been applied. No automatic rollback was performed.\n"

	cases := []struct {
		name   string
		record changeRecord
		want   string
	}{
		{
			name:   "complete",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMatches},
			want:   "  owner/repo: updated\n\nApply complete.\nRepositories updated: 1\n",
		},
		{
			name:   "not attempted because the repository ID differs",
			record: changeRecord{precondition: preconditionIdentityMismatch},
			want:   "  owner/repo: not attempted\n    Precondition: repository ID differs from the approved repository\n",
		},
		{
			name:   "not attempted because the current value changed",
			record: changeRecord{precondition: preconditionCurrentValueChanged},
			want:   "  owner/repo: not attempted\n    Precondition: general.allow_auto_merge changed since approval\n",
		},
		{
			name:   "not attempted because the precondition is not confirmed",
			record: changeRecord{precondition: preconditionUnconfirmed, cause: github.ErrNotFound},
			want:   "  owner/repo: not attempted\n    Precondition: not confirmed (read failed)\n",
		},
		{
			name:   "rejected update",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateRejected, cause: github.ErrForbidden},
			want: "  owner/repo: failed\n    Update request: rejected\n    Desired state: not checked\n\n" +
				rollbackNote,
		},
		{
			name:   "unknown outcome with a matching state",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateUnknown, confirmation: desiredStateMatches},
			want: "  owner/repo: failed\n    Update request: unknown (response unavailable)\n" +
				"    Desired state: matches (update outcome still unknown)\n\nNote: No automatic retry was performed.\n",
		},
		{
			name:   "success with an unconfirmed state",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateUnconfirmed},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not confirmed (read failed)\n\n" +
				rollbackNote,
		},
		{
			name:   "success with a changed repository ID",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateRepositoryChanged},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: not confirmed (repository ID changed)\n\n" +
				rollbackNote,
		},
		{
			name:   "success with a mismatch shows the possible cause after the desired state",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMismatch},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: does not match\n" +
				"    Possible cause: auto-merge is not available for private repositories on GitHub Free, where GitHub may accept the update without applying it\n\n" +
				rollbackNote,
		},
		{
			name: "unmanaged setting change",
			record: changeRecord{
				precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMatches, unexpectedUnmanagedChanges: unmanaged,
			},
			want: "  owner/repo: failed\n    Update request: succeeded\n    Desired state: matches\n" +
				"    Unmanaged settings: general.delete_branch_on_merge changed false -> true (cause not determined)\n\n" +
				rollbackNote,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeChangeReport(&out, target, change, tc.record); err != nil {
				t.Fatalf("writeChangeReport() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangeReport() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}

	t.Run("write failure is returned", func(t *testing.T) {
		err := writeChangeReport(failingWriter{}, target, change, changeRecord{})
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangeReport() error = %v, want %v", err, errWrite)
		}
	})
}
