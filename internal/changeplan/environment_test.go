package changeplan

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-001
//trace:checks FR-005
func TestEnvironmentPlans(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	protected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyProtectedBranches}
	all := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}
	selected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "main"}}}
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
