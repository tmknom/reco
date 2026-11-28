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
		return &github.WorkflowPermissions{DefaultWorkflowPermissions: github.Observed(level), CanApprovePullRequestReviews: canApprove}
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

// TestNewSettingPlan_OIDCSubject は、期待値が省略できる項目を省略したとき、計画の観測結果がその項目を除いた値になり、
// その項目の違いでは不一致にならず、宣言したときは観測値のまま比べることを確かめる。
//
//trace:checks FR-006
func TestNewSettingPlan_OIDCSubject(t *testing.T) {
	unit := setting.Unit{Key: setting.OIDCSubject}
	cases := []struct {
		name         string
		observed     setting.OIDCSubjectCustomization
		desired      setting.OIDCSubjectCustomization
		wantObserved setting.OIDCSubjectCustomization
		wantResult   ReconciliationResult
	}{
		{
			name:         "omitted immutable subject is dropped from the observation",
			observed:     setting.OIDCSubjectCustomization{UseDefault: true, UseImmutableSubject: new(true)},
			desired:      setting.OIDCSubjectCustomization{UseDefault: true},
			wantObserved: setting.OIDCSubjectCustomization{UseDefault: true},
			wantResult:   Match,
		},
		{
			name:         "declared immutable subject keeps the observed value",
			observed:     setting.OIDCSubjectCustomization{UseDefault: true, UseImmutableSubject: new(false)},
			desired:      setting.OIDCSubjectCustomization{UseDefault: true, UseImmutableSubject: new(true)},
			wantObserved: setting.OIDCSubjectCustomization{UseDefault: true, UseImmutableSubject: new(false)},
			wantResult:   Mismatch,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := newSettingPlan(unit, github.Observed(setting.OIDCSubjectCustomizationValue(tc.observed)), setting.OIDCSubjectCustomizationValue(tc.desired))
			want := SettingPlan{
				Unit: unit, Observed: github.Observed(setting.OIDCSubjectCustomizationValue(tc.wantObserved)),
				Desired: setting.OIDCSubjectCustomizationValue(tc.desired), Result: tc.wantResult,
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("newSettingPlan() = %+v, want %+v", got, want)
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
			name: "environments follow the repository settings areas",
			desired: configuration.DesiredState{
				General:      configuration.General{AllowAutoMerge: new(true)},
				Environments: []configuration.EnvironmentDeclaration{{Name: mustEnvironmentName(t, "production")}},
			},
			want: []setting.Area{setting.AreaGeneral, setting.AreaEnvironments},
		},
		{
			name: "environment with variables includes environments and environment variables",
			desired: configuration.DesiredState{Environments: []configuration.EnvironmentDeclaration{
				{Name: mustEnvironmentName(t, "staging")},
				{Name: mustEnvironmentName(t, "production"), Variables: []configuration.VariableDeclaration{{Name: mustVariableName(t, "REGION"), Desired: setting.VariableValue("eu")}}},
			}},
			want: []setting.Area{setting.AreaEnvironments, setting.AreaEnvironmentVariables},
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

//trace:checks FR-004
func TestManagedEnvironments(t *testing.T) {
	production, staging, qa := mustEnvironmentName(t, "production"), mustEnvironmentName(t, "staging"), mustEnvironmentName(t, "qa")
	protected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}
	region := []configuration.VariableDeclaration{{Name: mustVariableName(t, "REGION"), Desired: setting.VariableValue("eu")}}
	cases := []struct {
		name    string
		desired configuration.DesiredState
		want    []setting.EnvironmentName
	}{
		{
			name: "no environment declared names no environment",
			desired: configuration.DesiredState{
				General:             configuration.General{AllowAutoMerge: new(true)},
				RepositoryVariables: region,
			},
		},
		{
			name: "each declared environment is named once whether it declares a name only, a mode, or variables",
			desired: configuration.DesiredState{Environments: []configuration.EnvironmentDeclaration{
				{Name: production, DeploymentBranchPolicy: &protected, Variables: region},
				{Name: staging},
				{Name: qa, Variables: region},
			}},
			want: []setting.EnvironmentName{production, staging, qa},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := managedEnvironments(tc.desired); !slices.Equal(got, tc.want) {
				t.Errorf("managedEnvironments() = %v, want %v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-004
func TestManagedRulesets(t *testing.T) {
	protect, release := mustRulesetName(t, "protect"), mustRulesetName(t, "release")
	value := setting.RulesetValue(protectRuleset())
	cases := []struct {
		name    string
		desired configuration.DesiredState
		want    []setting.RulesetName
	}{
		{
			name: "no ruleset declared names no ruleset even with environments",
			desired: configuration.DesiredState{
				General:      configuration.General{AllowAutoMerge: new(true)},
				Environments: []configuration.EnvironmentDeclaration{{Name: mustEnvironmentName(t, "production")}},
			},
		},
		{
			name:    "declared rulesets are named in the declared order",
			desired: configuration.DesiredState{Rulesets: []configuration.RulesetDeclaration{{Name: release, Desired: value}, {Name: protect, Desired: value}}},
			want:    []setting.RulesetName{release, protect},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := managedRulesets(tc.desired); !slices.Equal(got, tc.want) {
				t.Errorf("managedRulesets() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestReconcile_Rulesets は、宣言した Ruleset を同じ名前の観測と照合し、解釈しない部分を理由に比較不能な計画にだけ解釈しなかったものを持たせることを確かめる。
//
//trace:checks FR-005 INV-001
func TestReconcile_Rulesets(t *testing.T) {
	unit := rulesetUnit(t, "protect")
	desired := setting.RulesetValue(protectRuleset())
	evaluate := protectRuleset()
	evaluate.Enforcement = setting.RulesetEnforcementEvaluate
	parts := []github.UninterpretedPart{{Kind: github.UninterpretedRuleType, Name: "required_status_checks"}}
	observed := func(rulesets ...github.Ruleset) *github.Rulesets {
		return &github.Rulesets{Existing: github.Observed(rulesets)}
	}
	cases := []struct {
		name     string
		rulesets *github.Rulesets
		desired  setting.Value
		want     SettingPlan
	}{
		{
			name:     "absent ruleset is a mismatch with its absence",
			rulesets: observed(github.InterpretedRuleset(1, "release", protectRuleset())),
			desired:  desired,
			want:     SettingPlan{Unit: unit, Observed: github.Observed(setting.Absent()), Desired: desired, Result: Mismatch},
		},
		{
			name:     "same value under a name differing only in letter case is a match based on its ID",
			rulesets: observed(github.InterpretedRuleset(1, "Protect", protectRuleset())),
			desired:  desired,
			want:     SettingPlan{Unit: unit, Observed: github.Observed(desired), RulesetIdentity: ObservedRulesetIdentity(1), Desired: desired, Result: Match},
		},
		{
			name:     "value that the declaration cannot write is a mismatch with the observed value and ID",
			rulesets: observed(github.InterpretedRuleset(1, "protect", evaluate)),
			desired:  desired,
			want:     SettingPlan{Unit: unit, Observed: github.Observed(setting.RulesetValue(evaluate)), RulesetIdentity: ObservedRulesetIdentity(1), Desired: desired, Result: Mismatch},
		},
		{
			name:     "ruleset with uninterpreted parts is not comparable and carries the parts and its ID",
			rulesets: observed(github.UninterpretedRuleset(1, "protect", parts)),
			desired:  desired,
			want: SettingPlan{
				Unit: unit, Observed: github.Unavailable[setting.Value](github.UnavailableUninterpretedParts), RulesetIdentity: ObservedRulesetIdentity(1), Desired: desired, Result: NotComparable,
				UninterpretedParts: parts,
			},
		},
		{
			name:     "private repository is not comparable without uninterpreted parts",
			rulesets: &github.Rulesets{Existing: github.Unavailable[[]github.Ruleset](github.UnavailablePrivateRepository)},
			desired:  desired,
			want:     SettingPlan{Unit: unit, Observed: github.Unavailable[setting.Value](github.UnavailablePrivateRepository), Desired: desired, Result: NotComparable},
		},
		{
			name:     "declared absence of a ruleset under a name differing only in letter case is a mismatch with the whole observed value and ID",
			rulesets: observed(github.InterpretedRuleset(1, "Protect", evaluate)),
			desired:  setting.Absent(),
			want:     SettingPlan{Unit: unit, Observed: github.Observed(setting.RulesetValue(evaluate)), RulesetIdentity: ObservedRulesetIdentity(1), Desired: setting.Absent(), Result: Mismatch},
		},
		{
			name:     "declared absence with only another name listed is a match with absence",
			rulesets: observed(github.InterpretedRuleset(1, "release", protectRuleset())),
			desired:  setting.Absent(),
			want:     SettingPlan{Unit: unit, Observed: github.Observed(setting.Absent()), Desired: setting.Absent(), Result: Match},
		},
		{
			name:     "declared absence with only a name that cannot be compared is not comparable",
			rulesets: observed(github.InterpretedRuleset(1, "Prot\u00e9ct", protectRuleset())),
			desired:  setting.Absent(),
			want:     SettingPlan{Unit: unit, Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined), Desired: setting.Absent(), Result: NotComparable},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := configuration.DesiredState{Rulesets: []configuration.RulesetDeclaration{{Name: unit.Ruleset, Desired: tc.desired}}}
			got := reconcile(state, github.ObservationResult{Rulesets: tc.rulesets})
			if want := []SettingPlan{tc.want}; !reflect.DeepEqual(got, want) {
				t.Errorf("reconcile() = %+v, want %+v", got, want)
			}
		})
	}
}

//trace:checks FR-005 INV-001
func TestReconcile_RulesetsOrder(t *testing.T) {
	value := setting.RulesetValue(protectRuleset())
	desired := configuration.DesiredState{
		Rulesets: []configuration.RulesetDeclaration{
			{Name: mustRulesetName(t, "zeta"), Desired: value},
			{Name: mustRulesetName(t, "alpha"), Desired: value},
		},
		Environments:                  []configuration.EnvironmentDeclaration{{Name: mustEnvironmentName(t, "production")}},
		PrivateVulnerabilityReporting: configuration.PrivateVulnerabilityReporting{Enabled: new(true)},
	}
	observed := github.ObservationResult{
		Environments:                  &github.Environments{Existing: []github.Environment{{Name: "production"}}},
		PrivateVulnerabilityReporting: &github.PrivateVulnerabilityReporting{Enabled: github.Observed(true)},
		Rulesets: &github.Rulesets{Existing: github.Observed([]github.Ruleset{
			github.InterpretedRuleset(1, "undeclared", protectRuleset()), github.InterpretedRuleset(2, "alpha", protectRuleset()),
		})},
	}
	var units []string
	for _, s := range reconcile(desired, observed) {
		units = append(units, s.Unit.String())
	}
	// YAML 設定仕様の章の並びで Security のあとに、宣言の並びで続き、宣言していない Ruleset は現れない
	want := []string{"environments.production", "security.private_vulnerability_reporting", "rulesets.zeta", "rulesets.alpha"}
	if !reflect.DeepEqual(units, want) {
		t.Errorf("reconcile() units = %v, want %v", units, want)
	}
}

//trace:checks FR-005
func TestReconcile_RepositoryVariables(t *testing.T) {
	cases := []struct {
		name         string
		observed     []github.Variable
		desired      setting.Value
		wantObserved setting.Value
		wantResult   ReconciliationResult
	}{
		{name: "same value is a match", observed: []github.Variable{{Name: "GO_VERSION", Value: "1.27"}}, desired: setting.VariableValue("1.27"), wantObserved: setting.VariableValue("1.27"), wantResult: Match},
		{name: "different value is a mismatch with the observed value", observed: []github.Variable{{Name: "GO_VERSION", Value: "1.26"}}, desired: setting.VariableValue("1.27"), wantObserved: setting.VariableValue("1.26"), wantResult: Mismatch},
		{name: "declared absence of an existing variable is a mismatch with the observed value", observed: []github.Variable{{Name: "GO_VERSION", Value: "1.26"}}, desired: setting.Absent(), wantObserved: setting.VariableValue("1.26"), wantResult: Mismatch},
		{name: "declared absence of an absent variable is a match", observed: []github.Variable{{Name: "OTHER", Value: "x"}}, desired: setting.Absent(), wantObserved: setting.Absent(), wantResult: Match},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			unit := variableUnit(t, "GO_VERSION")
			desired := configuration.DesiredState{RepositoryVariables: []configuration.VariableDeclaration{{Name: unit.Variable, Desired: tc.desired}}}
			observed := github.ObservationResult{RepositoryVariables: &github.Variables{Existing: tc.observed}}
			want := []SettingPlan{{Unit: unit, Observed: github.Observed(tc.wantObserved), Desired: tc.desired, Result: tc.wantResult}}
			if got := reconcile(desired, observed); !reflect.DeepEqual(got, want) {
				t.Errorf("reconcile() = %+v, want %+v", got, want)
			}
		})
	}
}

//trace:checks FR-005 INV-001
func TestReconcile_RepositoryVariablesOrder(t *testing.T) {
	desired := configuration.DesiredState{
		ActionsAccess:           configuration.ActionsAccess{AccessLevel: new(setting.AccessNone)},
		ForkPullRequestApproval: configuration.ForkPullRequestApproval{ApprovalPolicy: new(setting.ForkPullRequestApprovalFirstTimeContributors)},
		RepositoryVariables: []configuration.VariableDeclaration{
			{Name: mustVariableName(t, "ZETA"), Desired: setting.VariableValue("z")},
			{Name: mustVariableName(t, "ALPHA"), Desired: setting.VariableValue("a")},
		},
		Environments: []configuration.EnvironmentDeclaration{{Name: mustEnvironmentName(t, "production")}},
	}
	observed := github.ObservationResult{
		ActionsAccess:           &github.ActionsAccess{AccessLevel: github.Observed(setting.AccessNone)},
		ForkPullRequestApproval: &github.ForkPullRequestApproval{ApprovalPolicy: github.Observed(setting.ForkPullRequestApprovalFirstTimeContributors)},
		RepositoryVariables:     &github.Variables{Existing: []github.Variable{{Name: "ALPHA", Value: "a"}, {Name: "UNDECLARED", Value: "u"}}},
		Environments:            &github.Environments{Existing: []github.Environment{{Name: "production"}}},
	}
	var units []string
	for _, s := range reconcile(desired, observed) {
		units = append(units, s.Unit.String())
	}
	// YAML 設定仕様の章の並びで Fork pull requests と Environments のあいだに、宣言の並びで続き、宣言していない変数は現れない
	want := []string{"actions.access.access_level", "actions.fork_pull_requests.approval_policy", "actions.variables.ZETA", "actions.variables.ALPHA", "environments.production"}
	if !reflect.DeepEqual(units, want) {
		t.Errorf("reconcile() units = %v, want %v", units, want)
	}
}

// TestReconcile_EnvironmentVariablesOrder は、Environment Variable の計画が、設定の領域の順ではなく、属する Environment の計画の直後に並ぶことを確かめる。
//
//trace:checks FR-005 FR-007
func TestReconcile_EnvironmentVariablesOrder(t *testing.T) {
	production, staging := mustEnvironmentName(t, "production"), mustEnvironmentName(t, "staging")
	protected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}
	region := mustVariableName(t, "REGION")
	desired := configuration.DesiredState{
		RepositoryVariables: []configuration.VariableDeclaration{{Name: mustVariableName(t, "GO_VERSION"), Desired: setting.VariableValue("1.27")}},
		Environments: []configuration.EnvironmentDeclaration{
			{Name: production, DeploymentBranchPolicy: &protected, Variables: []configuration.VariableDeclaration{{Name: region, Desired: setting.VariableValue("eu")}}},
			{Name: staging, Variables: []configuration.VariableDeclaration{{Name: region, Desired: setting.VariableValue("us")}}},
		},
		Dependabot: configuration.Dependabot{Alerts: new(true)},
	}
	observed := github.ObservationResult{
		RepositoryVariables: &github.Variables{},
		Environments:        &github.Environments{Existing: []github.Environment{withVariables(github.Environment{Name: "production", DeploymentBranchPolicyMode: protected.Mode})}},
		Dependabot:          &github.Dependabot{Alerts: true},
	}
	var units []string
	for _, s := range reconcile(desired, observed) {
		units = append(units, s.Unit.String())
	}
	want := []string{
		"actions.variables.GO_VERSION",
		"environments.production", "environments.production.deployment_branch_policy", "environments.production.variables.REGION",
		"environments.staging", "environments.staging.variables.REGION",
		"security.dependabot_alerts",
	}
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

//trace:checks FR-001 FR-005
func TestReconcile_EnvironmentSettings(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	protected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}
	all := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}
	existing := func(name string, mode setting.DeploymentBranchPolicyMode) github.Environment {
		return github.Environment{ID: 7, Name: name, DeploymentBranchPolicyMode: mode, Protection: unprotected()}
	}
	region, debug, level := mustVariableName(t, "REGION"), mustVariableName(t, "DEBUG"), mustVariableName(t, "LEVEL")
	existence := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	matchedExistence := SettingPlan{Unit: existence, Observed: github.Observed(setting.Present()), Desired: setting.Present(), EnvironmentIdentity: ObservedEnvironmentIdentity(7), Result: Match}
	creation := SettingPlan{Unit: existence, Observed: github.Observed(setting.Absent()), Desired: setting.Present(), Result: Mismatch}
	undeterminedExistence := SettingPlan{
		Unit: existence, Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined), Desired: setting.Present(), Result: NotComparable,
	}
	reviewersUnit := setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: production}
	user1, user2 := setting.Reviewer{Type: setting.ReviewerUser, ID: 1}, setting.Reviewer{Type: setting.ReviewerUser, ID: 2}
	declaredReviewers := setting.ReviewerSetOf([]setting.Reviewer{user1, user2})
	noReviewers := setting.ReviewerSetOf(nil)
	observedRule := setting.RequiredReviewers{Reviewers: []setting.Reviewer{user2, user1}, Logins: map[setting.Reviewer]string{user1: "octocat"}}
	withProtection := func(e github.Environment, rule setting.RequiredReviewers) github.Environment {
		e.Protection = protectedByRule(rule, false)
		return e
	}
	cases := []struct {
		name     string
		declared configuration.EnvironmentDeclaration
		// observed は、Environment の一覧の全ページを読めた観測で確認できた Environment である。
		observed []github.Environment
		want     []SettingPlan
	}{
		{
			name:     "existing environment declared by name only matches",
			declared: configuration.EnvironmentDeclaration{Name: production},
			observed: []github.Environment{existing("production", all.Mode)},
			want:     []SettingPlan{matchedExistence},
		},
		{
			name:     "existing environment with the declared mode matches both",
			declared: configuration.EnvironmentDeclaration{Name: production, DeploymentBranchPolicy: &protected},
			observed: []github.Environment{existing("production", protected.Mode)},
			want: []SettingPlan{
				matchedExistence,
				{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(setting.DeploymentBranchPolicyValue(protected)), Desired: setting.DeploymentBranchPolicyValue(protected), Result: Match},
			},
		},
		{
			name:     "absent environment declared by name only is a creation",
			declared: configuration.EnvironmentDeclaration{Name: production},
			observed: []github.Environment{existing("staging", all.Mode)},
			want:     []SettingPlan{creation},
		},
		{
			name:     "absent environment with a mode is a creation and a not comparable mode",
			declared: configuration.EnvironmentDeclaration{Name: production, DeploymentBranchPolicy: &protected},
			observed: nil,
			want: []SettingPlan{
				creation,
				{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.DeploymentBranchPolicyValue(protected), Result: NotComparable},
			},
		},
		{
			name:     "environment whose existence cannot be determined is not comparable",
			declared: configuration.EnvironmentDeclaration{Name: production, DeploymentBranchPolicy: &protected},
			observed: []github.Environment{existing("prodüction", all.Mode)},
			want: []SettingPlan{
				undeterminedExistence,
				{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined), Desired: setting.DeploymentBranchPolicyValue(protected), Result: NotComparable},
			},
		},
		{
			name: "variables of an existing environment are matched against its list after the mode in the declared order",
			declared: configuration.EnvironmentDeclaration{Name: production, DeploymentBranchPolicy: &protected, Variables: []configuration.VariableDeclaration{
				{Name: region, Desired: setting.VariableValue("eu")},
				{Name: debug, Desired: setting.VariableValue("true")},
				{Name: level, Desired: setting.VariableValue("info")},
			}},
			observed: []github.Environment{withVariables(existing("production", protected.Mode), github.Variable{Name: "LEVEL", Value: "info"}, github.Variable{Name: "REGION", Value: "us"}, github.Variable{Name: "UNDECLARED", Value: "x"})},
			want: []SettingPlan{
				matchedExistence,
				{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(setting.DeploymentBranchPolicyValue(protected)), Desired: setting.DeploymentBranchPolicyValue(protected), Result: Match},
				{Unit: environmentVariableUnit(production, region), Observed: github.Observed(setting.VariableValue("us")), Desired: setting.VariableValue("eu"), Result: Mismatch},
				{Unit: environmentVariableUnit(production, debug), Observed: github.Observed(setting.Absent()), Desired: setting.VariableValue("true"), Result: Mismatch},
				{Unit: environmentVariableUnit(production, level), Observed: github.Observed(setting.VariableValue("info")), Desired: setting.VariableValue("info"), Result: Match},
			},
		},
		{
			name:     "variable of an absent environment is not comparable because the environment is absent",
			declared: configuration.EnvironmentDeclaration{Name: production, Variables: []configuration.VariableDeclaration{{Name: region, Desired: setting.VariableValue("eu")}}},
			observed: nil,
			want: []SettingPlan{
				creation,
				{Unit: environmentVariableUnit(production, region), Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.VariableValue("eu"), Result: NotComparable},
			},
		},
		{
			name: "declared absences in an existing environment are a mismatch for an existing variable and a match for an absent one",
			declared: configuration.EnvironmentDeclaration{Name: production, Variables: []configuration.VariableDeclaration{
				{Name: region, Desired: setting.Absent()},
				{Name: debug, Desired: setting.Absent()},
			}},
			observed: []github.Environment{withVariables(existing("production", all.Mode), github.Variable{Name: "REGION", Value: "us"})},
			want: []SettingPlan{
				matchedExistence,
				{Unit: environmentVariableUnit(production, region), Observed: github.Observed(setting.VariableValue("us")), Desired: setting.Absent(), Result: Mismatch},
				{Unit: environmentVariableUnit(production, debug), Observed: github.Observed(setting.Absent()), Desired: setting.Absent(), Result: Match},
			},
		},
		{
			name: "reviewers of an existing environment follow the mode and precede the variables and match regardless of order and logins",
			declared: configuration.EnvironmentDeclaration{
				Name: production, DeploymentBranchPolicy: &protected, RequiredReviewers: &declaredReviewers,
				Variables: []configuration.VariableDeclaration{{Name: region, Desired: setting.VariableValue("eu")}},
			},
			observed: []github.Environment{withVariables(withProtection(existing("production", protected.Mode), observedRule), github.Variable{Name: "REGION", Value: "eu"})},
			want: []SettingPlan{
				matchedExistence,
				{Unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production}, Observed: github.Observed(setting.DeploymentBranchPolicyValue(protected)), Desired: setting.DeploymentBranchPolicyValue(protected), Result: Match},
				{Unit: reviewersUnit, Observed: github.Observed(setting.ReviewerSetValue(setting.ReviewerSetOfRule(&observedRule))), Desired: setting.ReviewerSetValue(declaredReviewers), Result: Match},
				{Unit: environmentVariableUnit(production, region), Observed: github.Observed(setting.VariableValue("eu")), Desired: setting.VariableValue("eu"), Result: Match},
			},
		},
		{
			name:     "no reviewers declared for an environment with a protection rule without reviewers mismatch",
			declared: configuration.EnvironmentDeclaration{Name: production, RequiredReviewers: &noReviewers},
			observed: []github.Environment{withProtection(existing("production", all.Mode), setting.RequiredReviewers{})},
			want: []SettingPlan{
				matchedExistence,
				{Unit: reviewersUnit, Observed: github.Observed(setting.ReviewerSetValue(setting.ReviewerSetOfRule(&setting.RequiredReviewers{}))), Desired: setting.ReviewerSetValue(noReviewers), Result: Mismatch},
			},
		},
		{
			name:     "reviewers of an absent environment are not comparable because the environment is absent",
			declared: configuration.EnvironmentDeclaration{Name: production, RequiredReviewers: &declaredReviewers},
			observed: nil,
			want: []SettingPlan{
				creation,
				{Unit: reviewersUnit, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.ReviewerSetValue(declaredReviewers), Result: NotComparable},
			},
		},
		{
			// 属する先がなく読み取れなかった存在を、確かめた事実として一致にしない (ドメインモデルの存在の観測)
			name:     "declared absence in an absent environment is not comparable because the environment is absent",
			declared: configuration.EnvironmentDeclaration{Name: production, Variables: []configuration.VariableDeclaration{{Name: region, Desired: setting.Absent()}}},
			observed: nil,
			want: []SettingPlan{
				creation,
				{Unit: environmentVariableUnit(production, region), Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.Absent(), Result: NotComparable},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			desired := configuration.DesiredState{Environments: []configuration.EnvironmentDeclaration{tc.declared}}
			observed := github.ObservationResult{Environments: &github.Environments{Existing: tc.observed}}
			if got := reconcile(desired, observed); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("reconcile() =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}
