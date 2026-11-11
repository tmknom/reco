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

//trace:checks FR-012
func TestSettingPlan_Reconcile(t *testing.T) {
	approved := SettingPlan{Setting: SettingAllowAutoMerge, Current: false, Desired: true, Result: Mismatch, Applicability: Applicable}
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

//trace:checks FR-010
func TestSettingPlan_CurrentIn(t *testing.T) {
	s := SettingPlan{Setting: SettingAllowAutoMerge}
	for _, want := range []bool{true, false} {
		if got := s.CurrentIn(github.General{AllowAutoMerge: want, DeleteBranchOnMerge: !want}); got != want {
			t.Errorf("CurrentIn() = %t, want %t", got, want)
		}
	}
}

func TestSettingPlan_CurrentIn_UnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("CurrentIn() did not panic for an unknown setting")
		}
	}()
	SettingPlan{Setting: "general.unknown"}.CurrentIn(github.General{})
}
