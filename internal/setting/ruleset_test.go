package setting

import (
	"reflect"
	"testing"
)

func TestParseRulesetTarget(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   RulesetTarget
		wantOK bool
	}{
		{name: "branch is accepted", input: "branch", want: RulesetTargetBranch, wantOK: true},
		{name: "tag is rejected", input: "tag"},
		{name: "different case is rejected", input: "Branch"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseRulesetTarget(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseRulesetTarget(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParseRulesetEnforcement(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   RulesetEnforcement
		wantOK bool
	}{
		{name: "disabled is accepted", input: "disabled", want: RulesetEnforcementDisabled, wantOK: true},
		{name: "active is accepted", input: "active", want: RulesetEnforcementActive, wantOK: true},
		{name: "evaluate is accepted", input: "evaluate", want: RulesetEnforcementEvaluate, wantOK: true},
		{name: "undefined value is rejected", input: "enabled"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseRulesetEnforcement(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseRulesetEnforcement(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParseBypassActorType(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   BypassActorType
		wantOK bool
	}{
		{name: "RepositoryRole is accepted", input: "RepositoryRole", want: BypassActorRepositoryRole, wantOK: true},
		{name: "User is accepted", input: "User", want: BypassActorUser, wantOK: true},
		{name: "Integration is accepted", input: "Integration", want: BypassActorIntegration, wantOK: true},
		{name: "DeployKey is accepted", input: "DeployKey", want: BypassActorDeployKey, wantOK: true},
		{name: "Team is rejected", input: "Team"},
		{name: "different case is rejected", input: "user"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseBypassActorType(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseBypassActorType(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParseBypassMode(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   BypassMode
		wantOK bool
	}{
		{name: "always is accepted", input: "always", want: BypassModeAlways, wantOK: true},
		{name: "pull_request is accepted", input: "pull_request", want: BypassModePullRequest, wantOK: true},
		{name: "exempt is accepted", input: "exempt", want: BypassModeExempt, wantOK: true},
		{name: "pull_requests_only is rejected", input: "pull_requests_only"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseBypassMode(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseBypassMode(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParseRulesetRuleType(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   RulesetRuleType
		wantOK bool
	}{
		{name: "pull_request is accepted", input: "pull_request", want: RulesetRulePullRequest, wantOK: true},
		{name: "deletion is accepted", input: "deletion", want: RulesetRuleDeletion, wantOK: true},
		{name: "required_status_checks is accepted", input: "required_status_checks", want: RulesetRuleRequiredStatusChecks, wantOK: true},
		{name: "non_fast_forward is accepted", input: "non_fast_forward", want: RulesetRuleNonFastForward, wantOK: true},
		{name: "rule type reco does not interpret is rejected", input: "creation"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseRulesetRuleType(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseRulesetRuleType(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParseAllowedMergeMethod(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		want   AllowedMergeMethod
		wantOK bool
	}{
		{name: "merge is accepted", input: "merge", want: AllowedMergeMethodMerge, wantOK: true},
		{name: "squash is accepted", input: "squash", want: AllowedMergeMethodSquash, wantOK: true},
		{name: "rebase is accepted", input: "rebase", want: AllowedMergeMethodRebase, wantOK: true},
		{name: "different letter case is rejected", input: "Merge"},
		{name: "undefined method is rejected", input: "fast_forward"},
		{name: "empty string is rejected", input: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseAllowedMergeMethod(tc.input)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("ParseAllowedMergeMethod(%q) = (%q, %t), want (%q, %t)", tc.input, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestAllowedMergeMethodsText(t *testing.T) {
	got := AllowedMergeMethodsText([]AllowedMergeMethod{AllowedMergeMethodSquash, AllowedMergeMethodSquash, AllowedMergeMethodMerge})
	if want := "[squash, squash, merge]"; got != want {
		t.Errorf("AllowedMergeMethodsText() = %q, want %q (the given order with duplicates)", got, want)
	}
}

func TestRulesetRuleType_HasParameters(t *testing.T) {
	cases := []struct {
		ruleType RulesetRuleType
		want     bool
	}{
		{ruleType: RulesetRulePullRequest, want: true},
		{ruleType: RulesetRuleRequiredStatusChecks, want: true},
		{ruleType: RulesetRuleDeletion, want: false},
		{ruleType: RulesetRuleNonFastForward, want: false},
	}
	for _, tc := range cases {
		t.Run(string(tc.ruleType), func(t *testing.T) {
			if got := tc.ruleType.HasParameters(); got != tc.want {
				t.Errorf("HasParameters() = %t, want %t", got, tc.want)
			}
		})
	}
	t.Run("undefined rule type panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("HasParameters() did not panic")
			}
		}()
		RulesetRuleType("creation").HasParameters()
	})
}

func TestRulesetRule(t *testing.T) {
	t.Run("pull_request rule has its type and parameters", func(t *testing.T) {
		parameters := PullRequestParameters{
			RequiredApprovingReviewCount:               2,
			RequireCodeOwnerReview:                     true,
			AllowedMergeMethods:                        []AllowedMergeMethod{AllowedMergeMethodSquash},
			RequireExtraApprovalForUnattributedChanges: true,
		}
		rule := PullRequestRule(parameters)
		if got := rule.Type(); got != RulesetRulePullRequest {
			t.Errorf("Type() = %q, want %q", got, RulesetRulePullRequest)
		}
		if got := rule.PullRequest(); !reflect.DeepEqual(got, parameters) {
			t.Errorf("PullRequest() = %+v, want %+v", got, parameters)
		}
	})
	t.Run("pull_request rule does not share its parameters with the caller", func(t *testing.T) {
		methods := []AllowedMergeMethod{AllowedMergeMethodSquash}
		rule := PullRequestRule(PullRequestParameters{AllowedMergeMethods: methods})
		methods[0] = AllowedMergeMethodRebase
		got := rule.PullRequest()
		got.AllowedMergeMethods[0] = AllowedMergeMethodRebase
		again := rule.PullRequest()
		if again.AllowedMergeMethods[0] != AllowedMergeMethodSquash {
			t.Errorf("PullRequest() = %+v, want the parameters given at construction", again)
		}
	})
	t.Run("pull_request rule without allowed_merge_methods panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("PullRequestRule() did not panic")
			}
		}()
		PullRequestRule(PullRequestParameters{})
	})
	t.Run("pull_request rule with an undefined merge method panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("PullRequestRule() did not panic")
			}
		}()
		PullRequestRule(PullRequestParameters{AllowedMergeMethods: []AllowedMergeMethod{"squash\n    Rule deletion"}})
	})
	t.Run("deletion rule has its type", func(t *testing.T) {
		if got := DeletionRule().Type(); got != RulesetRuleDeletion {
			t.Errorf("Type() = %q, want %q", got, RulesetRuleDeletion)
		}
	})
	t.Run("required_status_checks rule has its type and parameters", func(t *testing.T) {
		parameters := RequiredStatusChecksParameters{
			RequiredStatusChecks:             []StatusCheck{{Context: "test"}, {Context: "lint", IntegrationID: 15368}},
			StrictRequiredStatusChecksPolicy: true,
			DoNotEnforceOnCreate:             true,
		}
		rule := RequiredStatusChecksRule(parameters)
		if got := rule.Type(); got != RulesetRuleRequiredStatusChecks {
			t.Errorf("Type() = %q, want %q", got, RulesetRuleRequiredStatusChecks)
		}
		if got := rule.RequiredStatusChecks(); !reflect.DeepEqual(got, parameters) {
			t.Errorf("RequiredStatusChecks() = %+v, want %+v", got, parameters)
		}
	})
	t.Run("required_status_checks rule does not share its parameters with the caller", func(t *testing.T) {
		checks := []StatusCheck{{Context: "test"}}
		rule := RequiredStatusChecksRule(RequiredStatusChecksParameters{RequiredStatusChecks: checks})
		checks[0].Context = "changed"
		got := rule.RequiredStatusChecks()
		got.RequiredStatusChecks[0].Context = "changed"
		again := rule.RequiredStatusChecks()
		if again.RequiredStatusChecks[0].Context != "test" {
			t.Errorf("RequiredStatusChecks() = %+v, want the parameters given at construction", again)
		}
	})
	t.Run("parameters of a rule other than pull_request panic", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("PullRequest() did not panic")
			}
		}()
		RequiredStatusChecksRule(RequiredStatusChecksParameters{}).PullRequest()
	})
	t.Run("parameters of a rule other than required_status_checks panic", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("RequiredStatusChecks() did not panic")
			}
		}()
		NonFastForwardRule().RequiredStatusChecks()
	})
}

func TestBypassActor_SameActor(t *testing.T) {
	admin := BypassActor{ActorType: BypassActorRepositoryRole, ActorID: 5, BypassMode: BypassModeAlways}
	cases := []struct {
		name  string
		other BypassActor
		want  bool
	}{
		{name: "same type and ID with a different mode is the same actor", other: BypassActor{ActorType: BypassActorRepositoryRole, ActorID: 5, BypassMode: BypassModePullRequest}, want: true},
		{name: "different ID is another actor", other: BypassActor{ActorType: BypassActorRepositoryRole, ActorID: 2, BypassMode: BypassModeAlways}, want: false},
		{name: "different type is another actor", other: BypassActor{ActorType: BypassActorUser, ActorID: 5, BypassMode: BypassModeAlways}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := admin.SameActor(tc.other); got != tc.want {
				t.Errorf("SameActor(%+v) = %t, want %t", tc.other, got, tc.want)
			}
		})
	}
}
