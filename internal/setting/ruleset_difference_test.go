package setting

import (
	"reflect"
	"testing"
)

//trace:checks FR-005
func TestRuleset_DifferenceTo(t *testing.T) {
	admin := BypassActor{ActorType: BypassActorRepositoryRole, ActorID: 5, BypassMode: BypassModeAlways}
	adminOnPullRequest := BypassActor{ActorType: BypassActorRepositoryRole, ActorID: 5, BypassMode: BypassModePullRequest}
	maintainer := BypassActor{ActorType: BypassActorRepositoryRole, ActorID: 2, BypassMode: BypassModeAlways}
	deployKey := BypassActor{ActorType: BypassActorDeployKey, BypassMode: BypassModeAlways}
	noApproval := PullRequestRule(PullRequestParameters{})
	oneApproval := PullRequestRule(PullRequestParameters{RequiredApprovingReviewCount: 1})
	test, testByActions := StatusCheck{Context: "test"}, StatusCheck{Context: "test", IntegrationID: 15368}
	merge, squash, rebase := AllowedMergeMethodMerge, AllowedMergeMethodSquash, AllowedMergeMethodRebase
	allMethods := []AllowedMergeMethod{merge, squash, rebase}
	cases := []struct {
		name  string
		from  Ruleset
		to    Ruleset
		want  RulesetDifference
		equal bool
	}{
		{
			name:  "identical values have no difference",
			from:  newTestRuleset(),
			to:    newTestRuleset(),
			equal: true,
		},
		{
			name:  "values differing only in the order of elements have no difference",
			from:  newTestRuleset(withRefName([]string{"a", "b"}, []string{"x", "y"}), withBypassActors(admin, deployKey), withRules(noApproval, DeletionRule())),
			to:    newTestRuleset(withRefName([]string{"b", "a"}, []string{"y", "x"}), withBypassActors(deployKey, admin), withRules(DeletionRule(), noApproval)),
			equal: true,
		},
		{
			name:  "duplicated pattern has no difference from a single one",
			from:  newTestRuleset(withRefName([]string{"a", "a"}, nil)),
			to:    newTestRuleset(withRefName([]string{"a"}, nil)),
			equal: true,
		},
		{
			name:  "values both without ref_name have no difference",
			from:  newTestRuleset(withoutRefName()),
			to:    newTestRuleset(withoutRefName()),
			equal: true,
		},
		{
			name: "different target is a target change",
			from: newTestRuleset(),
			to:   newTestRuleset(withTarget("tag")),
			want: RulesetDifference{TargetChanged: true},
		},
		{
			name: "evaluate to active is an enforcement change",
			from: newTestRuleset(withEnforcement(RulesetEnforcementEvaluate)),
			to:   newTestRuleset(withEnforcement(RulesetEnforcementActive)),
			want: RulesetDifference{EnforcementChanged: true},
		},
		{
			name: "adding ref_name is a ref_name change and adds all its patterns",
			from: newTestRuleset(withoutRefName()),
			to:   newTestRuleset(withRefName([]string{"a"}, []string{"x"})),
			want: RulesetDifference{RefNameChanged: true, IncludeAdded: []string{"a"}, ExcludeAdded: []string{"x"}},
		},
		{
			name: "removing ref_name is a ref_name change and removes all its patterns",
			from: newTestRuleset(withRefName([]string{"a"}, []string{"x"})),
			to:   newTestRuleset(withoutRefName()),
			want: RulesetDifference{RefNameChanged: true, IncludeRemoved: []string{"a"}, ExcludeRemoved: []string{"x"}},
		},
		{
			name: "empty ref_name differs from no ref_name",
			from: newTestRuleset(withRefName(nil, nil)),
			to:   newTestRuleset(withoutRefName()),
			want: RulesetDifference{RefNameChanged: true},
		},
		{
			name: "patterns differing only in letter case are different elements",
			from: newTestRuleset(withRefName([]string{"refs/heads/main"}, nil)),
			to:   newTestRuleset(withRefName([]string{"refs/heads/Main"}, nil)),
			want: RulesetDifference{IncludeRemoved: []string{"refs/heads/main"}, IncludeAdded: []string{"refs/heads/Main"}},
		},
		{
			name: "different exclude alone is an exclude difference",
			from: newTestRuleset(withRefName([]string{"a"}, []string{"x"})),
			to:   newTestRuleset(withRefName([]string{"a"}, []string{"y"})),
			want: RulesetDifference{ExcludeRemoved: []string{"x"}, ExcludeAdded: []string{"y"}},
		},
		{
			name: "different bypass mode alone removes and adds the same actor",
			from: newTestRuleset(withBypassActors(admin)),
			to:   newTestRuleset(withBypassActors(adminOnPullRequest)),
			want: RulesetDifference{BypassActorsRemoved: []BypassActor{admin}, BypassActorsAdded: []BypassActor{adminOnPullRequest}},
		},
		{
			name: "different actor ID alone removes and adds an actor",
			from: newTestRuleset(withBypassActors(admin)),
			to:   newTestRuleset(withBypassActors(maintainer)),
			want: RulesetDifference{BypassActorsRemoved: []BypassActor{admin}, BypassActorsAdded: []BypassActor{maintainer}},
		},
		{
			name: "missing and extra rules are added and removed",
			from: newTestRuleset(withRules(DeletionRule())),
			to:   newTestRuleset(withRules(noApproval)),
			want: RulesetDifference{RulesRemoved: []RulesetRule{DeletionRule()}, RulesAdded: []RulesetRule{noApproval}},
		},
		{
			name: "one different pull_request parameter removes and adds the rule",
			from: newTestRuleset(withRules(noApproval)),
			to:   newTestRuleset(withRules(oneApproval)),
			want: RulesetDifference{RulesRemoved: []RulesetRule{noApproval}, RulesAdded: []RulesetRule{oneApproval}},
		},
		{
			name: "removed elements follow the receiver's order and added ones the argument's order",
			from: newTestRuleset(withRefName([]string{"c", "a", "b"}, nil)),
			to:   newTestRuleset(withRefName([]string{"z", "x", "y"}, nil)),
			want: RulesetDifference{IncludeRemoved: []string{"c", "a", "b"}, IncludeAdded: []string{"z", "x", "y"}},
		},
		{
			name: "status check differing only in integration_id removes and adds the rule",
			from: newTestRuleset(withRules(statusChecksRule(false, nil, test))),
			to:   newTestRuleset(withRules(statusChecksRule(false, nil, testByActions))),
			want: RulesetDifference{RulesRemoved: []RulesetRule{statusChecksRule(false, nil, test)}, RulesAdded: []RulesetRule{statusChecksRule(false, nil, testByActions)}},
		},
		{
			name: "status checks differing only in letter case of context remove and add the rule",
			from: newTestRuleset(withRules(statusChecksRule(false, nil, test))),
			to:   newTestRuleset(withRules(statusChecksRule(false, nil, StatusCheck{Context: "Test"}))),
			want: RulesetDifference{RulesRemoved: []RulesetRule{statusChecksRule(false, nil, test)}, RulesAdded: []RulesetRule{statusChecksRule(false, nil, StatusCheck{Context: "Test"})}},
		},
		{
			name: "different strict policy removes and adds the rule",
			from: newTestRuleset(withRules(statusChecksRule(false, nil, test))),
			to:   newTestRuleset(withRules(statusChecksRule(true, nil, test))),
			want: RulesetDifference{RulesRemoved: []RulesetRule{statusChecksRule(false, nil, test)}, RulesAdded: []RulesetRule{statusChecksRule(true, nil, test)}},
		},
		{
			name: "do_not_enforce_on_create held by both values and different removes and adds the rule",
			from: newTestRuleset(withRules(statusChecksRule(false, new(true), test))),
			to:   newTestRuleset(withRules(statusChecksRule(false, new(false), test))),
			want: RulesetDifference{RulesRemoved: []RulesetRule{statusChecksRule(false, new(true), test)}, RulesAdded: []RulesetRule{statusChecksRule(false, new(false), test)}},
		},
		{
			name:  "do_not_enforce_on_create omitted by one value is not compared",
			from:  newTestRuleset(withRules(statusChecksRule(false, new(true), test))),
			to:    newTestRuleset(withRules(statusChecksRule(false, nil, test))),
			equal: true,
		},
		{
			name: "removed rule keeps the do_not_enforce_on_create that was not compared",
			from: newTestRuleset(withRules(statusChecksRule(false, new(true), test))),
			to:   newTestRuleset(withRules(statusChecksRule(true, nil, test))),
			want: RulesetDifference{RulesRemoved: []RulesetRule{statusChecksRule(false, new(true), test)}, RulesAdded: []RulesetRule{statusChecksRule(true, nil, test)}},
		},
		{
			name:  "allowed_merge_methods differing only in order and duplicates have no difference",
			from:  newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{merge, squash, rebase, rebase}, new(true)))),
			to:    newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{rebase, squash, merge}, new(true)))),
			equal: true,
		},
		{
			name: "allowed_merge_methods held by both values and different removes and adds the rule",
			from: newTestRuleset(withRules(pullRequestRule(0, allMethods, new(true)))),
			to:   newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{squash}, new(true)))),
			want: RulesetDifference{RulesRemoved: []RulesetRule{pullRequestRule(0, allMethods, new(true))}, RulesAdded: []RulesetRule{pullRequestRule(0, []AllowedMergeMethod{squash}, new(true))}},
		},
		{
			name: "require_extra_approval_for_unattributed_changes held by both values and different removes and adds the rule",
			from: newTestRuleset(withRules(pullRequestRule(0, allMethods, new(true)))),
			to:   newTestRuleset(withRules(pullRequestRule(0, allMethods, new(false)))),
			want: RulesetDifference{RulesRemoved: []RulesetRule{pullRequestRule(0, allMethods, new(true))}, RulesAdded: []RulesetRule{pullRequestRule(0, allMethods, new(false))}},
		},
		{
			name:  "require_extra_approval_for_unattributed_changes omitted by one value is not compared",
			from:  newTestRuleset(withRules(pullRequestRule(0, allMethods, new(false)))),
			to:    newTestRuleset(withRules(pullRequestRule(0, allMethods, nil))),
			equal: true,
		},
		{
			name: "parameter declared beside an omitted one in the same rule is compared",
			from: newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{squash}, new(true)))),
			to:   newTestRuleset(withRules(pullRequestRule(0, nil, new(false)))),
			want: RulesetDifference{RulesRemoved: []RulesetRule{pullRequestRule(0, []AllowedMergeMethod{squash}, new(true))}, RulesAdded: []RulesetRule{pullRequestRule(0, nil, new(false))}},
		},
		{
			name:  "parameter declared beside an omitted one that matches has no difference",
			from:  newTestRuleset(withRules(pullRequestRule(0, []AllowedMergeMethod{squash}, new(true)))),
			to:    newTestRuleset(withRules(pullRequestRule(0, nil, new(true)))),
			equal: true,
		},
		{
			name: "parameter omitted in another rule type does not stop comparing a declared one",
			from: newTestRuleset(withRules(pullRequestRule(0, allMethods, new(true)), statusChecksRule(false, new(false), test))),
			to:   newTestRuleset(withRules(pullRequestRule(0, nil, nil), statusChecksRule(false, new(true), test))),
			want: RulesetDifference{RulesRemoved: []RulesetRule{statusChecksRule(false, new(false), test)}, RulesAdded: []RulesetRule{statusChecksRule(false, new(true), test)}},
		},
		{
			name: "duplicated elements are reported once",
			from: newTestRuleset(withRefName([]string{"a", "a"}, nil), withBypassActors(deployKey, deployKey)),
			to:   newTestRuleset(withRefName([]string{"b", "b"}, nil), withBypassActors()),
			want: RulesetDifference{IncludeRemoved: []string{"a"}, IncludeAdded: []string{"b"}, BypassActorsRemoved: []BypassActor{deployKey}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.from.DifferenceTo(tc.to)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("DifferenceTo() = %+v, want %+v", got, tc.want)
			}
			if eq := tc.from.equal(tc.to); eq != tc.equal {
				t.Errorf("equal() = %t, want %t", eq, tc.equal)
			}
		})
	}
}

