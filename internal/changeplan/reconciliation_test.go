package changeplan

import (
	"reflect"
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-001 FR-005
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
			want:     []SettingPlan{{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch}},
		},
		{
			name:     "explicit desired false and observed false is a match",
			desired:  configuration.DesiredState{General: configuration.General{AllowAutoMerge: new(false)}},
			observed: github.ObservationResult{General: github.General{AllowAutoMerge: false}},
			want:     []SettingPlan{{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(false), Result: Match}},
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
				{Unit: setting.Unit{Key: setting.DeleteBranchOnMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match},
				{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
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
				{Unit: setting.Unit{Key: setting.DeleteBranchOnMerge}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(true), Result: Match},
				{Unit: setting.Unit{Key: setting.AllowAutoMerge}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
				{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Observed(setting.AllowedActionsPolicyValue(setting.AllowedActionsAll)), Desired: setting.AllowedActionsPolicyValue(setting.AllowedActionsSelected), Result: Mismatch},
				{
					Unit: setting.Unit{Key: setting.SelectedActions}, Observed: github.Unavailable[setting.Value](github.UnavailableNotSelected),
					Desired: setting.SelectedActionsCriteriaValue(setting.SelectedActionsCriteria{PatternsAllowed: []string{}}), Result: NotComparable,
				},
				{Unit: setting.Unit{Key: setting.DefaultWorkflowPermissions}, Observed: github.Observed(setting.PermissionLevelValue(setting.PermissionWrite)), Desired: setting.PermissionLevelValue(setting.PermissionRead), Result: Mismatch},
				{Unit: setting.Unit{Key: setting.CanApprovePullRequestReviews}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false), Result: Mismatch},
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

// 期待値と観測値の組を、Actions の設計の遷移表の行ごとに照合する。
//
//trace:checks FR-005 INV-001
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
	declareEnabled := func(enabled bool) configuration.DesiredState {
		return configuration.DesiredState{ActionsPermissions: configuration.ActionsPermissions{Enabled: &enabled}}
	}
	cases := []struct {
		name     string
		desired  configuration.DesiredState
		observed github.ObservationResult
		want     []SettingPlan
	}{
		{
			name:     "enabled true on disabled actions is a mismatch, not not comparable",
			desired:  declareEnabled(true),
			observed: disabled,
			want:     []SettingPlan{{Unit: setting.Unit{Key: setting.ActionsEnabled}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch}},
		},
		{
			name:     "enabled false on enabled actions is a mismatch of the enabled state alone",
			desired:  declareEnabled(false),
			observed: selected(criteria),
			want:     []SettingPlan{{Unit: setting.Unit{Key: setting.ActionsEnabled}, Observed: github.Observed(setting.BoolValue(true)), Desired: setting.BoolValue(false), Result: Mismatch}},
		},
		{
			name:     "enabled false on disabled actions is a match",
			desired:  declareEnabled(false),
			observed: disabled,
			want:     []SettingPlan{{Unit: setting.Unit{Key: setting.ActionsEnabled}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(false), Result: Match}},
		},
		{
			name:     "all to local_only is a mismatch of the policy",
			desired:  declarePolicy(setting.AllowedActionsLocalOnly),
			observed: notSelected(setting.AllowedActionsAll),
			want:     []SettingPlan{{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Observed(policyValue(setting.AllowedActionsAll)), Desired: policyValue(setting.AllowedActionsLocalOnly), Result: Mismatch}},
		},
		{
			name:     "all to selected is a policy mismatch and criteria not comparable because the policy is not selected",
			desired:  declareSelected(criteria),
			observed: notSelected(setting.AllowedActionsAll),
			want: []SettingPlan{
				{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Observed(policyValue(setting.AllowedActionsAll)), Desired: policyValue(setting.AllowedActionsSelected), Result: Mismatch},
				{Unit: setting.Unit{Key: setting.SelectedActions}, Observed: unavailable(github.UnavailableNotSelected), Desired: criteriaValue(criteria), Result: NotComparable},
			},
		},
		{
			name:     "selected with different criteria is a policy match and a criteria mismatch",
			desired:  declareSelected(criteria),
			observed: selected(changed),
			want: []SettingPlan{
				{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Observed(policyValue(setting.AllowedActionsSelected)), Desired: policyValue(setting.AllowedActionsSelected), Result: Match},
				{Unit: setting.Unit{Key: setting.SelectedActions}, Observed: github.Observed(criteriaValue(changed)), Desired: criteriaValue(criteria), Result: Mismatch},
			},
		},
		{
			name:     "selected to all reconciles only the policy because the criteria are not declared",
			desired:  declarePolicy(setting.AllowedActionsAll),
			observed: selected(criteria),
			want:     []SettingPlan{{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Observed(policyValue(setting.AllowedActionsSelected)), Desired: policyValue(setting.AllowedActionsAll), Result: Mismatch}},
		},
		{
			name:     "disabled to local_only is not comparable because actions is disabled",
			desired:  declarePolicy(setting.AllowedActionsLocalOnly),
			observed: disabled,
			want:     []SettingPlan{{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: unavailable(github.UnavailableActionsDisabled), Desired: policyValue(setting.AllowedActionsLocalOnly), Result: NotComparable}},
		},
		{
			name:     "disabled to selected makes both not comparable because actions is disabled",
			desired:  declareSelected(criteria),
			observed: disabled,
			want: []SettingPlan{
				{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: unavailable(github.UnavailableActionsDisabled), Desired: policyValue(setting.AllowedActionsSelected), Result: NotComparable},
				{Unit: setting.Unit{Key: setting.SelectedActions}, Observed: unavailable(github.UnavailableActionsDisabled), Desired: criteriaValue(criteria), Result: NotComparable},
			},
		},
		{
			name:     "patterns differing only in order are a match",
			desired:  declareSelected(criteria),
			observed: selected(reordered),
			want: []SettingPlan{
				{Unit: setting.Unit{Key: setting.AllowedActions}, Observed: github.Observed(policyValue(setting.AllowedActionsSelected)), Desired: policyValue(setting.AllowedActionsSelected), Result: Match},
				{Unit: setting.Unit{Key: setting.SelectedActions}, Observed: github.Observed(criteriaValue(reordered)), Desired: criteriaValue(criteria), Result: Match},
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
			name:    "enabled alone includes actions permissions",
			desired: configuration.DesiredState{ActionsPermissions: configuration.ActionsPermissions{Enabled: new(false)}},
			want:    []setting.Area{setting.AreaActionsPermissions},
		},
		{
			name: "each area appears once",
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
		{
			name:    "environments alone include environments",
			desired: configuration.DesiredState{Environments: []configuration.EnvironmentDeclaration{{Name: mustEnvironmentName(t, "production")}}},
			want:    []setting.Area{setting.AreaEnvironments},
		},
		{
			name: "environments follow the repository settings areas",
			desired: configuration.DesiredState{
				General:      configuration.General{AllowAutoMerge: new(true)},
				Environments: []configuration.EnvironmentDeclaration{{Name: mustEnvironmentName(t, "production")}},
			},
			want: []setting.Area{setting.AreaGeneral, setting.AreaEnvironments},
		},
		{
			name:    "repository variables alone include repository variables without the actions permission areas",
			desired: configuration.DesiredState{RepositoryVariables: []configuration.VariableDeclaration{{Name: mustVariableName(t, "GO_VERSION"), Desired: setting.VariableValue("1.27")}}},
			want:    []setting.Area{setting.AreaRepositoryVariables},
		},
		{
			name:    "dependabot_security_updates alone includes dependabot",
			desired: configuration.DesiredState{Dependabot: configuration.Dependabot{SecurityUpdates: new(true)}},
			want:    []setting.Area{setting.AreaDependabot},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := managedAreas(tc.desired)
			slices.Sort(got)
			slices.Sort(tc.want)
			if !slices.Equal(got, tc.want) {
				t.Errorf("managedAreas() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-005
func TestReconcile_RepositoryVariables(t *testing.T) {
	cases := []struct {
		name         string
		observed     []github.Variable
		desired      string
		wantObserved setting.Value
		wantResult   ReconciliationResult
	}{
		{name: "same value is a match", observed: []github.Variable{{Name: "GO_VERSION", Value: "1.27"}}, desired: "1.27", wantObserved: setting.VariableValue("1.27"), wantResult: Match},
		{name: "different value is a mismatch with the observed value", observed: []github.Variable{{Name: "GO_VERSION", Value: "1.26"}}, desired: "1.27", wantObserved: setting.VariableValue("1.26"), wantResult: Mismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			unit := variableUnit(t, "GO_VERSION")
			desired := configuration.DesiredState{RepositoryVariables: []configuration.VariableDeclaration{{Name: unit.Variable, Desired: setting.VariableValue(tc.desired)}}}
			observed := github.ObservationResult{RepositoryVariables: &github.Variables{Existing: tc.observed}}
			want := []SettingPlan{{Unit: unit, Observed: github.Observed(tc.wantObserved), Desired: setting.VariableValue(tc.desired), Result: tc.wantResult}}
			if got := reconcile(desired, observed); !reflect.DeepEqual(got, want) {
				t.Errorf("reconcile() = %+v, want %+v", got, want)
			}
		})
	}
}

//trace:checks FR-005 INV-001
func TestReconcile_RepositoryVariablesOrder(t *testing.T) {
	desired := configuration.DesiredState{
		ActionsAccess: configuration.ActionsAccess{AccessLevel: new(setting.AccessNone)},
		RepositoryVariables: []configuration.VariableDeclaration{
			{Name: mustVariableName(t, "ZETA"), Desired: setting.VariableValue("z")},
			{Name: mustVariableName(t, "ALPHA"), Desired: setting.VariableValue("a")},
		},
		Environments: []configuration.EnvironmentDeclaration{{Name: mustEnvironmentName(t, "production")}},
	}
	observed := github.ObservationResult{
		ActionsAccess:       &github.ActionsAccess{AccessLevel: github.Observed(setting.AccessNone)},
		RepositoryVariables: &github.Variables{Existing: []github.Variable{{Name: "ALPHA", Value: "a"}, {Name: "UNDECLARED", Value: "u"}}},
		Environments:        &github.Environments{Existing: []github.Environment{{Name: "production"}}},
	}
	var units []string
	for _, s := range reconcile(desired, observed) {
		units = append(units, s.Unit.String())
	}
	// YAML 設定仕様の章の並びで Actions access と Environments のあいだに、宣言の並びで続き、宣言していない変数は現れない
	want := []string{"actions.access.access_level", "actions.variables.ZETA", "actions.variables.ALPHA", "environments.production"}
	if !reflect.DeepEqual(units, want) {
		t.Errorf("reconcile() units = %v, want %v", units, want)
	}
}

//trace:checks FR-001 INV-001
func TestReconcile_Environments(t *testing.T) {
	zeta, alpha := mustEnvironmentName(t, "zeta"), mustEnvironmentName(t, "alpha")
	desired := configuration.DesiredState{
		General:      configuration.General{AllowAutoMerge: new(true)},
		Environments: []configuration.EnvironmentDeclaration{{Name: zeta}, {Name: alpha}},
	}
	observed := github.ObservationResult{
		General:      github.General{AllowAutoMerge: true},
		Environments: &github.Environments{Existing: []github.Environment{{Name: "alpha"}, {Name: "undeclared"}, {Name: "zeta"}}},
	}
	got := reconcile(desired, observed)
	var units []string
	for _, s := range got {
		units = append(units, s.Unit.String())
	}
	// リポジトリ設定のあとに宣言の並びで続き、宣言していない Environment は現れない
	want := []string{"general.allow_auto_merge", "environments.zeta", "environments.alpha"}
	if !reflect.DeepEqual(units, want) {
		t.Errorf("reconcile() units = %v, want %v", units, want)
	}
}

//trace:checks FR-005
func TestReconcile_PrivateVulnerabilityReportingFollowsSecretScanning(t *testing.T) {
	desired := configuration.DesiredState{
		SecretScanning:                configuration.SecretScanning{Enabled: new(true)},
		PrivateVulnerabilityReporting: configuration.PrivateVulnerabilityReporting{Enabled: new(true)},
	}
	observed := github.ObservationResult{
		SecretScanning:                &github.SecretScanning{Enabled: github.Observed(true), PushProtection: github.Observed(true)},
		PrivateVulnerabilityReporting: &github.PrivateVulnerabilityReporting{Enabled: github.Observed(true)},
	}
	var units []string
	for _, s := range reconcile(desired, observed) {
		units = append(units, s.Unit.String())
	}
	// YAML 設定仕様の表の並びで、Secret scanning のあとに Private vulnerability reporting が続く
	want := []string{"security.secret_scanning", "security.private_vulnerability_reporting"}
	if !reflect.DeepEqual(units, want) {
		t.Errorf("reconcile() units = %v, want %v", units, want)
	}
}

//trace:checks FR-005
func TestReconcile_DependabotFollowsEnvironments(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	desired := configuration.DesiredState{
		General:      configuration.General{AllowAutoMerge: new(true)},
		Environments: []configuration.EnvironmentDeclaration{{Name: production}},
		Dependabot:   configuration.Dependabot{Alerts: new(true), SecurityUpdates: new(true)},
	}
	observed := github.ObservationResult{
		General:      github.General{AllowAutoMerge: true},
		Environments: &github.Environments{Existing: []github.Environment{{Name: "production"}}},
		Dependabot:   &github.Dependabot{Alerts: true, SecurityUpdates: setting.DependabotSecurityUpdatesEnabled},
	}
	var units []string
	for _, s := range reconcile(desired, observed) {
		units = append(units, s.Unit.String())
	}
	// YAML 設定仕様の章の並びで、Environments のあとに Security が続く
	want := []string{"general.allow_auto_merge", "environments.production", "security.dependabot_alerts", "security.dependabot_security_updates"}
	if !reflect.DeepEqual(units, want) {
		t.Errorf("reconcile() units = %v, want %v", units, want)
	}
}
