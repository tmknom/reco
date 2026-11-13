package apply

import (
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-013
func TestChangeRecord_Result(t *testing.T) {
	cases := []struct {
		name   string
		record changeRecord
		want   changeResult
	}{
		{name: "zero value is not attempted", record: changeRecord{}, want: notAttempted},
		{
			name:   "success, match and no other setting change is complete",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMatches},
			want:   completed,
		},
		{
			name:   "success and mismatch is failed",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMismatch},
			want:   failed,
		},
		{
			name: "success, match and other setting change is failed",
			record: changeRecord{
				precondition: preconditionHolds, update: github.UpdateSucceeded, confirmation: desiredStateMatches,
				unexpectedOutsideChanges: []unexpectedOutsideChange{{Key: setting.DeleteBranchOnMerge, Before: changeplan.BoolValue(false), After: changeplan.BoolValue(true)}},
			},
			want: failed,
		},
		{
			name:   "unknown outcome with a match is failed",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateUnknown, confirmation: desiredStateMatches},
			want:   failed,
		},
		{
			name:   "rejected update is failed",
			record: changeRecord{precondition: preconditionHolds, update: github.UpdateRejected},
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
