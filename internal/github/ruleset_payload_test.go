package github

import (
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// baseRulesetValue は、rulesetDetail の基準の本文から読む値である。
func baseRulesetValue() setting.Ruleset {
	return setting.Ruleset{
		Target:       setting.RulesetTargetBranch,
		Enforcement:  setting.RulesetEnforcementActive,
		Conditions:   setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{"refs/heads/tmp"}}},
		BypassActors: []setting.BypassActor{{ActorType: setting.BypassActorRepositoryRole, ActorID: 5, BypassMode: setting.BypassModeAlways}},
		Rules: []setting.RulesetRule{
			setting.DeletionRule(),
			setting.PullRequestRule(basePullRequestParameters()),
			setting.NonFastForwardRule(),
			setting.RequiredStatusChecksRule(baseStatusChecksParameters()),
		},
	}
}

// basePullRequestParameters は、rulesetDetail の基準の本文の pull_request のルールから読むパラメータである。
func basePullRequestParameters() setting.PullRequestParameters {
	return setting.PullRequestParameters{
		RequiredApprovingReviewCount:               1,
		DismissStaleReviewsOnPush:                  true,
		RequireLastPushApproval:                    true,
		AllowedMergeMethods:                        []setting.AllowedMergeMethod{setting.AllowedMergeMethodMerge, setting.AllowedMergeMethodSquash, setting.AllowedMergeMethodRebase},
		RequireExtraApprovalForUnattributedChanges: true,
	}
}

// withPullRequestParameters は、基準の値の pull_request のルールのパラメータを edit で変える。
func withPullRequestParameters(v *setting.Ruleset, edit func(*setting.PullRequestParameters)) {
	p := basePullRequestParameters()
	edit(&p)
	v.Rules[1] = setting.PullRequestRule(p)
}

// baseStatusChecksParameters は、rulesetDetail の基準の本文の required_status_checks のルールから読むパラメータである。
func baseStatusChecksParameters() setting.RequiredStatusChecksParameters {
	return setting.RequiredStatusChecksParameters{
		RequiredStatusChecks:             []setting.StatusCheck{{Context: "test"}, {Context: "lint", IntegrationID: 15368}},
		StrictRequiredStatusChecksPolicy: true,
		DoNotEnforceOnCreate:             false,
	}
}

// statusChecksParameters は基準の本文の required_status_checks のルールのパラメータを返す。
func statusChecksParameters(body map[string]any) map[string]any {
	return rulesetRule(body, 3)["parameters"].(map[string]any)
}

// statusCheck は基準の本文の required_status_checks のルールの i 番目のチェックを返す。
func statusCheck(body map[string]any, i int) map[string]any {
	return statusChecksParameters(body)["required_status_checks"].([]any)[i].(map[string]any)
}

// withStatusChecksParameters は、基準の値の required_status_checks のルールのパラメータを edit で変える。
func withStatusChecksParameters(v *setting.Ruleset, edit func(*setting.RequiredStatusChecksParameters)) {
	p := baseStatusChecksParameters()
	edit(&p)
	v.Rules[3] = setting.RequiredStatusChecksRule(p)
}

func refName(body map[string]any) map[string]any {
	return body["conditions"].(map[string]any)["ref_name"].(map[string]any)
}

func bypassActor(body map[string]any, i int) map[string]any {
	return body["bypass_actors"].([]any)[i].(map[string]any)
}

func rulesetRule(body map[string]any, i int) map[string]any {
	return body["rules"].([]any)[i].(map[string]any)
}

// pullRequestParameters は基準の本文の pull_request のルールのパラメータを返す。
func pullRequestParameters(body map[string]any) map[string]any {
	return rulesetRule(body, 1)["parameters"].(map[string]any)
}

