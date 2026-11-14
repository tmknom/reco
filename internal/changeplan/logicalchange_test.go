package changeplan

import (
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

//trace:checks INV-002
//trace:checks FR-008
func TestPlan_LogicalChanges(t *testing.T) {
	match := SettingPlan{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(true), Desired: setting.BoolValue(true), Result: Match}
	mismatch := SettingPlan{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
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
		want := []LogicalChange{{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true)}}
		if !slices.Equal(got, want) {
			t.Errorf("LogicalChanges() = %+v, want %+v", got, want)
		}
	})

	t.Run("applicable mismatches of both settings become logical changes in the plan order", func(t *testing.T) {
		deleteBranch := SettingPlan{Key: setting.DeleteBranchOnMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: Applicable}
		got, err := newPlan(deleteBranch, mismatch).LogicalChanges()
		if err != nil {
			t.Fatalf("LogicalChanges() error = %v", err)
		}
		want := []LogicalChange{
			{Key: setting.DeleteBranchOnMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true)},
			{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true)},
		}
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
			setting: SettingPlan{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: NotApplicable},
		},
		{
			name:    "applicable not-comparable planned change is an error because it has no observed value",
			setting: SettingPlan{Key: setting.AllowAutoMerge, Desired: setting.BoolValue(true), Result: NotComparable, Applicability: Applicable},
		},
		{
			name:    "mismatch without an applicability judgment is an error",
			setting: SettingPlan{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true), Result: Mismatch, Applicability: ApplicabilityNotJudged},
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
