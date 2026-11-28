package changeplan

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestEnvironmentLossOf は、存在しないことを宣言した Environment が存在し、削除で失われる値の全体を読めたときだけ、
// 承認の前に示し変更前提で比べるもの (失われる値、変数、Secret) を計画時の観測から作ることを確かめる。
//
//trace:checks INV-001 FR-007
func TestEnvironmentLossOf(t *testing.T) {
	production := mustEnvironmentName(t, "production")
	unit := setting.Unit{Key: setting.EnvironmentResource, Environment: production}
	existing := github.Environment{ID: 7, Name: "Production", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicyAll, Protection: unprotected()}
	variables := []github.Variable{{Name: "REGION", Value: "eu"}}
	secrets := []github.EnvironmentSecret{{Name: "TOKEN", UpdatedAt: "2026-01-02T03:04:05Z"}}
	deletedValue := setting.EnvironmentDeletedValue{
		DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll},
		ReviewerSet:            setting.ReviewerSetOfRule(nil),
	}
	withUnknownRule := existing
	withUnknownRule.UninterpretedProtectionRules = []github.UninterpretedPart{{Kind: github.UninterpretedProtectionRuleType, Name: "custom"}}
	observedWith := func(environments ...github.Environment) github.ObservationResult {
		return github.ObservationResult{Environments: &github.Environments{Existing: environments}}
	}
	cases := []struct {
		name     string
		unit     setting.Unit
		desired  setting.Value
		observed github.ObservationResult
		want     *EnvironmentLoss
	}{
		{
			name: "existing environment declared absent under a name differing in letter case has the deleted value, variables, and secrets",
			unit: unit, desired: setting.Absent(),
			observed: observedWith(readForDeletion(existing, variables, secrets...)),
			want:     &EnvironmentLoss{DeletedValue: deletedValue, Variables: variables, Secrets: secrets},
		},
		{
			name: "existing environment declared present has none", unit: unit, desired: setting.Present(),
			observed: observedWith(readForDeletion(existing, variables, secrets...)),
		},
		{name: "absent environment declared absent has none", unit: unit, desired: setting.Absent(), observed: observedWith()},
		{
			name: "environment whose deleted value is unavailable has none", unit: unit, desired: setting.Absent(),
			observed: observedWith(readForDeletion(withUnknownRule, variables, secrets...)),
		},
		{
			name: "management unit other than the environment itself has none",
			unit: setting.Unit{Key: setting.EnvironmentVariable, Environment: production, Variable: mustVariableName(t, "REGION")}, desired: setting.Absent(),
			observed: observedWith(readForDeletion(existing, variables, secrets...)),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := environmentLossOf(tc.unit, tc.desired, tc.observed); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("environmentLossOf() = %+v, want %+v", got, tc.want)
			}
		})
	}

	// 読んでいない変数か Secret を、ない一覧として示さない
	notRead := func(e github.Environment) github.Environment {
		e.Secrets = &github.EnvironmentSecrets{}
		return e
	}(existing)
	panics := []struct {
		name        string
		environment github.Environment
	}{
		{name: "environment without its variables read panics", environment: notRead},
		{name: "environment without its secrets read panics", environment: withVariables(existing)},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("environmentLossOf() did not panic")
				}
			}()
			environmentLossOf(unit, setting.Absent(), observedWith(tc.environment))
		})
	}
}