//trace:checks FR-004 QR-002
func TestDecodeRulesetInterpreted(t *testing.T) {
	cases := []struct {
		name   string
		change func(body map[string]any)
		want   func(v *setting.Ruleset)
	}{
		{name: "the base body is read as the value and the unread top-level fields are ignored"},
		{
			name:   "evaluate enforcement is read as it is",
			change: func(b map[string]any) { b["enforcement"] = "evaluate" },
			want:   func(v *setting.Ruleset) { v.Enforcement = setting.RulesetEnforcementEvaluate },
		},
		{
			name:   "disabled enforcement is read as it is",
			change: func(b map[string]any) { b["enforcement"] = "disabled" },
			want:   func(v *setting.Ruleset) { v.Enforcement = setting.RulesetEnforcementDisabled },
		},
		{
			name:   "empty conditions have no ref_name condition",
			change: func(b map[string]any) { b["conditions"] = map[string]any{} },
			want:   func(v *setting.Ruleset) { v.Conditions = setting.RulesetConditions{} },
		},
		{
			name:   "include patterns keep their order and duplicates",
			change: func(b map[string]any) { refName(b)["include"] = []any{"b", "a", "b"} },
			want:   func(v *setting.Ruleset) { v.Conditions.RefName.Include = []string{"b", "a", "b"} },
		},
		{
			name:   "empty include and exclude are empty patterns",
			change: func(b map[string]any) { refName(b)["include"], refName(b)["exclude"] = []any{}, []any{} },
			want: func(v *setting.Ruleset) {
				v.Conditions.RefName = &setting.RefNameCondition{Include: []string{}, Exclude: []string{}}
			},
		},
		{
			name: "every handled actor type and bypass mode is read in order with duplicates",
			change: func(b map[string]any) {
				b["bypass_actors"] = []any{
					map[string]any{"actor_type": "User", "actor_id": 7, "bypass_mode": "pull_request"},
					map[string]any{"actor_type": "Integration", "actor_id": 8, "bypass_mode": "exempt"},
					map[string]any{"actor_type": "RepositoryRole", "actor_id": 1, "bypass_mode": "always"},
					map[string]any{"actor_type": "User", "actor_id": 7, "bypass_mode": "pull_request"},
				}
			},
			want: func(v *setting.Ruleset) {
				v.BypassActors = []setting.BypassActor{
					{ActorType: setting.BypassActorUser, ActorID: 7, BypassMode: setting.BypassModePullRequest},
					{ActorType: setting.BypassActorIntegration, ActorID: 8, BypassMode: setting.BypassModeExempt},
					{ActorType: setting.BypassActorRepositoryRole, ActorID: 1, BypassMode: setting.BypassModeAlways},
					{ActorType: setting.BypassActorUser, ActorID: 7, BypassMode: setting.BypassModePullRequest},
				}
			},
		},
		{
			name: "deploy key with null actor_id has actor ID 0",
			change: func(b map[string]any) {
				b["bypass_actors"] = []any{map[string]any{"actor_type": "DeployKey", "actor_id": nil, "bypass_mode": "always"}}
			},
			want: func(v *setting.Ruleset) {
				v.BypassActors = []setting.BypassActor{{ActorType: setting.BypassActorDeployKey, BypassMode: setting.BypassModeAlways}}
			},
		},
		{
			name: "deploy key without actor_id has actor ID 0",
			change: func(b map[string]any) {
				b["bypass_actors"] = []any{map[string]any{"actor_type": "DeployKey", "bypass_mode": "exempt"}}
			},
			want: func(v *setting.Ruleset) {
				v.BypassActors = []setting.BypassActor{{ActorType: setting.BypassActorDeployKey, BypassMode: setting.BypassModeExempt}}
			},
		},
		{
			name:   "empty bypass actors and rules are read as empty",
			change: func(b map[string]any) { b["bypass_actors"], b["rules"] = []any{}, []any{} },
			want:   func(v *setting.Ruleset) { v.BypassActors, v.Rules = []setting.BypassActor{}, []setting.RulesetRule{} },
		},
		{
			name: "rules of the same type are kept in order",
			change: func(b map[string]any) {
				b["rules"] = []any{map[string]any{"type": "deletion"}, map[string]any{"type": "deletion"}}
			},
			want: func(v *setting.Ruleset) {
				v.Rules = []setting.RulesetRule{setting.DeletionRule(), setting.DeletionRule()}
			},
		},
		{
			name: "status checks keep their order, duplicates, and empty context",
			change: func(b map[string]any) {
				statusChecksParameters(b)["required_status_checks"] = []any{
					map[string]any{"context": "lint"}, map[string]any{"context": ""}, map[string]any{"context": "lint"},
				}
			},
			want: func(v *setting.Ruleset) {
				withStatusChecksParameters(v, func(p *setting.RequiredStatusChecksParameters) {
					p.RequiredStatusChecks = []setting.StatusCheck{{Context: "lint"}, {Context: ""}, {Context: "lint"}}
				})
			},
		},
		{
			name:   "empty status checks are read as empty",
			change: func(b map[string]any) { statusChecksParameters(b)["required_status_checks"] = []any{} },
			want: func(v *setting.Ruleset) {
				withStatusChecksParameters(v, func(p *setting.RequiredStatusChecksParameters) { p.RequiredStatusChecks = []setting.StatusCheck{} })
			},
		},
		{
			name: "do_not_enforce_on_create true and strict policy false are read as values",
			change: func(b map[string]any) {
				p := statusChecksParameters(b)
				p["do_not_enforce_on_create"], p["strict_required_status_checks_policy"] = true, false
			},
			want: func(v *setting.Ruleset) {
				withStatusChecksParameters(v, func(p *setting.RequiredStatusChecksParameters) {
					p.DoNotEnforceOnCreate, p.StrictRequiredStatusChecksPolicy = true, false
				})
			},
		},
		{
			name: "allowed merge methods are read in the received order with duplicates",
			change: func(b map[string]any) {
				pullRequestParameters(b)["allowed_merge_methods"] = []any{"squash", "squash", "merge"}
			},
			want: func(v *setting.Ruleset) {
				withPullRequestParameters(v, func(p *setting.PullRequestParameters) {
					p.AllowedMergeMethods = []setting.AllowedMergeMethod{setting.AllowedMergeMethodSquash, setting.AllowedMergeMethodSquash, setting.AllowedMergeMethodMerge}
				})
			},
		},
		{
			name: "allowed merge methods other than all methods and false extra approval are read as values",
			change: func(b map[string]any) {
				p := pullRequestParameters(b)
				p["allowed_merge_methods"], p["require_extra_approval_for_unattributed_changes"] = []any{"rebase"}, false
			},
			want: func(v *setting.Ruleset) {
				withPullRequestParameters(v, func(p *setting.PullRequestParameters) {
					p.AllowedMergeMethods, p.RequireExtraApprovalForUnattributedChanges = []setting.AllowedMergeMethod{setting.AllowedMergeMethodRebase}, false
				})
			},
		},
		{
			name: "pull request parameters are read as they are without a range check",
			change: func(b map[string]any) {
				p := pullRequestParameters(b)
				p["required_approving_review_count"] = 11
				p["dismiss_stale_reviews_on_push"] = false
				p["require_code_owner_review"] = true
				p["require_last_push_approval"] = false
				p["required_review_thread_resolution"] = true
			},
			want: func(v *setting.Ruleset) {
				withPullRequestParameters(v, func(p *setting.PullRequestParameters) {
					p.RequiredApprovingReviewCount, p.DismissStaleReviewsOnPush, p.RequireCodeOwnerReview = 11, false, true
					p.RequireLastPushApproval, p.RequiredReviewThreadResolution = false, true
				})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := baseRulesetValue()
			if tc.want != nil {
				tc.want(&want)
			}
			got, err := decodeRuleset(rulesetDetail(t, tc.change))
			if err != nil {
				t.Fatalf("decodeRuleset() error = %v", err)
			}
			if got.ID != 42 || got.Name != "protect" {
				t.Errorf("decodeRuleset() ID, Name = %d, %q, want 42, %q", got.ID, got.Name, "protect")
			}
			value, ok := got.Value()
			if !ok {
				t.Fatalf("decodeRuleset() has no value, uninterpreted parts = %+v", got.UninterpretedParts())
			}
			if !reflect.DeepEqual(value, want) {
				t.Errorf("decodeRuleset() value = %+v, want %+v", value, want)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestDecodeRulesetUninterpreted(t *testing.T) {
	pullRequest, deletion := setting.RulesetRulePullRequest, setting.RulesetRuleDeletion
	nonFastForward, statusChecks := setting.RulesetRuleNonFastForward, setting.RulesetRuleRequiredStatusChecks
	parameter := func(name string) UninterpretedPart {
		return UninterpretedPart{Kind: UninterpretedRuleParameter, Rule: pullRequest, Name: name}
	}
	statusChecksParameter := func(name string) UninterpretedPart {
		return UninterpretedPart{Kind: UninterpretedRuleParameter, Rule: statusChecks, Name: name}
	}
	cases := []struct {
		name   string
		change func(body map[string]any)
		want   []UninterpretedPart
	}{
		{
			name:   "target other than branch is uninterpreted",
			change: func(b map[string]any) { b["target"] = "tag" },
			want:   []UninterpretedPart{{Kind: UninterpretedTarget, Name: "tag"}},
		},
		{
			// GitHub が値を足し得る列挙で、足された値の意味を reco は知らないため、失敗にせず解釈しない部分にする
			name:   "enforcement other than disabled, active, and evaluate is uninterpreted",
			change: func(b map[string]any) { b["enforcement"] = "enforced" },
			want:   []UninterpretedPart{{Kind: UninterpretedEnforcement, Name: "enforced"}},
		},
		{
			name:   "conditions key other than ref_name is uninterpreted by its name",
			change: func(b map[string]any) { b["conditions"].(map[string]any)["repository_name"] = map[string]any{} },
			want:   []UninterpretedPart{{Kind: UninterpretedConditionsKey, Name: "repository_name"}},
		},
		{
			name:   "ref_name key other than include and exclude is uninterpreted under ref_name",
			change: func(b map[string]any) { refName(b)["match"] = "exact" },
			want:   []UninterpretedPart{{Kind: UninterpretedConditionsKey, Name: "ref_name.match"}},
		},
		{
			name: "unhandled actor type is uninterpreted without checking the other fields of the element",
			change: func(b map[string]any) {
				b["bypass_actors"] = []any{map[string]any{"actor_type": "OrganizationAdmin", "actor_id": "x", "extra": 1}}
			},
			want: []UninterpretedPart{{Kind: UninterpretedBypassActorType, Name: "OrganizationAdmin"}},
		},
		{
			name:   "unhandled bypass mode is uninterpreted",
			change: func(b map[string]any) { bypassActor(b, 0)["bypass_mode"] = "sometimes" },
			want:   []UninterpretedPart{{Kind: UninterpretedBypassMode, Name: "sometimes"}},
		},
		{
			name:   "unread bypass actor key is uninterpreted",
			change: func(b map[string]any) { bypassActor(b, 0)["actor_name"] = "maintain" },
			want:   []UninterpretedPart{{Kind: UninterpretedBypassActorKey, Name: "actor_name"}},
		},
		{
			name: "unhandled rule type is uninterpreted without checking the other fields of the element",
			change: func(b map[string]any) {
				b["rules"] = append(b["rules"].([]any), map[string]any{"type": "creation", "parameters": "x"})
			},
			want: []UninterpretedPart{{Kind: UninterpretedRuleType, Name: "creation"}},
		},
		{
			name:   "unread pull request rule key is uninterpreted with the rule type",
			change: func(b map[string]any) { rulesetRule(b, 1)["ruleset_source"] = "x" },
			want:   []UninterpretedPart{{Kind: UninterpretedRuleKey, Rule: pullRequest, Name: "ruleset_source"}},
		},
		{
			name:   "unread deletion rule key is uninterpreted with the rule type",
			change: func(b map[string]any) { rulesetRule(b, 0)["ruleset_source"] = "x" },
			want:   []UninterpretedPart{{Kind: UninterpretedRuleKey, Rule: deletion, Name: "ruleset_source"}},
		},
		{
			name:   "empty parameters of deletion are uninterpreted as a rule key",
			change: func(b map[string]any) { rulesetRule(b, 0)["parameters"] = map[string]any{} },
			want:   []UninterpretedPart{{Kind: UninterpretedRuleKey, Rule: deletion, Name: "parameters"}},
		},
		{
			name:   "null parameters of deletion are uninterpreted as a rule key",
			change: func(b map[string]any) { rulesetRule(b, 0)["parameters"] = nil },
			want:   []UninterpretedPart{{Kind: UninterpretedRuleKey, Rule: deletion, Name: "parameters"}},
		},
		{
			name:   "empty parameters of non_fast_forward are uninterpreted as a rule key",
			change: func(b map[string]any) { rulesetRule(b, 2)["parameters"] = map[string]any{} },
			want:   []UninterpretedPart{{Kind: UninterpretedRuleKey, Rule: nonFastForward, Name: "parameters"}},
		},
		{
			name:   "null parameters of non_fast_forward are uninterpreted as a rule key",
			change: func(b map[string]any) { rulesetRule(b, 2)["parameters"] = nil },
			want:   []UninterpretedPart{{Kind: UninterpretedRuleKey, Rule: nonFastForward, Name: "parameters"}},
		},
		{
			name:   "unread required_status_checks rule key is uninterpreted with the rule type",
			change: func(b map[string]any) { rulesetRule(b, 3)["ruleset_source"] = "x" },
			want:   []UninterpretedPart{{Kind: UninterpretedRuleKey, Rule: statusChecks, Name: "ruleset_source"}},
		},
		{
			name:   "missing do_not_enforce_on_create is uninterpreted",
			change: func(b map[string]any) { delete(statusChecksParameters(b), "do_not_enforce_on_create") },
			want:   []UninterpretedPart{statusChecksParameter("do_not_enforce_on_create")},
		},
		{
			name:   "null do_not_enforce_on_create is uninterpreted",
			change: func(b map[string]any) { statusChecksParameters(b)["do_not_enforce_on_create"] = nil },
			want:   []UninterpretedPart{statusChecksParameter("do_not_enforce_on_create")},
		},
		{
			name:   "non-boolean do_not_enforce_on_create is uninterpreted",
			change: func(b map[string]any) { statusChecksParameters(b)["do_not_enforce_on_create"] = "false" },
			want:   []UninterpretedPart{statusChecksParameter("do_not_enforce_on_create")},
		},
		{
			name:   "null integration_id is uninterpreted",
			change: func(b map[string]any) { statusCheck(b, 0)["integration_id"] = nil },
			want:   []UninterpretedPart{statusChecksParameter("required_status_checks.integration_id")},
		},
		{
			name:   "zero integration_id is uninterpreted",
			change: func(b map[string]any) { statusCheck(b, 1)["integration_id"] = 0 },
			want:   []UninterpretedPart{statusChecksParameter("required_status_checks.integration_id")},
		},
		{
			name:   "string integration_id is uninterpreted",
			change: func(b map[string]any) { statusCheck(b, 1)["integration_id"] = "15368" },
			want:   []UninterpretedPart{statusChecksParameter("required_status_checks.integration_id")},
		},
		{
			name:   "unread status check key is uninterpreted under required_status_checks",
			change: func(b map[string]any) { statusCheck(b, 0)["app_slug"] = "x" },
			want:   []UninterpretedPart{statusChecksParameter("required_status_checks.app_slug")},
		},
		{
			name:   "unread required_status_checks parameter is uninterpreted",
			change: func(b map[string]any) { statusChecksParameters(b)["enforce_on_merge_queue"] = true },
			want:   []UninterpretedPart{statusChecksParameter("enforce_on_merge_queue")},
		},
		{
			name: "required_status_checks parts are ordered by checked values before unread keys",
			change: func(b map[string]any) {
				p := statusChecksParameters(b)
				p["z_key"], p["a_key"], p["do_not_enforce_on_create"] = 1, 1, nil
				statusCheck(b, 1)["integration_id"], statusCheck(b, 1)["b_key"] = nil, 1
				rulesetRule(b, 3)["x"] = 1
			},
			want: []UninterpretedPart{
				{Kind: UninterpretedRuleKey, Rule: statusChecks, Name: "x"},
				statusChecksParameter("do_not_enforce_on_create"),
				statusChecksParameter("required_status_checks.integration_id"),
				statusChecksParameter("required_status_checks.b_key"),
				statusChecksParameter("a_key"),
				statusChecksParameter("z_key"),
			},
		},
		{
			name:   "empty allowed merge methods are uninterpreted",
			change: func(b map[string]any) { pullRequestParameters(b)["allowed_merge_methods"] = []any{} },
			want:   []UninterpretedPart{parameter("allowed_merge_methods")},
		},
		{
			name: "allowed merge methods with an undefined method are uninterpreted",
			change: func(b map[string]any) {
				pullRequestParameters(b)["allowed_merge_methods"] = []any{"merge", "squash", "rebase", "fast_forward"}
			},
			want: []UninterpretedPart{parameter("allowed_merge_methods")},
		},
		{
			name:   "missing allowed merge methods are uninterpreted",
			change: func(b map[string]any) { delete(pullRequestParameters(b), "allowed_merge_methods") },
			want:   []UninterpretedPart{parameter("allowed_merge_methods")},
		},
		{
			name:   "null allowed merge methods are uninterpreted",
			change: func(b map[string]any) { pullRequestParameters(b)["allowed_merge_methods"] = nil },
			want:   []UninterpretedPart{parameter("allowed_merge_methods")},
		},
		{
			name:   "allowed merge methods that are not an array are uninterpreted",
			change: func(b map[string]any) { pullRequestParameters(b)["allowed_merge_methods"] = "merge" },
			want:   []UninterpretedPart{parameter("allowed_merge_methods")},
		},
		{
			name: "allowed merge methods with a null element are uninterpreted",
			change: func(b map[string]any) {
				pullRequestParameters(b)["allowed_merge_methods"] = []any{"merge", "squash", "rebase", nil}
			},
			want: []UninterpretedPart{parameter("allowed_merge_methods")},
		},
		{
			name: "allowed merge methods with a non-string element are uninterpreted",
			change: func(b map[string]any) {
				pullRequestParameters(b)["allowed_merge_methods"] = []any{"merge", "squash", "rebase", 1}
			},
			want: []UninterpretedPart{parameter("allowed_merge_methods")},
		},
		{
			name: "null extra approval for unattributed changes is uninterpreted",
			change: func(b map[string]any) {
				pullRequestParameters(b)["require_extra_approval_for_unattributed_changes"] = nil
			},
			want: []UninterpretedPart{parameter("require_extra_approval_for_unattributed_changes")},
		},
		{
			name: "uninterpreted pull request parameters are ordered as the checked ones, then unread keys by name",
			change: func(b map[string]any) {
				p := pullRequestParameters(b)
				p["z_key"], p["a_key"], p["required_reviewers"] = 1, 1, nil
				p["require_extra_approval_for_unattributed_changes"], p["allowed_merge_methods"] = nil, nil
			},
			want: []UninterpretedPart{
				parameter("allowed_merge_methods"),
				parameter("require_extra_approval_for_unattributed_changes"),
				parameter("required_reviewers"),
				parameter("a_key"),
				parameter("z_key"),
			},
		},
		{
			name: "missing extra approval for unattributed changes is uninterpreted",
			change: func(b map[string]any) {
				delete(pullRequestParameters(b), "require_extra_approval_for_unattributed_changes")
			},
			want: []UninterpretedPart{parameter("require_extra_approval_for_unattributed_changes")},
		},
		{
			name: "non-boolean extra approval for unattributed changes is uninterpreted",
			change: func(b map[string]any) {
				pullRequestParameters(b)["require_extra_approval_for_unattributed_changes"] = "true"
			},
			want: []UninterpretedPart{parameter("require_extra_approval_for_unattributed_changes")},
		},
		{
			name: "non-empty required reviewers are uninterpreted",
			change: func(b map[string]any) {
				pullRequestParameters(b)["required_reviewers"] = []any{map[string]any{"reviewer_id": 1}}
			},
			want: []UninterpretedPart{parameter("required_reviewers")},
		},
		{
			name:   "missing required reviewers are uninterpreted",
			change: func(b map[string]any) { delete(pullRequestParameters(b), "required_reviewers") },
			want:   []UninterpretedPart{parameter("required_reviewers")},
		},
		{
			name:   "null required reviewers are uninterpreted",
			change: func(b map[string]any) { pullRequestParameters(b)["required_reviewers"] = nil },
			want:   []UninterpretedPart{parameter("required_reviewers")},
		},
		{
			name:   "required reviewers that are not an array are uninterpreted",
			change: func(b map[string]any) { pullRequestParameters(b)["required_reviewers"] = map[string]any{} },
			want:   []UninterpretedPart{parameter("required_reviewers")},
		},
		{
			name:   "dismissal restriction is uninterpreted",
			change: func(b map[string]any) { pullRequestParameters(b)["dismissal_restriction"] = map[string]any{} },
			want:   []UninterpretedPart{parameter("dismissal_restriction")},
		},
		{
			name:   "unread pull request parameter is uninterpreted",
			change: func(b map[string]any) { pullRequestParameters(b)["automatic_copilot_code_review_enabled"] = false },
			want:   []UninterpretedPart{parameter("automatic_copilot_code_review_enabled")},
		},
		{
			name: "uninterpreted parts are ordered target, enforcement, conditions, bypass actors, rules even when target is not branch",
			change: func(b map[string]any) {
				b["target"] = "push"
				b["enforcement"] = "enforced"
				refName(b)["match"] = "exact"
				bypassActor(b, 0)["bypass_mode"] = "sometimes"
				b["rules"] = append(b["rules"].([]any), map[string]any{"type": "file_path_restriction"})
			},
			want: []UninterpretedPart{
				{Kind: UninterpretedTarget, Name: "push"},
				{Kind: UninterpretedEnforcement, Name: "enforced"},
				{Kind: UninterpretedConditionsKey, Name: "ref_name.match"},
				{Kind: UninterpretedBypassMode, Name: "sometimes"},
				{Kind: UninterpretedRuleType, Name: "file_path_restriction"},
			},
		},
		{
			name: "array elements are ordered as in the response",
			change: func(b map[string]any) {
				b["rules"] = []any{map[string]any{"type": "update"}, map[string]any{"type": "creation"}, map[string]any{"type": "required_signatures"}}
			},
			want: []UninterpretedPart{
				{Kind: UninterpretedRuleType, Name: "update"},
				{Kind: UninterpretedRuleType, Name: "creation"},
				{Kind: UninterpretedRuleType, Name: "required_signatures"},
			},
		},
		{
			name: "the same part is kept only at its first place",
			change: func(b map[string]any) {
				b["bypass_actors"] = []any{
					map[string]any{"actor_type": "OrganizationAdmin", "actor_id": 1, "bypass_mode": "always"},
					map[string]any{"actor_type": "EnterpriseOwner", "actor_id": 1, "bypass_mode": "always"},
					map[string]any{"actor_type": "OrganizationAdmin", "actor_id": 2, "bypass_mode": "always"},
				}
			},
			want: []UninterpretedPart{
				{Kind: UninterpretedBypassActorType, Name: "OrganizationAdmin"},
				{Kind: UninterpretedBypassActorType, Name: "EnterpriseOwner"},
			},
		},
		{
			name: "the same name of another kind or another rule type is a different part",
			change: func(b map[string]any) {
				b["conditions"].(map[string]any)["x"] = 1
				bypassActor(b, 0)["x"] = 1
				rulesetRule(b, 0)["x"] = 1
				rulesetRule(b, 1)["x"] = 1
				pullRequestParameters(b)["x"] = 1
			},
			want: []UninterpretedPart{
				{Kind: UninterpretedConditionsKey, Name: "x"},
				{Kind: UninterpretedBypassActorKey, Name: "x"},
				{Kind: UninterpretedRuleKey, Rule: deletion, Name: "x"},
				{Kind: UninterpretedRuleKey, Rule: pullRequest, Name: "x"},
				parameter("x"),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeRuleset(rulesetDetail(t, tc.change))
			if err != nil {
				t.Fatalf("decodeRuleset() error = %v", err)
			}
			if value, ok := got.Value(); ok {
				t.Fatalf("decodeRuleset() value = %+v, want no value", value)
			}
			if got.ID != 42 || got.Name != "protect" {
				t.Errorf("decodeRuleset() ID, Name = %d, %q, want 42, %q", got.ID, got.Name, "protect")
			}
			if parts := got.UninterpretedParts(); !slices.Equal(parts, tc.want) {
				t.Errorf("decodeRuleset() uninterpreted parts = %+v, want %+v", parts, tc.want)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestDecodeRulesetUnexpectedResponse(t *testing.T) {
	requiredParameter := func(name string) func(b map[string]any) {
		return func(b map[string]any) { delete(pullRequestParameters(b), name) }
	}
	cases := []struct {
		name        string
		body        []byte
		errContains string
	}{
		{name: "non-JSON body", body: []byte(`<html>`)},
		{name: "array body", body: []byte(`[]`)},
		{name: "null body names every missing field", body: []byte(`null`), errContains: "missing fields: id, name, target, enforcement, conditions, bypass_actors, rules"},
		{name: "missing id", body: rulesetDetail(t, func(b map[string]any) { delete(b, "id") }), errContains: "missing fields: id"},
		{name: "string id", body: rulesetDetail(t, func(b map[string]any) { b["id"] = "42" }), errContains: "id"},
		{name: "fractional id", body: rulesetDetail(t, func(b map[string]any) { b["id"] = 4.2 }), errContains: "id"},
		{name: "null name", body: rulesetDetail(t, func(b map[string]any) { b["name"] = nil }), errContains: "missing fields: name"},
		{name: "non-string name", body: rulesetDetail(t, func(b map[string]any) { b["name"] = 1 }), errContains: "name"},
		{name: "missing target", body: rulesetDetail(t, func(b map[string]any) { delete(b, "target") }), errContains: "missing fields: target"},
		{name: "non-string target", body: rulesetDetail(t, func(b map[string]any) { b["target"] = 1 }), errContains: "target"},
		{name: "missing enforcement", body: rulesetDetail(t, func(b map[string]any) { delete(b, "enforcement") }), errContains: "missing fields: enforcement"},
		{
			// 解釈しない部分を見つけても、応答の全体を取得契約と照らす
			name: "unknown enforcement with null rules",
			body: rulesetDetail(t, func(b map[string]any) {
				b["enforcement"] = "enforced"
				b["rules"] = nil
			}),
			errContains: "missing fields: rules",
		},
		{name: "non-string enforcement", body: rulesetDetail(t, func(b map[string]any) { b["enforcement"] = true }), errContains: "enforcement"},
		{name: "missing conditions", body: rulesetDetail(t, func(b map[string]any) { delete(b, "conditions") }), errContains: "missing fields: conditions"},
		{name: "null conditions", body: rulesetDetail(t, func(b map[string]any) { b["conditions"] = nil }), errContains: "missing fields: conditions"},
		{name: "array conditions", body: rulesetDetail(t, func(b map[string]any) { b["conditions"] = []any{} }), errContains: "conditions"},
		{name: "null ref_name", body: rulesetDetail(t, func(b map[string]any) { b["conditions"].(map[string]any)["ref_name"] = nil }), errContains: "unsupported value: conditions.ref_name"},
		{name: "array ref_name", body: rulesetDetail(t, func(b map[string]any) { b["conditions"].(map[string]any)["ref_name"] = []any{} }), errContains: "conditions.ref_name"},
		{name: "missing include", body: rulesetDetail(t, func(b map[string]any) { delete(refName(b), "include") }), errContains: "missing fields: conditions.ref_name.include"},
		{name: "null exclude", body: rulesetDetail(t, func(b map[string]any) { refName(b)["exclude"] = nil }), errContains: "missing fields: conditions.ref_name.exclude"},
		{name: "string include", body: rulesetDetail(t, func(b map[string]any) { refName(b)["include"] = "main" }), errContains: "conditions.ref_name.include"},
		{name: "non-string include pattern", body: rulesetDetail(t, func(b map[string]any) { refName(b)["include"] = []any{1} }), errContains: "conditions.ref_name.include"},
		{name: "null exclude pattern", body: rulesetDetail(t, func(b map[string]any) { refName(b)["exclude"] = []any{nil} }), errContains: "conditions.ref_name.exclude"},
		{name: "null bypass actors", body: rulesetDetail(t, func(b map[string]any) { b["bypass_actors"] = nil }), errContains: "missing fields: bypass_actors"},
		{name: "object bypass actors", body: rulesetDetail(t, func(b map[string]any) { b["bypass_actors"] = map[string]any{} }), errContains: "bypass_actors"},
		{name: "null bypass actor", body: rulesetDetail(t, func(b map[string]any) { b["bypass_actors"] = []any{nil} }), errContains: "unsupported value: bypass_actors"},
		{name: "string bypass actor", body: rulesetDetail(t, func(b map[string]any) { b["bypass_actors"] = []any{"User"} }), errContains: "bypass_actors"},
		{name: "missing actor type", body: rulesetDetail(t, func(b map[string]any) { delete(bypassActor(b, 0), "actor_type") }), errContains: "missing fields: bypass_actors.actor_type"},
		{name: "non-string actor type", body: rulesetDetail(t, func(b map[string]any) { bypassActor(b, 0)["actor_type"] = 1 }), errContains: "bypass_actors.actor_type"},
		{name: "missing bypass mode", body: rulesetDetail(t, func(b map[string]any) { delete(bypassActor(b, 0), "bypass_mode") }), errContains: "missing fields: bypass_actors.bypass_mode"},
		{name: "non-string bypass mode", body: rulesetDetail(t, func(b map[string]any) { bypassActor(b, 0)["bypass_mode"] = 1 }), errContains: "bypass_actors.bypass_mode"},
		{name: "missing actor id of a repository role", body: rulesetDetail(t, func(b map[string]any) { delete(bypassActor(b, 0), "actor_id") }), errContains: "missing fields: bypass_actors.actor_id"},
		{name: "null actor id of a repository role", body: rulesetDetail(t, func(b map[string]any) { bypassActor(b, 0)["actor_id"] = nil }), errContains: "missing fields: bypass_actors.actor_id"},
		{name: "string actor id of a repository role", body: rulesetDetail(t, func(b map[string]any) { bypassActor(b, 0)["actor_id"] = "5" }), errContains: "bypass_actors.actor_id"},
		{name: "integer actor id of a deploy key", body: rulesetDetail(t, func(b map[string]any) {
			b["bypass_actors"] = []any{map[string]any{"actor_type": "DeployKey", "actor_id": 7, "bypass_mode": "always"}}
		}), errContains: "unsupported value: bypass_actors.actor_id"},
		{name: "missing rules", body: rulesetDetail(t, func(b map[string]any) { delete(b, "rules") }), errContains: "missing fields: rules"},
		{name: "null rules", body: rulesetDetail(t, func(b map[string]any) { b["rules"] = nil }), errContains: "missing fields: rules"},
		{name: "object rules", body: rulesetDetail(t, func(b map[string]any) { b["rules"] = map[string]any{} }), errContains: "rules"},
		{name: "null rule", body: rulesetDetail(t, func(b map[string]any) { b["rules"] = []any{nil} }), errContains: "unsupported value: rules"},
		{name: "missing rule type", body: rulesetDetail(t, func(b map[string]any) { delete(rulesetRule(b, 0), "type") }), errContains: "missing fields: rules.type"},
		{name: "non-string rule type", body: rulesetDetail(t, func(b map[string]any) { rulesetRule(b, 0)["type"] = 1 }), errContains: "rules.type"},
		{name: "missing pull request parameters", body: rulesetDetail(t, func(b map[string]any) { delete(rulesetRule(b, 1), "parameters") }), errContains: "missing fields: rules.parameters"},
		{name: "null pull request parameters", body: rulesetDetail(t, func(b map[string]any) { rulesetRule(b, 1)["parameters"] = nil }), errContains: "missing fields: rules.parameters"},
		{name: "array pull request parameters", body: rulesetDetail(t, func(b map[string]any) { rulesetRule(b, 1)["parameters"] = []any{} }), errContains: "rules.parameters"},
		{name: "missing required approving review count", body: rulesetDetail(t, requiredParameter("required_approving_review_count")), errContains: "missing fields: rules.parameters.required_approving_review_count"},
		{name: "missing dismiss stale reviews on push", body: rulesetDetail(t, requiredParameter("dismiss_stale_reviews_on_push")), errContains: "missing fields: rules.parameters.dismiss_stale_reviews_on_push"},
		{name: "missing require code owner review", body: rulesetDetail(t, requiredParameter("require_code_owner_review")), errContains: "missing fields: rules.parameters.require_code_owner_review"},
		{name: "missing require last push approval", body: rulesetDetail(t, requiredParameter("require_last_push_approval")), errContains: "missing fields: rules.parameters.require_last_push_approval"},
		{name: "missing required review thread resolution", body: rulesetDetail(t, requiredParameter("required_review_thread_resolution")), errContains: "missing fields: rules.parameters.required_review_thread_resolution"},
		{name: "null required approving review count", body: rulesetDetail(t, func(b map[string]any) { pullRequestParameters(b)["required_approving_review_count"] = nil }), errContains: "missing fields: rules.parameters.required_approving_review_count"},
		{name: "fractional required approving review count", body: rulesetDetail(t, func(b map[string]any) { pullRequestParameters(b)["required_approving_review_count"] = 1.5 }), errContains: "rules.parameters.required_approving_review_count"},
		{name: "string dismiss stale reviews on push", body: rulesetDetail(t, func(b map[string]any) { pullRequestParameters(b)["dismiss_stale_reviews_on_push"] = "true" }), errContains: "rules.parameters.dismiss_stale_reviews_on_push"},
		{name: "missing status checks parameters", body: rulesetDetail(t, func(b map[string]any) { delete(rulesetRule(b, 3), "parameters") }), errContains: "missing fields: rules.parameters"},
		{name: "array status checks parameters", body: rulesetDetail(t, func(b map[string]any) { rulesetRule(b, 3)["parameters"] = []any{} }), errContains: "rules.parameters"},
		{name: "missing status checks", body: rulesetDetail(t, func(b map[string]any) { delete(statusChecksParameters(b), "required_status_checks") }), errContains: "missing fields: rules.parameters.required_status_checks"},
		{name: "null status checks", body: rulesetDetail(t, func(b map[string]any) { statusChecksParameters(b)["required_status_checks"] = nil }), errContains: "missing fields: rules.parameters.required_status_checks"},
		{name: "object status checks", body: rulesetDetail(t, func(b map[string]any) { statusChecksParameters(b)["required_status_checks"] = map[string]any{} }), errContains: "rules.parameters.required_status_checks"},
		{name: "null status check", body: rulesetDetail(t, func(b map[string]any) { statusChecksParameters(b)["required_status_checks"] = []any{nil} }), errContains: "unsupported value: rules.parameters.required_status_checks"},
		{name: "missing context", body: rulesetDetail(t, func(b map[string]any) { delete(statusCheck(b, 0), "context") }), errContains: "missing fields: rules.parameters.required_status_checks.context"},
		{name: "non-string context", body: rulesetDetail(t, func(b map[string]any) { statusCheck(b, 0)["context"] = 1 }), errContains: "rules.parameters.required_status_checks.context"},
		{name: "missing strict policy", body: rulesetDetail(t, func(b map[string]any) { delete(statusChecksParameters(b), "strict_required_status_checks_policy") }), errContains: "missing fields: rules.parameters.strict_required_status_checks_policy"},
		{name: "string strict policy", body: rulesetDetail(t, func(b map[string]any) { statusChecksParameters(b)["strict_required_status_checks_policy"] = "true" }), errContains: "rules.parameters.strict_required_status_checks_policy"},
		{name: "contract violation wins over an uninterpreted part in the same response", body: rulesetDetail(t, func(b map[string]any) {
			b["target"] = "tag"
			delete(rulesetRule(b, 1), "parameters")
		}), errContains: "missing fields: rules.parameters"},
		{name: "contract violation after an uninterpreted element wins", body: rulesetDetail(t, func(b map[string]any) {
			b["bypass_actors"] = []any{
				map[string]any{"actor_type": "OrganizationAdmin"},
				map[string]any{"actor_type": "User", "actor_id": "7", "bypass_mode": "always"},
			}
		}), errContains: "bypass_actors.actor_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name+" is unexpected response", func(t *testing.T) {
			_, err := decodeRuleset(tc.body)
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeRuleset() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeRuleset() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

// TestDecodeRulesetMissingBypassActors は、権限の不足で返らない bypass_actors を空集合と読まず、観測操作の失敗にすることを確かめる。
//
//trace:checks QR-002
func TestDecodeRulesetMissingBypassActors(t *testing.T) {
	_, err := decodeRuleset(rulesetDetail(t, func(b map[string]any) { delete(b, "bypass_actors") }))
	if !errors.Is(err, ErrUnexpectedResponse) {
		t.Fatalf("decodeRuleset() error = %v, want %v", err, ErrUnexpectedResponse)
	}
	if want := "missing fields: bypass_actors"; !strings.Contains(err.Error(), want) {
		t.Errorf("decodeRuleset() error = %q, want it to contain %q", err, want)
	}
}

//trace:checks FR-004 QR-002
func TestDecodeRulesetSummaries(t *testing.T) {
	t.Run("summaries keep the list order", func(t *testing.T) {
		items := []json.RawMessage{
			json.RawMessage(`{"id": 2, "name": "b", "target": "branch"}`),
			json.RawMessage(`{"id": 1, "name": "a"}`),
		}
		got, err := decodeRulesetSummaries(items)
		if err != nil {
			t.Fatalf("decodeRulesetSummaries() error = %v", err)
		}
		want := []rulesetSummary{{id: 2, name: "b"}, {id: 1, name: "a"}}
		if !slices.Equal(got, want) {
			t.Errorf("decodeRulesetSummaries() = %+v, want %+v", got, want)
		}
	})

	failures := []struct {
		name        string
		items       []string
		errContains string
	}{
		{name: "missing id", items: []string{`{"name": "a"}`}, errContains: "missing fields: id"},
		{name: "null id", items: []string{`{"id": null, "name": "a"}`}, errContains: "missing fields: id"},
		{name: "string id", items: []string{`{"id": "1", "name": "a"}`}},
		{name: "missing name", items: []string{`{"id": 1}`}, errContains: "missing fields: name"},
		{name: "null name", items: []string{`{"id": 1, "name": null}`}, errContains: "missing fields: name"},
		{name: "non-string name", items: []string{`{"id": 1, "name": 1}`}},
		{name: "empty name", items: []string{`{"id": 1, "name": ""}`}, errContains: "unsupported value: name"},
		{name: "null element", items: []string{`null`}, errContains: "missing fields: id, name"},
		{name: "names that differ only in ASCII case", items: []string{`{"id": 1, "name": "Protect"}`, `{"id": 2, "name": "protect"}`}, errContains: "duplicate ruleset name"},
		{name: "names that differ only in non-ASCII case", items: []string{`{"id": 1, "name": "ÄB"}`, `{"id": 2, "name": "äb"}`}, errContains: "duplicate ruleset name"},
		{name: "contract violation in the second element", items: []string{`{"id": 1, "name": "a"}`, `{"id": 2}`}, errContains: "missing fields: name"},
	}
	for _, tc := range failures {
		t.Run(tc.name+" is unexpected response", func(t *testing.T) {
			items := make([]json.RawMessage, len(tc.items))
			for i, item := range tc.items {
				items[i] = json.RawMessage(item)
			}
			got, err := decodeRulesetSummaries(items)
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeRulesetSummaries() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if got != nil {
				t.Errorf("decodeRulesetSummaries() = %+v, want none on failure", got)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeRulesetSummaries() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}

	t.Run("names that differ beyond case are not duplicates", func(t *testing.T) {
		items := []json.RawMessage{json.RawMessage(`{"id": 1, "name": "protect"}`), json.RawMessage(`{"id": 2, "name": "protect "}`)}
		if _, err := decodeRulesetSummaries(items); err != nil {
			t.Errorf("decodeRulesetSummaries() error = %v, want nil", err)
		}
	})
}

func TestDecodeBypassActorIDUnknownTypePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("decodeBypassActorID() did not panic for an unknown actor type")
		}
	}()
	_, _ = decodeBypassActorID(setting.BypassActorType("OrganizationAdmin"), nil)
}

func TestDecodeHandledRulesetRuleUnknownTypePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("decodeHandledRulesetRule() did not panic for an unknown rule type")
		}
	}()
	_, _, _ = decodeHandledRulesetRule(setting.RulesetRuleType("creation"), map[string]json.RawMessage{})
}
