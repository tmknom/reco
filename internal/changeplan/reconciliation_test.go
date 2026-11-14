package changeplan

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-001
//trace:checks FR-005
func TestReconcile(t *testing.T) {
	workflow := func(level setting.PermissionLevel, canApprove bool) *github.WorkflowPermissions {
		return &github.WorkflowPermissions{DefaultWorkflowPermissions: level, CanApprovePullRequestReviews: canApprove}
	}
	cases := []struct {
		name     string
		desired  configuration.DesiredState
		observed github.ObservationResult
		want     []SettingPlan
	}{
		{
			name:     "desired true and observed false is a mismatch",
			desired:  configuration.DesiredState{General: configuration.General{AllowAutoMerge: new(true)}},
			observed: github.ObservationResult{General: github.General{AllowAutoMerge: false}},
			want:     []SettingPlan{{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true), Result: Mismatch}},
		},
		{
			name:     "explicit desired false and observed false is a match",
			desired:  configuration.DesiredState{General: configuration.General{AllowAutoMerge: new(false)}},
			observed: github.ObservationResult{General: github.General{AllowAutoMerge: false}},
			want:     []SettingPlan{{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(false), Result: Match}},
		},
		{
			name:     "omitted desired is outside the management scope",
			desired:  configuration.DesiredState{},
			observed: github.ObservationResult{General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true}, WorkflowPermissions: workflow(setting.PermissionWrite, true)},
			want:     nil,
		},
		{
			name: "general only is reconciled without workflow permissions observed",
			desired: configuration.DesiredState{General: configuration.General{
				AllowAutoMerge: new(true), DeleteBranchOnMerge: new(true),
			}},
			observed: github.ObservationResult{General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true}},
			want: []SettingPlan{
				{Key: setting.DeleteBranchOnMerge, Observed: setting.BoolValue(true), Desired: setting.BoolValue(true), Result: Match},
				{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true), Result: Mismatch},
			},
		},
		{
			name: "all four settings follow the configuration order and a match stays in the plan",
			desired: configuration.DesiredState{
				General: configuration.General{AllowAutoMerge: new(true), DeleteBranchOnMerge: new(true)},
				WorkflowPermissions: configuration.WorkflowPermissions{
					DefaultWorkflowPermissions: new(setting.PermissionRead), CanApprovePullRequestReviews: new(false),
				},
			},
			observed: github.ObservationResult{
				General:             github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true},
				WorkflowPermissions: workflow(setting.PermissionWrite, true),
			},
			want: []SettingPlan{
				{Key: setting.DeleteBranchOnMerge, Observed: setting.BoolValue(true), Desired: setting.BoolValue(true), Result: Match},
				{Key: setting.AllowAutoMerge, Observed: setting.BoolValue(false), Desired: setting.BoolValue(true), Result: Mismatch},
				{Key: setting.DefaultWorkflowPermissions, Observed: setting.PermissionLevelValue(setting.PermissionWrite), Desired: setting.PermissionLevelValue(setting.PermissionRead), Result: Mismatch},
				{Key: setting.CanApprovePullRequestReviews, Observed: setting.BoolValue(true), Desired: setting.BoolValue(false), Result: Mismatch},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reconcile(tc.desired, tc.observed)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("reconcile() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-004
func TestManagedAreas(t *testing.T) {
	cases := []struct {
		name    string
		desired configuration.DesiredState
		want    []setting.Area
	}{
		{
			name:    "general only does not include workflow permissions",
			desired: configuration.DesiredState{General: configuration.General{AllowAutoMerge: new(true)}},
			want:    []setting.Area{setting.AreaGeneral},
		},
		{
			name:    "can_approve_pull_request_reviews alone includes workflow permissions",
			desired: configuration.DesiredState{WorkflowPermissions: configuration.WorkflowPermissions{CanApprovePullRequestReviews: new(true)}},
			want:    []setting.Area{setting.AreaWorkflowPermissions},
		},
		{
			name: "each area appears once in the configuration order",
			desired: configuration.DesiredState{
				General: configuration.General{AllowAutoMerge: new(true), DeleteBranchOnMerge: new(true)},
				WorkflowPermissions: configuration.WorkflowPermissions{
					DefaultWorkflowPermissions: new(setting.PermissionRead), CanApprovePullRequestReviews: new(true),
				},
			},
			want: []setting.Area{setting.AreaGeneral, setting.AreaWorkflowPermissions},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := managedAreas(tc.desired); !slices.Equal(got, tc.want) {
				t.Errorf("managedAreas() = %v, want %v", got, tc.want)
			}
		})
	}
}
