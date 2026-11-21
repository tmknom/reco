package changeplan

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-001 FR-005
func TestEnvironmentPlans(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	protected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}
	all := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}
	selected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "main"}}}
	patternMain := setting.DeploymentPattern{Type: setting.DeploymentPatternBranch, Name: "main"}
	patternTag := setting.DeploymentPattern{Type: setting.DeploymentPatternTag, Name: "v*"}
	selectedTwo := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{patternMain, patternTag}}
	selectedTwoReversed := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{patternTag, patternMain}}
	existing := func(name string, policy setting.DeploymentBranchPolicy) github.Environment {
		return github.Environment{ID: 7, Name: name, DeploymentBranchPolicy: policy}
	}
	cases := []struct {
		name     string
		declared configuration.EnvironmentDeclaration
		observed github.EnvironmentObservation
		// want は、ObservedEnvironment を除いた計画。ObservedEnvironment は existence に observed が渡ることを別に確かめる。
		want []SettingPlan
	}{
		{
			name:     "existing environment declared by name only matches",
			declared: configuration.EnvironmentDeclaration{Name: production},
			observed: lookup(t, "production", existing("production", all)),
			want:     []SettingPlan{{Key: setting.EnvironmentResource, Environment: production, Result: Match}},
		},
		{
			name:     "existing environment with the declared mode matches both",
			declared: configuration.EnvironmentDeclaration{Name: production, DeploymentBranchPolicy: &protected},
			observed: lookup(t, "production", existing("production", protected)),
			want: []SettingPlan{
				{Key: setting.EnvironmentResource, Environment: production, Result: Match},
				{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Observed(setting.DeploymentBranchPolicyValue(protected)), Desired: setting.DeploymentBranchPolicyValue(protected), Result: Match},
			},
		},
		{
			name:     "existing environment with another mode is a mismatch of the mode",
			declared: configuration.EnvironmentDeclaration{Name: production, DeploymentBranchPolicy: &protected},
			observed: lookup(t, "production", existing("production", selected)),
			want: []SettingPlan{
				{Key: setting.EnvironmentResource, Environment: production, Result: Match},
				{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Observed(setting.DeploymentBranchPolicyValue(selected)), Desired: setting.DeploymentBranchPolicyValue(protected), Result: Mismatch},
			},
		},
		{
			name:     "patterns differing only in order match and the desired value keeps the declared patterns",
			declared: configuration.EnvironmentDeclaration{Name: production, DeploymentBranchPolicy: &selectedTwo},
			observed: lookup(t, "production", existing("production", selectedTwoReversed)),
			want: []SettingPlan{
				{Key: setting.EnvironmentResource, Environment: production, Result: Match},
				{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Observed(setting.DeploymentBranchPolicyValue(selectedTwoReversed)), Desired: setting.DeploymentBranchPolicyValue(selectedTwo), Result: Match},
			},
		},
		{
			name:     "absent environment declared by name only is a creation",
			declared: configuration.EnvironmentDeclaration{Name: production},
			observed: lookup(t, "production", existing("staging", all)),
			want:     []SettingPlan{{Key: setting.EnvironmentResource, Environment: production, Result: Mismatch}},
		},
		{
			name:     "absent environment with a mode is a creation and a not comparable mode",
			declared: configuration.EnvironmentDeclaration{Name: production, DeploymentBranchPolicy: &protected},
			observed: lookup(t, "production"),
			want: []SettingPlan{
				{Key: setting.EnvironmentResource, Environment: production, Result: Mismatch},
				{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentAbsent), Desired: setting.DeploymentBranchPolicyValue(protected), Result: NotComparable},
			},
		},
		{
			name:     "environment whose existence cannot be determined is not comparable",
			declared: configuration.EnvironmentDeclaration{Name: production, DeploymentBranchPolicy: &protected},
			observed: lookup(t, "production", existing("prodüction", all)),
			want: []SettingPlan{
				{Key: setting.EnvironmentResource, Environment: production, Result: NotComparable},
				{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: production, Observed: github.Unavailable[setting.Value](github.UnavailableEnvironmentNameUndetermined), Desired: setting.DeploymentBranchPolicyValue(protected), Result: NotComparable},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := environmentPlans(tc.declared, tc.observed)
			if len(got) == 0 || !reflect.DeepEqual(got[0].ObservedEnvironment, tc.observed) {
				t.Fatalf("existence plan does not carry the observation: %+v", got)
			}
			for i := range got {
				got[i].ObservedEnvironment = github.EnvironmentObservation{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("environmentPlans() =\n%+v\nwant\n%+v", got, tc.want)
			}
		})
	}
}

// TestRepositoryPlan_EnvironmentOperationOfModeMismatchPanicsWithoutMatchedExistence は、存在する Environment の方式の不一致を、
// 同じ名前の存在の計画が一致でなければ更新操作にせず panic することを確かめる。
// 作成の計画と組むと、存在しない Environment の ID を基準にした方式の変更になるため。
//
//trace:checks INV-004
func TestRepositoryPlan_EnvironmentOperationOfModeMismatchPanicsWithoutMatchedExistence(t *testing.T) {
	staging := mustEnvironmentName(t, "staging")
	creation := SettingPlan{Key: setting.EnvironmentResource, Environment: staging, ObservedEnvironment: lookup(t, "staging"), Result: Mismatch, Applicability: Applicable}
	all := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll})
	protected := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches})
	modeChange := SettingPlan{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: staging, Observed: github.Observed(all), Desired: protected, Result: Mismatch, Applicability: Applicable}
	r := RepositoryPlan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: []SettingPlan{creation, modeChange}}
	defer func() {
		if recover() == nil {
			t.Error("environmentOperationOf() did not panic")
		}
	}()
	_, _ = r.environmentOperationOf(modeChange)
}