func TestRulesetDifference_Empty(t *testing.T) {
	cases := []struct {
		name       string
		difference RulesetDifference
		want       bool
	}{
		{name: "zero difference is empty", difference: RulesetDifference{}, want: true},
		{name: "target change is not empty", difference: RulesetDifference{TargetChanged: true}},
		{name: "enforcement change is not empty", difference: RulesetDifference{EnforcementChanged: true}},
		{name: "ref_name change is not empty", difference: RulesetDifference{RefNameChanged: true}},
		{name: "removed include pattern is not empty", difference: RulesetDifference{IncludeRemoved: []string{"a"}}},
		{name: "added include pattern is not empty", difference: RulesetDifference{IncludeAdded: []string{"a"}}},
		{name: "removed exclude pattern is not empty", difference: RulesetDifference{ExcludeRemoved: []string{"a"}}},
		{name: "added exclude pattern is not empty", difference: RulesetDifference{ExcludeAdded: []string{"a"}}},
		{name: "removed bypass actor is not empty", difference: RulesetDifference{BypassActorsRemoved: []BypassActor{{ActorType: BypassActorDeployKey, BypassMode: BypassModeAlways}}}},
		{name: "added bypass actor is not empty", difference: RulesetDifference{BypassActorsAdded: []BypassActor{{ActorType: BypassActorDeployKey, BypassMode: BypassModeAlways}}}},
		{name: "removed rule is not empty", difference: RulesetDifference{RulesRemoved: []RulesetRule{DeletionRule()}}},
		{name: "added rule is not empty", difference: RulesetDifference{RulesAdded: []RulesetRule{DeletionRule()}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.difference.Empty(); got != tc.want {
				t.Errorf("Empty() = %t, want %t", got, tc.want)
			}
		})
	}
}

