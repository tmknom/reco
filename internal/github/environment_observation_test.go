package github

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004
func TestEnvironmentObservation(t *testing.T) {
	declared := mustEnvironmentName(t, "production")
	noPatterns := Observed(DeploymentPatterns{})
	existing := Environments{Existing: []Environment{{
		ID:                         7,
		Name:                       "production",
		DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected,
		DeploymentPatterns:         &noPatterns,
		Protection:                 unprotectedEnvironment(),
		Variables:                  &Variables{Existing: []Variable{{Name: "REGION", Value: "eu"}}},
	}}}.Lookup(declared)
	absent := Environments{}.Lookup(declared)
	unavailable := Environments{Existing: []Environment{{Name: "café"}}}.Lookup(declared)

	t.Run("existence is present, absent, or unavailable with the same reason", func(t *testing.T) {
		cases := []struct {
			observation EnvironmentObservation
			want        SettingObservation[setting.Value]
		}{
			{observation: existing, want: Observed(setting.Present())},
			{observation: absent, want: Observed(setting.Absent())},
			{observation: unavailable, want: Unavailable[setting.Value](UnavailableNameUndetermined)},
		}
		for _, tc := range cases {
			if got := tc.observation.Existence(); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Existence() = %+v, want %+v", got, tc.want)
			}
		}
	})

	t.Run("Existence panics on the zero value", func(t *testing.T) {
		assertPanics(t, func() { EnvironmentObservation{}.Existence() })
	})

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

	t.Run("existing environment with unavailable deployment patterns makes its deployment branch policy unavailable with the same reason", func(t *testing.T) {
		unknownPatterns := Unavailable[DeploymentPatterns](UnavailableUnknownValue)
		observation := Environments{Existing: []Environment{{
			ID: 7, Name: "production", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicySelected, DeploymentPatterns: &unknownPatterns, Protection: unprotectedEnvironment(),
		}}}.Lookup(declared)
		if got, want := observation.DeploymentBranchPolicy(), Unavailable[setting.Value](UnavailableUnknownValue); !reflect.DeepEqual(got, want) {
			t.Errorf("DeploymentBranchPolicy() = %+v, want %+v", got, want)
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
		if got := o.UnavailableReason(); got != UnavailableNameUndetermined {
			t.Errorf("UnavailableReason() = %v, want %v", got, UnavailableNameUndetermined)
		}
	})

	t.Run("existing environment observes its reviewer set with the same observation", func(t *testing.T) {
		reviewers := setting.ReviewerSetOf([]setting.Reviewer{{Type: setting.ReviewerUser, ID: 1}, {Type: setting.ReviewerTeam, ID: 2}})
		withReviewerSet := func(reviewerSet SettingObservation[setting.ReviewerSet]) EnvironmentObservation {
			return Environments{Existing: []Environment{{
				ID: 7, Name: "production", Protection: EnvironmentProtection{ReviewerSet: reviewerSet, WaitTimer: 30, PreventSelfReview: true},
			}}}.Lookup(declared)
		}
		cases := []struct {
			name        string
			observation EnvironmentObservation
			want        SettingObservation[setting.Value]
		}{
			{name: "observed reviewers are the value", observation: withReviewerSet(Observed(reviewers)), want: Observed(setting.ReviewerSetValue(reviewers))},
			{
				name:        "unavailable reviewers are unavailable with the same reason",
				observation: withReviewerSet(Unavailable[setting.ReviewerSet](UnavailableUnknownValue)),
				want:        Unavailable[setting.Value](UnavailableUnknownValue),
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got := tc.observation.ReviewerSet(); !reflect.DeepEqual(got, tc.want) {
					t.Errorf("ReviewerSet() = %+v, want %+v", got, tc.want)
				}
				// 担当者の集合だけが観測不能になり、同じ保護設定のほかの項目は観測値を持つ
				if got, want := tc.observation.PreventSelfReview(), Observed(setting.BoolValue(true)); !reflect.DeepEqual(got, want) {
					t.Errorf("PreventSelfReview() = %+v, want %+v", got, want)
				}
				if got, want := tc.observation.WaitTimer(), Observed(setting.WaitTimerValue(30)); !reflect.DeepEqual(got, want) {
					t.Errorf("WaitTimer() = %+v, want %+v", got, want)
				}
			})
		}
	})

	t.Run("absent environment makes its reviewer set unavailable as absent", func(t *testing.T) {
		got, want := absent.ReviewerSet(), Unavailable[setting.Value](UnavailableEnvironmentAbsent)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ReviewerSet() = %+v, want %+v", got, want)
		}
	})

	t.Run("unavailable environment makes its reviewer set unavailable with the same reason", func(t *testing.T) {
		got, want := unavailable.ReviewerSet(), Unavailable[setting.Value](UnavailableNameUndetermined)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("ReviewerSet() = %+v, want %+v", got, want)
		}
	})

	t.Run("existing environment observes its prevent_self_review", func(t *testing.T) {
		withProtection := func(protection EnvironmentProtection) EnvironmentObservation {
			return Environments{Existing: []Environment{{ID: 7, Name: "production", Protection: protection}}}.Lookup(declared)
		}
		cases := []struct {
			name        string
			observation EnvironmentObservation
			want        bool
		}{
			{name: "false is false", observation: withProtection(unprotectedEnvironment()), want: false},
			{name: "true is true", observation: withProtection(EnvironmentProtection{ReviewerSet: Observed(setting.ReviewerSetOfRule(&setting.RequiredReviewers{})), PreventSelfReview: true}), want: true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if got, want := tc.observation.PreventSelfReview(), Observed(setting.BoolValue(tc.want)); !reflect.DeepEqual(got, want) {
					t.Errorf("PreventSelfReview() = %+v, want %+v", got, want)
				}
			})
		}
	})

	t.Run("absent environment makes its prevent_self_review unavailable as absent", func(t *testing.T) {
		got, want := absent.PreventSelfReview(), Unavailable[setting.Value](UnavailableEnvironmentAbsent)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("PreventSelfReview() = %+v, want %+v", got, want)
		}
	})

	t.Run("unavailable environment makes its prevent_self_review unavailable with the same reason", func(t *testing.T) {
		got, want := unavailable.PreventSelfReview(), Unavailable[setting.Value](UnavailableNameUndetermined)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("PreventSelfReview() = %+v, want %+v", got, want)
		}
	})

	t.Run("existing environment observes its wait_timer in minutes", func(t *testing.T) {
		observation := Environments{Existing: []Environment{{ID: 7, Name: "production", Protection: EnvironmentProtection{WaitTimer: 30, ReviewerSet: Observed(setting.ReviewerSetOfRule(nil))}}}}.Lookup(declared)
		if got, want := observation.WaitTimer(), Observed(setting.WaitTimerValue(30)); !reflect.DeepEqual(got, want) {
			t.Errorf("WaitTimer() = %+v, want %+v", got, want)
		}
	})

	t.Run("absent environment makes its wait_timer unavailable as absent", func(t *testing.T) {
		got, want := absent.WaitTimer(), Unavailable[setting.Value](UnavailableEnvironmentAbsent)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("WaitTimer() = %+v, want %+v", got, want)
		}
	})

	t.Run("unavailable environment makes its wait_timer unavailable with the same reason", func(t *testing.T) {
		got, want := unavailable.WaitTimer(), Unavailable[setting.Value](UnavailableNameUndetermined)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("WaitTimer() = %+v, want %+v", got, want)
		}
	})

	t.Run("existing environment observes a listed variable's value and an unlisted one as absent", func(t *testing.T) {
		if got, want := existing.Variable(mustVariableName(t, "REGION")), Observed(setting.VariableValue("eu")); !reflect.DeepEqual(got, want) {
			t.Errorf("Variable(REGION) = %+v, want %+v", got, want)
		}
		if got, want := existing.Variable(mustVariableName(t, "STAGE")), Observed(setting.Absent()); !reflect.DeepEqual(got, want) {
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
		got, want := unavailable.Variable(mustVariableName(t, "REGION")), Unavailable[setting.Value](UnavailableNameUndetermined)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("Variable(REGION) = %+v, want %+v", got, want)
		}
	})

	t.Run("existing environment whose variables were not read panics naming the environment", func(t *testing.T) {
		unread := Environments{Existing: []Environment{{ID: 7, Name: "production", DeploymentBranchPolicyMode: setting.DeploymentBranchPolicyAll}}}.Lookup(declared)
		defer func() {
			// 範囲の導き方の漏れを、nil 参照ではなくどの Environment で起きたかが分かる形で止めることを確かめる
			if msg, ok := recover().(string); !ok || !strings.Contains(msg, `"production"`) {
				t.Errorf("recovered %v, want a message naming the environment", msg)
			}
		}()
		unread.Variable(mustVariableName(t, "REGION"))
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
