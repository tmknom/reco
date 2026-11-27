package setting

import (
	"reflect"
	"testing"
)

//trace:checks INV-001
func TestRuleset_ParametersResetBy(t *testing.T) {
	test := StatusCheck{Context: "test"}
	doNotEnforceOnCreate := ResetParameter{Rule: RulesetRuleRequiredStatusChecks, Name: "do_not_enforce_on_create", Value: "true"}
	cases := []struct {
		name     string
		observed Ruleset
		declared Ruleset
		want     []ResetParameter
	}{
		{
			name:     "omitted do_not_enforce_on_create observed true is reset",
			observed: newTestRuleset(withRules(statusChecksRule(false, new(true), test))),
			declared: newTestRuleset(withRules(statusChecksRule(true, nil, test))),
			want:     []ResetParameter{doNotEnforceOnCreate},
		},
		{
			name:     "omitted do_not_enforce_on_create observed false is not reset",
			observed: newTestRuleset(withRules(statusChecksRule(false, new(false), test))),
			declared: newTestRuleset(withRules(statusChecksRule(true, nil, test))),
		},
		{
			name:     "declared do_not_enforce_on_create is not reset",
			observed: newTestRuleset(withRules(statusChecksRule(false, new(true), test))),
			declared: newTestRuleset(withRules(statusChecksRule(true, new(false), test))),
		},
		{
			name:     "observed rule that the declaration does not have is not reset but removed",
			observed: newTestRuleset(withRules(statusChecksRule(false, new(true), test))),
			declared: newTestRuleset(withRules(DeletionRule())),
		},
		{
			name:     "declared rule that is not observed is not reset but added",
			observed: newTestRuleset(withRules(DeletionRule())),
			declared: newTestRuleset(withRules(statusChecksRule(true, nil, test))),
		},
		{
			name:     "observed duplicated rules are reported once",
			observed: newTestRuleset(withRules(statusChecksRule(false, new(true), test), statusChecksRule(true, new(true)))),
			declared: newTestRuleset(withRules(statusChecksRule(true, nil, test))),
			want:     []ResetParameter{doNotEnforceOnCreate},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.observed.ParametersResetBy(tc.declared); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ParametersResetBy() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

//trace:checks FR-012
func TestRuleset_OmittedParameterChangesTo(t *testing.T) {
	test := StatusCheck{Context: "test"}
	omitted := newTestRuleset(withRules(statusChecksRule(true, nil, test)))
	cases := []struct {
		name          string
		before, after Ruleset
		declared      Ruleset
		want          []OmittedParameterChange
	}{
		{
			name:     "omitted do_not_enforce_on_create that changed is reported with both values",
			before:   newTestRuleset(withRules(statusChecksRule(false, new(false), test))),
			after:    newTestRuleset(withRules(statusChecksRule(true, new(true), test))),
			declared: omitted,
			want:     []OmittedParameterChange{{Rule: RulesetRuleRequiredStatusChecks, Name: "do_not_enforce_on_create", Before: "false", After: "true"}},
		},
		{
			name:     "omitted do_not_enforce_on_create that did not change is not reported",
			before:   newTestRuleset(withRules(statusChecksRule(false, new(false), test))),
			after:    newTestRuleset(withRules(statusChecksRule(true, new(false), test))),
			declared: omitted,
		},
		{
			name:     "declared do_not_enforce_on_create is not reported because the desired state confirmation compares it",
			before:   newTestRuleset(withRules(statusChecksRule(false, new(false), test))),
			after:    newTestRuleset(withRules(statusChecksRule(true, new(true), test))),
			declared: newTestRuleset(withRules(statusChecksRule(true, new(true), test))),
		},
		{
			name:     "value before the update without the rule has nothing to compare",
			before:   newTestRuleset(withRules(DeletionRule())),
			after:    newTestRuleset(withRules(statusChecksRule(true, new(true), test))),
			declared: omitted,
		},
		{
			// 更新直前を true にし、更新後の値がないことを false と読めば変化として現れるようにする
			name:     "value after the update without the rule has nothing to compare",
			before:   newTestRuleset(withRules(statusChecksRule(true, new(true), test))),
			after:    newTestRuleset(withRules(DeletionRule())),
			declared: omitted,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.before.OmittedParameterChangesTo(tc.after, tc.declared); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("OmittedParameterChangesTo() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
