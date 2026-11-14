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
			want:     []SettingPlan{{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch}},
		},
		{
			name:     "explicit desired false and observed false is a match",
			desired:  configuration.DesiredState{General: configuration.General{AllowAutoMerge: new(false)}},
			observed: github.ObservationResult{General: github.General{AllowAutoMerge: false}},
			want:     []SettingPlan{{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(false), Result: Match}},
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
				{Key: setting.DeleteBranchOnMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match},
				{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
			},
		},
		{
			name: "all six settings follow the configuration order and a match stays in the plan",
			desired: configuration.DesiredState{
				General: configuration.General{AllowAutoMerge: new(true), DeleteBranchOnMerge: new(true)},
				ActionsPermissions: configuration.ActionsPermissions{
					AllowedActions: new(setting.AllowedActionsSelected), SelectedActions: &setting.SelectedActionsCriteria{PatternsAllowed: []string{}},
				},
				WorkflowPermissions: configuration.WorkflowPermissions{
					DefaultWorkflowPermissions: new(setting.PermissionRead), CanApprovePullRequestReviews: new(false),
				},
			},
			observed: github.ObservationResult{
				General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true},
				ActionsPermissions: &github.ActionsPermissions{
					Enabled: true, AllowedActions: github.Observed(setting.AllowedActionsAll), SelectedActions: github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableNotSelected),
				},
				WorkflowPermissions: workflow(setting.PermissionWrite, true),
			},
			want: []SettingPlan{
				{Key: setting.DeleteBranchOnMerge, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match},
				{Key: setting.AllowAutoMerge, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
				{Key: setting.AllowedActions, Observed: github.Observed(setting.AllowedActionsPolicyValue(setting.AllowedActionsAll)), Desired: setting.AllowedActionsPolicyValue(setting.AllowedActionsSelected), Result: Mismatch},
				{
					Key: setting.SelectedActions, Observed: github.Unavailable[setting.Value](github.UnavailableNotSelected),
					Desired: setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{PatternsAllowed: []string{}}), Result: NotComparable,
				},
				{Key: setting.DefaultWorkflowPermissions, Observed: github.Observed(setting.PermissionLevelValue(setting.PermissionWrite)), Desired: setting.PermissionLevelValue(setting.PermissionRead), Result: Mismatch},
				{Key: setting.CanApprovePullRequestReviews, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false), Result: Mismatch},
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

