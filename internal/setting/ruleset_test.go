package setting

import "testing"

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
		{name: "rule type reco does not interpret is rejected", input: "non_fast_forward"},
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

func TestRulesetRule(t *testing.T) {
	t.Run("pull_request rule has its type and parameters", func(t *testing.T) {
		parameters := PullRequestParameters{RequiredApprovingReviewCount: 2, RequireCodeOwnerReview: true}
		rule := PullRequestRule(parameters)
		if got := rule.Type(); got != RulesetRulePullRequest {
			t.Errorf("Type() = %q, want %q", got, RulesetRulePullRequest)
		}
		if got := rule.PullRequest(); got != parameters {
			t.Errorf("PullRequest() = %+v, want %+v", got, parameters)
		}
	})
	t.Run("deletion rule has its type", func(t *testing.T) {
		if got := DeletionRule().Type(); got != RulesetRuleDeletion {
			t.Errorf("Type() = %q, want %q", got, RulesetRuleDeletion)
		}
	})
	t.Run("parameters of a rule other than pull_request panic", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("PullRequest() did not panic")
			}
		}()
		DeletionRule().PullRequest()
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
