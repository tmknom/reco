package github

import (
	"reflect"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004
func TestEnvironmentObservation(t *testing.T) {
	declared := mustEnvironmentName(t, "production")
	existing := Environments{Existing: []Environment{{
		ID:                     7,
		Name:                   "production",
		DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected},
		Variables:              &Variables{Existing: []Variable{{Name: "REGION", Value: "eu"}}},
	}}}.Lookup(declared)
	absent := Environments{}.Lookup(declared)
	unavailable := Environments{Existing: []Environment{{Name: "café"}}}.Lookup(declared)

	t.Run("existing environment observes its deployment branch policy", func(t *testing.T) {
		v, ok := existing.DeploymentBranchPolicy().Value()
		if !ok {
			t.Fatal("DeploymentBranchPolicy().Value() ok = false, want true")
		}
		want := setting.DeploymentBranchPolicyValue(setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected})
		if !v.Equal(want) {
			t.Errorf("value = %v, want %v", v, want)
		}
	})

	t.Run("absent environment makes its setting unavailable as absent", func(t *testing.T) {
		o := absent.DeploymentBranchPolicy()
		if _, ok := o.Value(); ok {
			t.Fatal("Value() ok = true, want false")
		}
		if got := o.UnavailableReason(); got != UnavailableEnvironmentAbsent {
			t.Errorf("UnavailableReason() = %v, want %v", got, UnavailableEnvironmentAbsent)
		}
	})

	t.Run("unavailable environment makes its setting unavailable with the same reason", func(t *testing.T) {
		o := unavailable.DeploymentBranchPolicy()
		if _, ok := o.Value(); ok {
			t.Fatal("Value() ok = true, want false")
		}
		if got := o.UnavailableReason(); got != UnavailableEnvironmentNameUndetermined {
			t.Errorf("UnavailableReason() = %v, want %v", got, UnavailableEnvironmentNameUndetermined)
		}
	})

	t.Run("existing environment observes a listed variable's value and an unlisted one as absent", func(t *testing.T) {
		if got, want := existing.Variable(mustVariableName(t, "REGION")), Observed(setting.VariableValue("eu")); !reflect.DeepEqual(got, want) {
			t.Errorf("Variable(REGION) = %+v, want %+v", got, want)
		}
		if got, want := existing.Variable(mustVariableName(t, "STAGE")), Observed(setting.AbsentVariable()); !reflect.DeepEqual(got, want) {
			t.Errorf("Variable(STAGE) = %+v, want %+v", got, want)
		}
	})

	t.Run("absent environment makes its variable unavailable as absent", func(t *testing.T) {
		got, want := absent.Variable(mustVariableName(t, "REGION")), Unavailable[setting.Value](UnavailableEnvironmentAbsent)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Variable(REGION) = %+v, want %+v", got, want)
		}
	})

	t.Run("unavailable environment makes its variable unavailable with the same reason", func(t *testing.T) {
		got, want := unavailable.Variable(mustVariableName(t, "REGION")), Unavailable[setting.Value](UnavailableEnvironmentNameUndetermined)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Variable(REGION) = %+v, want %+v", got, want)
		}
	})

	t.Run("UnavailableReason panics when the environment exists", func(t *testing.T) {
		assertPanics(t, func() { existing.UnavailableReason() })
	})

	t.Run("UnavailableReason panics when the environment is absent", func(t *testing.T) {
		assertPanics(t, func() { absent.UnavailableReason() })
	})

	t.Run("UnavailableReason panics on the zero value", func(t *testing.T) {
		assertPanics(t, func() { EnvironmentObservation{}.UnavailableReason() })
	})
}

func assertPanics(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Error("did not panic")
		}
	}()
	f()
}
