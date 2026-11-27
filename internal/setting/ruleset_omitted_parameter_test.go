package setting

import (
	"reflect"
	"testing"
)

//trace:checks INV-001
func TestRuleset_ParametersResetBy(t *testing.T) {
	test := StatusCheck{Context: "test"}
	doNotEnforceOnCreate := ResetParameter{Rule: RulesetRuleRequiredStatusChecks, Name: "do_not_enforce_on_create", Value: "true"}
	merge, squash, rebase := AllowedMergeMethodMerge, AllowedMergeMethodSquash, AllowedMergeMethodRebase
	allMethods := []AllowedMergeMethod{merge, squash, rebase}
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
		{
			name:     "omitted allowed_merge_methods observed as all methods in another order with duplicates is not reset",
			observed: newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{rebase, merge, squash, squash}, new(true)))),
			declared: newTestRuleset(withRules(pullRequestRule(1, nil, new(true)))),
		},
		{
			name:     "reset allowed_merge_methods shows the observed order with duplicates",
			observed: newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{squash, squash, merge}, new(true)))),
			declared: newTestRuleset(withRules(pullRequestRule(1, nil, new(true)))),
			want:     []ResetParameter{{Rule: RulesetRulePullRequest, Name: "allowed_merge_methods", Value: "[squash, squash, merge]"}},
		},
		{
			name:     "omitted require_extra_approval_for_unattributed_changes observed false is reset",
			observed: newTestRuleset(withRules(pullRequestRule(0, allMethods, new(false)))),
			declared: newTestRuleset(withRules(pullRequestRule(1, allMethods, nil))),
			want:     []ResetParameter{{Rule: RulesetRulePullRequest, Name: "require_extra_approval_for_unattributed_changes", Value: "false"}},
		},
		{
			name:     "omitted require_extra_approval_for_unattributed_changes observed true is not reset",
			observed: newTestRuleset(withRules(pullRequestRule(0, allMethods, new(true)))),
			declared: newTestRuleset(withRules(pullRequestRule(1, allMethods, nil))),
		},
		{
			name:     "declared pull_request parameters are not reset",
			observed: newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{squash}, new(false)))),
			declared: newTestRuleset(withRules(pullRequestRule(1, allMethods, new(true)))),
		},
		{
			name:     "only the omitted one of two parameters is reset",
			observed: newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{squash}, new(false)))),
			declared: newTestRuleset(withRules(pullRequestRule(1, nil, new(true)))),
			want:     []ResetParameter{{Rule: RulesetRulePullRequest, Name: "allowed_merge_methods", Value: "[squash]"}},
		},
		{
			name:     "two omitted parameters of one rule are reset in the order of the parameter table",
			observed: newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{squash}, new(false)))),
			declared: newTestRuleset(withRules(pullRequestRule(1, nil, nil))),
			want: []ResetParameter{
				{Rule: RulesetRulePullRequest, Name: "allowed_merge_methods", Value: "[squash]"},
				{Rule: RulesetRulePullRequest, Name: "require_extra_approval_for_unattributed_changes", Value: "false"},
			},
		},
		{
			name:     "parameters of several rules are reset in the order of the declared rules",
			observed: newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{squash}, new(true)), statusChecksRule(false, new(true), test))),
			declared: newTestRuleset(withRules(statusChecksRule(true, nil, test), pullRequestRule(1, nil, new(true)))),
			want: []ResetParameter{
				doNotEnforceOnCreate,
				{Rule: RulesetRulePullRequest, Name: "allowed_merge_methods", Value: "[squash]"},
			},
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
	squash, merge, rebase := AllowedMergeMethodSquash, AllowedMergeMethodMerge, AllowedMergeMethodRebase
	allMethods := []AllowedMergeMethod{merge, squash, rebase}
	omittedPullRequest := newTestRuleset(withRules(pullRequestRule(1, nil, nil)))
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
		{
			name:     "omitted allowed_merge_methods that changed is reported in the observed order",
			before:   newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{squash}, new(true)))),
			after:    newTestRuleset(withRules(pullRequestRule(1, allMethods, new(true)))),
			declared: omittedPullRequest,
			want:     []OmittedParameterChange{{Rule: RulesetRulePullRequest, Name: "allowed_merge_methods", Before: "[squash]", After: "[merge, squash, rebase]"}},
		},
		{
			name:     "omitted allowed_merge_methods differing only in order is not reported",
			before:   newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{rebase, squash, merge}, new(true)))),
			after:    newTestRuleset(withRules(pullRequestRule(1, allMethods, new(true)))),
			declared: omittedPullRequest,
		},
		{
			name:     "omitted require_extra_approval_for_unattributed_changes that changed is reported",
			before:   newTestRuleset(withRules(pullRequestRule(0, allMethods, new(false)))),
			after:    newTestRuleset(withRules(pullRequestRule(1, allMethods, new(true)))),
			declared: omittedPullRequest,
			want:     []OmittedParameterChange{{Rule: RulesetRulePullRequest, Name: "require_extra_approval_for_unattributed_changes", Before: "false", After: "true"}},
		},
		{
			name:     "only the omitted one of two changed parameters is reported",
			before:   newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{squash}, new(false)))),
			after:    newTestRuleset(withRules(pullRequestRule(1, allMethods, new(true)))),
			declared: newTestRuleset(withRules(pullRequestRule(1, nil, new(true)))),
			want:     []OmittedParameterChange{{Rule: RulesetRulePullRequest, Name: "allowed_merge_methods", Before: "[squash]", After: "[merge, squash, rebase]"}},
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
