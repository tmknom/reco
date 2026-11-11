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
		desired  *bool
		observed bool
		want     []SettingPlan
	}{
		{
			name:     "desired true and observed false is a mismatch",
			desired:  new(true),
			observed: false,
			want:     []SettingPlan{{Setting: "general.allow_auto_merge", Current: false, Desired: true, Result: Mismatch}},
		},
		{
			name:     "desired false and observed true is a mismatch",
			desired:  new(false),
			observed: true,
			want:     []SettingPlan{{Setting: "general.allow_auto_merge", Current: true, Desired: false, Result: Mismatch}},
		},
		{
			name:     "explicit desired false and observed false is a match",
			desired:  new(false),
			observed: false,
			want:     []SettingPlan{{Setting: "general.allow_auto_merge", Current: false, Desired: false, Result: Match}},
		},
		{
			name:     "omitted desired is outside the management scope",
			desired:  nil,
			observed: false,
			want:     nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reconcile(configuration.General{AllowAutoMerge: tc.desired}, github.General{AllowAutoMerge: tc.observed})
			if !slices.Equal(got, tc.want) {
				t.Errorf("reconcile() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