// 期待値と観測値の組を、アーキテクチャ設計の遷移表の行ごとに照合する。
//
//trace:checks FR-005
//trace:checks INV-001
func TestReconcile_ActionsPermissions(t *testing.T) {
	criteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: []string{"a/*", "b/*"}}
	reordered := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: true, PatternsAllowed: []string{"b/*", "a/*"}}
	changed := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: []string{"a/*", "b/*"}}
	policyValue := setting.AllowedActionsPolicyValue
	criteriaValue := setting.SelectedActionsCriteriaValue
	declarePolicy := func(policy setting.AllowedActionsPolicy) configuration.DesiredState {
		return configuration.DesiredState{ActionsPermissions: configuration.ActionsPermissions{AllowedActions: &policy}}
	}
	declareSelected := func(c setting.SelectedActionsCriteria) configuration.DesiredState {
		return configuration.DesiredState{ActionsPermissions: configuration.ActionsPermissions{AllowedActions: new(setting.AllowedActionsSelected), SelectedActions: &c}}
	}
	notSelected := func(policy setting.AllowedActionsPolicy) github.ObservationResult {
		return github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{
			Enabled: true, AllowedActions: github.Observed(policy), SelectedActions: github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableNotSelected),
		}}
	}
	selected := func(c setting.SelectedActionsCriteria) github.ObservationResult {
		return github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{
			Enabled: true, AllowedActions: github.Observed(setting.AllowedActionsSelected), SelectedActions: github.Observed(c),
		}}
	}
	disabled := github.ObservationResult{ActionsPermissions: &github.ActionsPermissions{
		AllowedActions:  github.Unavailable[setting.AllowedActionsPolicy](github.UnavailableActionsDisabled),
		SelectedActions: github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableActionsDisabled),
	}}
	unavailable := github.Unavailable[setting.Value]
	cases := []struct {
		name     string
		desired  configuration.DesiredState
		observed github.ObservationResult
		want     []SettingPlan
	}{
		{
			name:     "all to local_only is a mismatch of the policy",
			desired:  declarePolicy(setting.AllowedActionsLocalOnly),
			observed: notSelected(setting.AllowedActionsAll),
			want:     []SettingPlan{{Key: setting.AllowedActions, Observed: github.Observed(policyValue(setting.AllowedActionsAll)), Desired: policyValue(setting.AllowedActionsLocalOnly), Result: Mismatch}},
		},
		{
			name:     "all to selected is a policy mismatch and criteria not comparable because the policy is not selected",
			desired:  declareSelected(criteria),
			observed: notSelected(setting.AllowedActionsAll),
			want: []SettingPlan{
				{Key: setting.AllowedActions, Observed: github.Observed(policyValue(setting.AllowedActionsAll)), Desired: policyValue(setting.AllowedActionsSelected), Result: Mismatch},
				{Key: setting.SelectedActions, Observed: unavailable(github.UnavailableNotSelected), Desired: criteriaValue(criteria), Result: NotComparable},
			},
		},
		{
			name:     "selected with different criteria is a policy match and a criteria mismatch",
			desired:  declareSelected(criteria),
			observed: selected(changed),
			want: []SettingPlan{
				{Key: setting.AllowedActions, Observed: github.Observed(policyValue(setting.AllowedActionsSelected)), Desired: policyValue(setting.AllowedActionsSelected), Result: Match},
				{Key: setting.SelectedActions, Observed: github.Observed(criteriaValue(changed)), Desired: criteriaValue(criteria), Result: Mismatch},
			},
		},
		{
			name:     "selected to all reconciles only the policy because the criteria are not declared",
			desired:  declarePolicy(setting.AllowedActionsAll),
			observed: selected(criteria),
			want:     []SettingPlan{{Key: setting.AllowedActions, Observed: github.Observed(policyValue(setting.AllowedActionsSelected)), Desired: policyValue(setting.AllowedActionsAll), Result: Mismatch}},
		},
		{
			name:     "disabled to local_only is not comparable because actions is disabled",
			desired:  declarePolicy(setting.AllowedActionsLocalOnly),
			observed: disabled,
			want:     []SettingPlan{{Key: setting.AllowedActions, Observed: unavailable(github.UnavailableActionsDisabled), Desired: policyValue(setting.AllowedActionsLocalOnly), Result: NotComparable}},
		},
		{
			name:     "disabled to selected makes both not comparable because actions is disabled",
			desired:  declareSelected(criteria),
			observed: disabled,
			want: []SettingPlan{
				{Key: setting.AllowedActions, Observed: unavailable(github.UnavailableActionsDisabled), Desired: policyValue(setting.AllowedActionsSelected), Result: NotComparable},
				{Key: setting.SelectedActions, Observed: unavailable(github.UnavailableActionsDisabled), Desired: criteriaValue(criteria), Result: NotComparable},
			},
		},
		{
			name:     "patterns differing only in order are a match",
			desired:  declareSelected(criteria),
			observed: selected(reordered),
			want: []SettingPlan{
				{Key: setting.AllowedActions, Observed: github.Observed(policyValue(setting.AllowedActionsSelected)), Desired: policyValue(setting.AllowedActionsSelected), Result: Match},
				{Key: setting.SelectedActions, Observed: github.Observed(criteriaValue(reordered)), Desired: criteriaValue(criteria), Result: Match},
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
			name:    "allowed_actions alone includes actions permissions",
			desired: configuration.DesiredState{ActionsPermissions: configuration.ActionsPermissions{AllowedActions: new(setting.AllowedActionsAll)}},
			want:    []setting.Area{setting.AreaActionsPermissions},
		},
		{
			name: "each area appears once in the configuration order",
			desired: configuration.DesiredState{
				General: configuration.General{AllowAutoMerge: new(true), DeleteBranchOnMerge: new(true)},
				ActionsPermissions: configuration.ActionsPermissions{
					AllowedActions: new(setting.AllowedActionsSelected), SelectedActions: &setting.SelectedActionsCriteria{},
				},
				WorkflowPermissions: configuration.WorkflowPermissions{
					DefaultWorkflowPermissions: new(setting.PermissionRead), CanApprovePullRequestReviews: new(true),
				},
			},
			want: []setting.Area{setting.AreaGeneral, setting.AreaActionsPermissions, setting.AreaWorkflowPermissions},
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