// TestPullRequestParameters_DifferenceTo は、パラメータの差を確かめ、差が空であることと、ルールを同じ値とする判定が一致することも確かめる。
// 二つは別々に実装しているため、食い違うと、同じ値と判定したルールの表示に差の行が出るか、差の行のない不一致になる。
//
//trace:checks FR-005
func TestPullRequestParameters_DifferenceTo(t *testing.T) {
	merge, squash, rebase := AllowedMergeMethodMerge, AllowedMergeMethodSquash, AllowedMergeMethodRebase
	all := []AllowedMergeMethod{merge, squash, rebase}
	parameters := func(methods []AllowedMergeMethod, extraApproval *bool) PullRequestParameters {
		return PullRequestParameters{AllowedMergeMethods: methods, RequireExtraApprovalForUnattributedChanges: extraApproval}
	}
	cases := []struct {
		name     string
		from, to PullRequestParameters
		want     PullRequestDifference
	}{
		{name: "same parameters have no difference", from: parameters(all, new(true)), to: parameters(all, new(true))},
		{
			name: "different required_approving_review_count is a change",
			from: PullRequestParameters{}, to: PullRequestParameters{RequiredApprovingReviewCount: 1},
			want: PullRequestDifference{RequiredApprovingReviewCountChanged: true},
		},
		{
			name: "different dismiss_stale_reviews_on_push is a change",
			from: PullRequestParameters{}, to: PullRequestParameters{DismissStaleReviewsOnPush: true},
			want: PullRequestDifference{DismissStaleReviewsOnPushChanged: true},
		},
		{
			name: "different require_code_owner_review is a change",
			from: PullRequestParameters{}, to: PullRequestParameters{RequireCodeOwnerReview: true},
			want: PullRequestDifference{RequireCodeOwnerReviewChanged: true},
		},
		{
			name: "different require_last_push_approval is a change",
			from: PullRequestParameters{}, to: PullRequestParameters{RequireLastPushApproval: true},
			want: PullRequestDifference{RequireLastPushApprovalChanged: true},
		},
		{
			name: "different required_review_thread_resolution is a change",
			from: PullRequestParameters{}, to: PullRequestParameters{RequiredReviewThreadResolution: true},
			want: PullRequestDifference{RequiredReviewThreadResolutionChanged: true},
		},
		{
			name: "allowed_merge_methods differing only in order and duplicates have no difference",
			from: parameters([]AllowedMergeMethod{squash, squash, merge}, nil), to: parameters([]AllowedMergeMethod{merge, squash}, nil),
		},
		{
			name: "allowed_merge_methods held by both and different is a change",
			from: parameters(all, nil), to: parameters([]AllowedMergeMethod{rebase}, nil),
			want: PullRequestDifference{AllowedMergeMethodsChanged: true},
		},
		{name: "allowed_merge_methods omitted by the argument is not compared", from: parameters(all, nil), to: parameters(nil, nil)},
		{name: "allowed_merge_methods omitted by the receiver is not compared", from: parameters(nil, nil), to: parameters([]AllowedMergeMethod{squash}, nil)},
		{
			name: "require_extra_approval_for_unattributed_changes held by both and different is a change",
			from: parameters(nil, new(true)), to: parameters(nil, new(false)),
			want: PullRequestDifference{RequireExtraApprovalForUnattributedChangesChanged: true},
		},
		{name: "require_extra_approval_for_unattributed_changes omitted by the argument is not compared", from: parameters(nil, new(true)), to: parameters(nil, nil)},
		{name: "require_extra_approval_for_unattributed_changes omitted by the receiver is not compared", from: parameters(nil, nil), to: parameters(nil, new(false))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.from.DifferenceTo(tc.to)
			if got != tc.want {
				t.Errorf("DifferenceTo() = %+v, want %+v", got, tc.want)
			}
			from, to := newTestRuleset(withRules(PullRequestRule(tc.from))), newTestRuleset(withRules(PullRequestRule(tc.to)))
			if eq, empty := from.equal(to), got.Empty(); eq != empty {
				t.Errorf("rulesets equal() = %t, but parameter difference Empty() = %t", eq, empty)
			}
		})
	}
}

