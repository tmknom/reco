package changeplan

import (
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
)

//trace:checks FR-005
func TestReconcile(t *testing.T) {
	cases := []struct {
		name     string
		desired  configuration.General
		observed github.General
		want     []SettingPlan
	}{
		{
			name:     "desired true and observed false is a mismatch",
			desired:  configuration.General{AllowAutoMerge: new(true)},
			observed: github.General{AllowAutoMerge: false},
			want:     []SettingPlan{{Key: "general.allow_auto_merge", Observed: false, Desired: true, Result: Mismatch}},
		},
		{
			name:     "desired false and observed true is a mismatch",
			desired:  configuration.General{AllowAutoMerge: new(false)},
			observed: github.General{AllowAutoMerge: true},
			want:     []SettingPlan{{Key: "general.allow_auto_merge", Observed: true, Desired: false, Result: Mismatch}},
		},
		{
			name:     "explicit desired false and observed false is a match",
			desired:  configuration.General{AllowAutoMerge: new(false)},
			observed: github.General{AllowAutoMerge: false},
			want:     []SettingPlan{{Key: "general.allow_auto_merge", Observed: false, Desired: false, Result: Match}},
		},
		{
			name:     "omitted desired is outside the management scope",
			desired:  configuration.General{},
			observed: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true},
			want:     nil,
		},
		{
			name:     "both settings keep the table order and a match stays in the plan",
			desired:  configuration.General{AllowAutoMerge: new(true), DeleteBranchOnMerge: new(true)},
			observed: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true},
			want: []SettingPlan{
				{Key: "general.delete_branch_on_merge", Observed: true, Desired: true, Result: Match},
				{Key: "general.allow_auto_merge", Observed: false, Desired: true, Result: Mismatch},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reconcile(configuration.DesiredState{General: tc.desired}, tc.observed)
			if !slices.Equal(got, tc.want) {
				t.Errorf("reconcile() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestObservedValue_UnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("observedValue() did not panic for an unknown setting")
		}
	}()
	observedValue("general.unknown", github.General{})
}
