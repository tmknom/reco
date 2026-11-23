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