func TestPullRequestDifference_Empty(t *testing.T) {
	cases := []struct {
		name       string
		difference PullRequestDifference
		want       bool
	}{
		{name: "zero difference is empty", want: true},
		{name: "required_approving_review_count change is not empty", difference: PullRequestDifference{RequiredApprovingReviewCountChanged: true}},
		{name: "dismiss_stale_reviews_on_push change is not empty", difference: PullRequestDifference{DismissStaleReviewsOnPushChanged: true}},
		{name: "require_code_owner_review change is not empty", difference: PullRequestDifference{RequireCodeOwnerReviewChanged: true}},
		{name: "require_last_push_approval change is not empty", difference: PullRequestDifference{RequireLastPushApprovalChanged: true}},
		{name: "required_review_thread_resolution change is not empty", difference: PullRequestDifference{RequiredReviewThreadResolutionChanged: true}},
		{name: "allowed_merge_methods change is not empty", difference: PullRequestDifference{AllowedMergeMethodsChanged: true}},
		{name: "require_extra_approval_for_unattributed_changes change is not empty", difference: PullRequestDifference{RequireExtraApprovalForUnattributedChangesChanged: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.difference.Empty(); got != tc.want {
				t.Errorf("Empty() = %t, want %t", got, tc.want)
			}
		})
	}
}

