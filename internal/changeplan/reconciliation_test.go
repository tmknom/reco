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
			want:     []SettingPlan{{Key: setting.AllowAutoMerge, Observed: BoolValue(false), Desired: BoolValue(true), Result: Mismatch}},
		},
		{
			name:     "desired false and observed true is a mismatch",
			desired:  configuration.DesiredState{General: configuration.General{AllowAutoMerge: new(false)}},
			observed: github.ObservationResult{General: github.General{AllowAutoMerge: true}},
			want:     []SettingPlan{{Key: setting.AllowAutoMerge, Observed: BoolValue(true), Desired: BoolValue(false), Result: Mismatch}},
		},
		{
			name:     "explicit desired false and observed false is a match",
			desired:  configuration.DesiredState{General: configuration.General{AllowAutoMerge: new(false)}},
			observed: github.ObservationResult{General: github.General{AllowAutoMerge: false}},
			want:     []SettingPlan{{Key: setting.AllowAutoMerge, Observed: BoolValue(false), Desired: BoolValue(false), Result: Match}},
		},
		{
			name:     "omitted desired is outside the management scope",
			desired:  configuration.DesiredState{},
			observed: github.ObservationResult{General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true}, WorkflowPermissions: workflow(setting.PermissionWrite, true)},
			want:     nil,
		},
		{
			name:     "desired read and observed write of default_workflow_permissions is a mismatch",
			desired:  configuration.DesiredState{WorkflowPermissions: configuration.WorkflowPermissions{DefaultWorkflowPermissions: new(setting.PermissionRead)}},
			observed: github.ObservationResult{WorkflowPermissions: workflow(setting.PermissionWrite, false)},
			want: []SettingPlan{{
				Key: setting.DefaultWorkflowPermissions, Observed: PermissionLevelValue(setting.PermissionWrite), Desired: PermissionLevelValue(setting.PermissionRead), Result: Mismatch,
			}},
		},
		{
			name:     "same default_workflow_permissions is a match",
			desired:  configuration.DesiredState{WorkflowPermissions: configuration.WorkflowPermissions{DefaultWorkflowPermissions: new(setting.PermissionWrite)}},
			observed: github.ObservationResult{WorkflowPermissions: workflow(setting.PermissionWrite, true)},
			want: []SettingPlan{{
				Key: setting.DefaultWorkflowPermissions, Observed: PermissionLevelValue(setting.PermissionWrite), Desired: PermissionLevelValue(setting.PermissionWrite), Result: Match,
			}},
		},
		{
			name: "general only is reconciled without workflow permissions observed",
			desired: configuration.DesiredState{General: configuration.General{
				AllowAutoMerge: new(true), DeleteBranchOnMerge: new(true),
			}},
			observed: github.ObservationResult{General: github.General{AllowAutoMerge: false, DeleteBranchOnMerge: true}},
			want: []SettingPlan{
				{Key: setting.DeleteBranchOnMerge, Observed: BoolValue(true), Desired: BoolValue(true), Result: Match},
				{Key: setting.AllowAutoMerge, Observed: BoolValue(false), Desired: BoolValue(true), Result: Mismatch},
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
				{Key: setting.DeleteBranchOnMerge, Observed: BoolValue(true), Desired: BoolValue(true), Result: Match},
				{Key: setting.AllowAutoMerge, Observed: BoolValue(false), Desired: BoolValue(true), Result: Mismatch},
				{Key: setting.DefaultWorkflowPermissions, Observed: PermissionLevelValue(setting.PermissionWrite), Desired: PermissionLevelValue(setting.PermissionRead), Result: Mismatch},
				{Key: setting.CanApprovePullRequestReviews, Observed: BoolValue(true), Desired: BoolValue(false), Result: Mismatch},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reconcile(tc.desired, tc.observed)
			if !slices.Equal(got, tc.want) {
				t.Errorf("reconcile() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestReconcile_EveryDeclaredSettingIsReconciled は、期待状態が持つ期待値の数と、照合する設定の数が食い違わないことを確かめる。
// 期待状態に設定を足して setting.Keys や desiredValue に足し忘れると、宣言を受理した設定が照合されないまま
// コンパイルもほかのテストも通るため、期待状態の全フィールドを埋めて数える。
func TestReconcile_EveryDeclaredSettingIsReconciled(t *testing.T) {
	var desired configuration.DesiredState
	declared := fillDesiredValues(reflect.ValueOf(&desired).Elem())
	observed := github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead}}

	got := reconcile(desired, observed)
	keys := make([]setting.Key, len(got))
	for i, s := range got {
		keys[i] = s.Key
	}
	if len(keys) != declared || !slices.Equal(keys, setting.Keys()) {
		t.Errorf("reconcile() keys = %v for %d declared settings, want %v", keys, declared, setting.Keys())
	}
}

// fillDesiredValues は、v の中の期待値 (ポインタのフィールド) をすべてゼロ値への参照で埋め、その数を返す。
// 列挙値のゼロ値は定義外の値だが、ここでは期待値の有無だけを数える。
func fillDesiredValues(v reflect.Value) int {
	n := 0
	for _, field := range v.Fields() {
		switch field.Kind() {
		case reflect.Pointer:
			field.Set(reflect.New(field.Type().Elem()))
			n++
		case reflect.Struct:
			n += fillDesiredValues(field)
		default:
			panic("unexpected field kind in desired state: " + field.Kind().String())
		}
	}
	return n
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

func TestObservedValue_UnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("ObservedValue() did not panic for an unknown setting")
		}
	}()
	ObservedValue("general.unknown", github.ObservationResult{})
}

func TestDesiredValue_UnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("desiredValue() did not panic for an unknown setting")
		}
	}()
	desiredValue("general.unknown", configuration.DesiredState{})
}
