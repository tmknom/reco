package setting

import "testing"

type testRulesetOption func(*Ruleset)

// newTestRuleset は、全体を解釈できた値の形を持つ基準の Ruleset を作り、options で一つずつ変える。
func newTestRuleset(options ...testRulesetOption) Ruleset {
	r := Ruleset{
		Target:       RulesetTargetBranch,
		Enforcement:  RulesetEnforcementActive,
		Conditions:   RulesetConditions{RefName: &RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{}}},
		BypassActors: []BypassActor{{ActorType: BypassActorRepositoryRole, ActorID: 5, BypassMode: BypassModeAlways}},
		Rules:        []RulesetRule{DeletionRule()},
	}
	for _, option := range options {
		option(&r)
	}
	return r
}

func withTarget(target RulesetTarget) testRulesetOption {
	return func(r *Ruleset) { r.Target = target }
}

func withEnforcement(enforcement RulesetEnforcement) testRulesetOption {
	return func(r *Ruleset) { r.Enforcement = enforcement }
}

func withRefName(include, exclude []string) testRulesetOption {
	return func(r *Ruleset) { r.Conditions.RefName = &RefNameCondition{Include: include, Exclude: exclude} }
}

func withoutRefName() testRulesetOption {
	return func(r *Ruleset) { r.Conditions.RefName = nil }
}

func withBypassActors(actors ...BypassActor) testRulesetOption {
	return func(r *Ruleset) { r.BypassActors = actors }
}

func withRules(rules ...RulesetRule) testRulesetOption {
	return func(r *Ruleset) { r.Rules = rules }
}

func mustRulesetName(t *testing.T, s string) RulesetName {
	t.Helper()
	name, err := ParseRulesetName(s)
	if err != nil {
		t.Fatal(err)
	}
	return name
}
