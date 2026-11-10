package reco

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
		want     []settingPlan
	}{
		{
			name:     "desired true and observed false is a mismatch",
			desired:  new(true),
			observed: false,
			want:     []settingPlan{{setting: "general.allow_auto_merge", current: false, desired: true, result: mismatch}},
		},
		{
			name:     "desired false and observed true is a mismatch",
			desired:  new(false),
			observed: true,
			want:     []settingPlan{{setting: "general.allow_auto_merge", current: true, desired: false, result: mismatch}},
		},
		{
			name:     "explicit desired false and observed false is a match",
			desired:  new(false),
			observed: false,
			want:     []settingPlan{{setting: "general.allow_auto_merge", current: false, desired: false, result: match}},
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
