package changeplan

import (
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks INV-002
//trace:checks FR-008
func TestPlan_LogicalChanges(t *testing.T) {
	match := SettingPlan{Key: setting.AllowAutoMerge, Observed: true, Desired: true, Result: Match}
	mismatch := SettingPlan{Key: setting.AllowAutoMerge, Observed: false, Desired: true, Result: Mismatch, Applicability: Applicable}
	newPlan := func(settings ...SettingPlan) Plan {
		return Plan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
	}

	t.Run("all matches have no logical change", func(t *testing.T) {
		got, err := newPlan(match).LogicalChanges()
		if err != nil || len(got) != 0 {
			t.Errorf("LogicalChanges() = %+v, error = %v, want none without error", got, err)
		}
	})

	t.Run("applicable mismatch becomes a logical change with the observed and desired values", func(t *testing.T) {
		got, err := newPlan(match, mismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{{Key: setting.AllowAutoMerge, Observed: false, Desired: true}}
		if !slices.Equal(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	errorCases := []struct {
		name    string
		setting SettingPlan
	}{
		{
			name:    "not-applicable mismatch is an error",
			setting: SettingPlan{Key: setting.AllowAutoMerge, Observed: false, Desired: true, Result: Mismatch, Applicability: NotApplicable},
		},
		{
			name:    "applicable not-comparable planned change is an error because it has no observed value",
			setting: SettingPlan{Key: setting.AllowAutoMerge, Desired: true, Result: NotComparable, Applicability: Applicable},
		},
		{
			name:    "mismatch without an applicability judgment is an error",
			setting: SettingPlan{Key: setting.AllowAutoMerge, Observed: false, Desired: true, Result: Mismatch, Applicability: ApplicabilityNotJudged},
		},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			// 適用可能な変更予定と並べ、ほかの変更予定があっても承認の対象にしないことを確かめる
			if got, err := newPlan(mismatch, tc.setting).LogicalChanges(); err == nil {
				t.Errorf("LogicalChanges() = %+v, error = nil, want an error", got)
			}
		})
	}
}

//trace:checks FR-012
func TestLogicalChange_Reconcile(t *testing.T) {
	approved := LogicalChange{Key: setting.AllowAutoMerge, Observed: false, Desired: true}
	cases := []struct {
		name     string
		observed github.General
		want     ReconciliationResult
	}{
		{name: "observed value equal to the desired value is a match", observed: github.General{AllowAutoMerge: true}, want: Match},
		{name: "observed value different from the desired value is a mismatch", observed: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true}, want: Mismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := approved.Reconcile(tc.observed); got != tc.want {
				t.Errorf("Reconcile() = %v, want %v", got, tc.want)
			}
		})
	}
}