// TestRequiredStatusChecksParameters_DifferenceTo は、パラメータの差を確かめ、差が空であることと、ルールを同じ値とする判定が一致することも確かめる。
// 二つは別々に実装しているため、食い違うと、同じ値と判定したルールの表示に差の行が出るか、差の行のない不一致になる。
//
//trace:checks FR-005
func TestRequiredStatusChecksParameters_DifferenceTo(t *testing.T) {
	test, testByActions, lint := StatusCheck{Context: "test"}, StatusCheck{Context: "test", IntegrationID: 15368}, StatusCheck{Context: "lint"}
	parameters := func(strict bool, doNotEnforceOnCreate *bool, checks ...StatusCheck) RequiredStatusChecksParameters {
		return RequiredStatusChecksParameters{RequiredStatusChecks: checks, StrictRequiredStatusChecksPolicy: strict, DoNotEnforceOnCreate: doNotEnforceOnCreate}
	}
	cases := []struct {
		name     string
		from, to RequiredStatusChecksParameters
		want     RequiredStatusChecksDifference
	}{
		{name: "same parameters have no difference", from: parameters(true, new(true), test), to: parameters(true, new(true), test)},
		{name: "checks differing only in order and duplicates have no difference", from: parameters(false, nil, test, lint, lint), to: parameters(false, nil, lint, test)},
		{name: "empty checks have no difference", from: parameters(false, nil), to: parameters(false, nil)},
		{
			name: "removed checks follow the receiver's order once each and added ones the argument's order",
			from: parameters(false, nil, lint, test, lint),
			to:   parameters(false, nil, testByActions),
			want: RequiredStatusChecksDifference{ChecksRemoved: []StatusCheck{lint, test}, ChecksAdded: []StatusCheck{testByActions}},
		},
		{name: "different strict policy is a strict policy change", from: parameters(false, nil), to: parameters(true, nil), want: RequiredStatusChecksDifference{StrictPolicyChanged: true}},
		{name: "do_not_enforce_on_create held by both and different is a change", from: parameters(false, new(true)), to: parameters(false, new(false)), want: RequiredStatusChecksDifference{DoNotEnforceOnCreateChanged: true}},
		{name: "do_not_enforce_on_create omitted by the argument is not compared", from: parameters(false, new(true)), to: parameters(false, nil)},
		{name: "do_not_enforce_on_create omitted by the receiver is not compared", from: parameters(false, nil), to: parameters(false, new(true))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.from.DifferenceTo(tc.to)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("DifferenceTo() = %+v, want %+v", got, tc.want)
			}
			from, to := newTestRuleset(withRules(RequiredStatusChecksRule(tc.from))), newTestRuleset(withRules(RequiredStatusChecksRule(tc.to)))
			if eq, empty := from.equal(to), got.Empty(); eq != empty {
				t.Errorf("rulesets equal() = %t, but parameter difference Empty() = %t", eq, empty)
			}
		})
	}
}

func TestRequiredStatusChecksDifference_Empty(t *testing.T) {
	cases := []struct {
		name       string
		difference RequiredStatusChecksDifference
		want       bool
	}{
		{name: "zero difference is empty", want: true},
		{name: "removed check is not empty", difference: RequiredStatusChecksDifference{ChecksRemoved: []StatusCheck{{Context: "test"}}}},
		{name: "added check is not empty", difference: RequiredStatusChecksDifference{ChecksAdded: []StatusCheck{{Context: "test"}}}},
		{name: "strict policy change is not empty", difference: RequiredStatusChecksDifference{StrictPolicyChanged: true}},
		{name: "do_not_enforce_on_create change is not empty", difference: RequiredStatusChecksDifference{DoNotEnforceOnCreateChanged: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.difference.Empty(); got != tc.want {
				t.Errorf("Empty() = %t, want %t", got, tc.want)
			}
		})
	}
}
