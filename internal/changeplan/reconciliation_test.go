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
			want:     []SettingPlan{{Key: "general.allow_auto_merge", Observed: false, Desired: true, Result: Mismatch}},
		},
		{
			name:     "desired false and observed true is a mismatch",
			desired:  new(false),
			observed: true,
			want:     []SettingPlan{{Key: "general.allow_auto_merge", Observed: true, Desired: false, Result: Mismatch}},
		},
		{
			name:     "explicit desired false and observed false is a match",
			desired:  new(false),
			observed: false,
			want:     []SettingPlan{{Key: "general.allow_auto_merge", Observed: false, Desired: false, Result: Match}},
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
			desired := configuration.DesiredState{General: configuration.General{AllowAutoMerge: tc.desired}}
			got := reconcile(desired, github.General{AllowAutoMerge: tc.observed})
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
