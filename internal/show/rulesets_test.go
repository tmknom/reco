package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestWriteRulesets(t *testing.T) {
	const (
		unavailableLine = "        # unavailable (the ruleset contains parts that reco does not interpret)\n"
		emptyBody       = "        target: branch\n        enforcement: active\n        conditions:\n          ref_name:\n            include: []\n            exclude: []\n" +
			"        bypass_actors: []\n        rules: []\n"
	)
	active := func(conditions setting.RulesetConditions, actors []setting.BypassActor, rules []setting.RulesetRule) setting.Ruleset {
		return setting.Ruleset{Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive, Conditions: conditions, BypassActors: actors, Rules: rules}
	}
	emptyRefName := setting.RulesetConditions{RefName: &setting.RefNameCondition{}}
	// observedPullRequest は、観測した値と同じく、省略できるパラメータを持つ pull_request のルールを作る。持たない p の項目には GitHub の既定値を入れる。
	observedPullRequest := func(p setting.PullRequestParameters) setting.RulesetRule {
		if p.AllowedMergeMethods == nil {
			p.AllowedMergeMethods = []setting.AllowedMergeMethod{setting.AllowedMergeMethodMerge, setting.AllowedMergeMethodSquash, setting.AllowedMergeMethodRebase}
		}
		if p.RequireExtraApprovalForUnattributedChanges == nil {
			p.RequireExtraApprovalForUnattributedChanges = new(true)
		}
		return setting.PullRequestRule(p)
	}
	const defaultOptionalLines = "              allowed_merge_methods:\n                - merge\n                - squash\n                - rebase\n" +
		"              require_extra_approval_for_unattributed_changes: true\n"
	observed := func(rulesets ...github.Ruleset) github.Rulesets {
		return github.Rulesets{Existing: github.Observed(rulesets)}
	}
	cases := []struct {
		name string
		in   github.Rulesets
		want string
	}{
		{
			name: "private repository is shown as unavailable in a comment",
			in:   github.Rulesets{Existing: github.Unavailable[[]github.Ruleset](github.UnavailablePrivateRepository)},
			want: "    # rulesets: unavailable (the repository is private)\n",
		},
		{
			name: "no rulesets is shown as none in a comment",
			in:   observed(),
			want: "    # rulesets: none\n",
		},
		{
			name: "the example ruleset is shown as in the CLI specification including the deploy key without actor_id",
			in: observed(github.InterpretedRuleset(1, "protect-default-branch", active(
				setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}}},
				[]setting.BypassActor{
					{ActorType: setting.BypassActorRepositoryRole, ActorID: 5, BypassMode: setting.BypassModeAlways},
					{ActorType: setting.BypassActorDeployKey, BypassMode: setting.BypassModeAlways},
				},
				[]setting.RulesetRule{observedPullRequest(setting.PullRequestParameters{RequiredApprovingReviewCount: 1}), setting.DeletionRule()},
			))),
			want: "    rulesets:\n" +
				"      - name: \"protect-default-branch\"\n" +
				"        target: branch\n" +
				"        enforcement: active\n" +
				"        conditions:\n" +
				"          ref_name:\n" +
				"            include:\n" +
				"              - \"~DEFAULT_BRANCH\"\n" +
				"            exclude: []\n" +
				"        bypass_actors:\n" +
				"          - actor_type: RepositoryRole\n" +
				"            actor_id: 5\n" +
				"            bypass_mode: always\n" +
				"          - actor_type: DeployKey\n" +
				"            bypass_mode: always\n" +
				"        rules:\n" +
				"          - type: pull_request\n" +
				"            parameters:\n" +
				"              required_approving_review_count: 1\n" +
				"              dismiss_stale_reviews_on_push: false\n" +
				"              require_code_owner_review: false\n" +
				"              require_last_push_approval: false\n" +
				"              required_review_thread_resolution: false\n" +
				defaultOptionalLines +
				"          - type: deletion\n",
		},
		{
			name: "opposite values of the enforcement, the pull_request parameters, and the actor are shown as observed",
			in: observed(github.InterpretedRuleset(1, "n", setting.Ruleset{
				Target:       setting.RulesetTargetBranch,
				Enforcement:  setting.RulesetEnforcementEvaluate,
				Conditions:   emptyRefName,
				BypassActors: []setting.BypassActor{{ActorType: setting.BypassActorUser, ActorID: 7, BypassMode: setting.BypassModePullRequest}},
				// 各ブール値を別々に true にして、項目の取り違えを検出する
				Rules: []setting.RulesetRule{
					observedPullRequest(setting.PullRequestParameters{
						RequiredApprovingReviewCount: 2, DismissStaleReviewsOnPush: true,
						AllowedMergeMethods:                        []setting.AllowedMergeMethod{setting.AllowedMergeMethodSquash, setting.AllowedMergeMethodSquash, setting.AllowedMergeMethodMerge},
						RequireExtraApprovalForUnattributedChanges: new(false),
					}),
					observedPullRequest(setting.PullRequestParameters{RequireCodeOwnerReview: true}),
					observedPullRequest(setting.PullRequestParameters{RequireLastPushApproval: true}),
					observedPullRequest(setting.PullRequestParameters{RequiredReviewThreadResolution: true}),
				},
			})),
			want: "    rulesets:\n      - name: \"n\"\n        target: branch\n        enforcement: evaluate\n" +
				"        conditions:\n          ref_name:\n            include: []\n            exclude: []\n" +
				"        bypass_actors:\n          - actor_type: User\n            actor_id: 7\n            bypass_mode: pull_request\n" +
				"        rules:\n" +
				"          - type: pull_request\n            parameters:\n              required_approving_review_count: 2\n" +
				"              dismiss_stale_reviews_on_push: true\n              require_code_owner_review: false\n" +
				"              require_last_push_approval: false\n              required_review_thread_resolution: false\n" +
				"              allowed_merge_methods:\n                - squash\n                - squash\n                - merge\n" +
				"              require_extra_approval_for_unattributed_changes: false\n" +
				"          - type: pull_request\n            parameters:\n              required_approving_review_count: 0\n" +
				"              dismiss_stale_reviews_on_push: false\n              require_code_owner_review: true\n" +
				"              require_last_push_approval: false\n              required_review_thread_resolution: false\n" + defaultOptionalLines +
				"          - type: pull_request\n            parameters:\n              required_approving_review_count: 0\n" +
				"              dismiss_stale_reviews_on_push: false\n              require_code_owner_review: false\n" +
				"              require_last_push_approval: true\n              required_review_thread_resolution: false\n" + defaultOptionalLines +
				"          - type: pull_request\n            parameters:\n              required_approving_review_count: 0\n" +
				"              dismiss_stale_reviews_on_push: false\n              require_code_owner_review: false\n" +
				"              require_last_push_approval: false\n              required_review_thread_resolution: true\n" + defaultOptionalLines,
		},
		{
			name: "non_fast_forward and required_status_checks are shown with quoted checks in the returned order with duplicates",
			in: observed(github.InterpretedRuleset(1, "n", active(emptyRefName, nil, []setting.RulesetRule{
				setting.NonFastForwardRule(),
				setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{
					RequiredStatusChecks:             []setting.StatusCheck{{Context: "test"}, {Context: "lint", IntegrationID: 15368}, {Context: "a\"b\n# x:"}, {Context: "test"}},
					StrictRequiredStatusChecksPolicy: true,
					DoNotEnforceOnCreate:             new(false),
				}),
				setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{RequiredStatusChecks: []setting.StatusCheck{}, DoNotEnforceOnCreate: new(true)}),
			}))),
			want: "    rulesets:\n      - name: \"n\"\n        target: branch\n        enforcement: active\n" +
				"        conditions:\n          ref_name:\n            include: []\n            exclude: []\n        bypass_actors: []\n" +
				"        rules:\n" +
				"          - type: non_fast_forward\n" +
				"          - type: required_status_checks\n            parameters:\n              required_status_checks:\n" +
				"                - context: \"test\"\n" +
				"                - context: \"lint\"\n                  integration_id: 15368\n" +
				"                - context: \"a\\\"b\\n# x:\"\n" +
				"                - context: \"test\"\n" +
				"              strict_required_status_checks_policy: true\n              do_not_enforce_on_create: false\n" +
				"          - type: required_status_checks\n            parameters:\n              required_status_checks: []\n" +
				"              strict_required_status_checks_policy: false\n              do_not_enforce_on_create: true\n",
		},
		{
			name: "conditions without ref_name is shown as an empty mapping",
			in:   observed(github.InterpretedRuleset(1, "n", active(setting.RulesetConditions{}, nil, nil))),
			want: "    rulesets:\n      - name: \"n\"\n        target: branch\n        enforcement: active\n        conditions: {}\n        bypass_actors: []\n        rules: []\n",
		},
		{
			name: "empty include, exclude, bypass_actors, and rules are shown as empty sequences",
			in:   observed(github.InterpretedRuleset(1, "n", active(emptyRefName, nil, nil))),
			want: "    rulesets:\n      - name: \"n\"\n" + emptyBody,
		},
		{
			name: "include and exclude patterns are shown one quoted pattern per line in the returned order",
			in: observed(github.InterpretedRuleset(1, "n", active(
				setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"b", "a"}, Exclude: []string{"c"}}}, nil, nil))),
			want: "    rulesets:\n      - name: \"n\"\n        target: branch\n        enforcement: active\n" +
				"        conditions:\n          ref_name:\n            include:\n              - \"b\"\n              - \"a\"\n" +
				"            exclude:\n              - \"c\"\n        bypass_actors: []\n        rules: []\n",
		},
		{
			name: "interpreted and uninterpreted rulesets are shown in the listed order",
			in: observed(
				github.InterpretedRuleset(1, "first", active(emptyRefName, nil, nil)),
				github.UninterpretedRuleset(2, "second", []github.UninterpretedPart{{Kind: github.UninterpretedTarget, Name: "tag"}}),
				github.InterpretedRuleset(3, "third", active(emptyRefName, nil, nil)),
			),
			want: "    rulesets:\n      - name: \"first\"\n" + emptyBody +
				"      - name: \"second\"\n" + unavailableLine + "        #   target: \"tag\"\n" +
				"      - name: \"third\"\n" + emptyBody,
		},
		{
			name: "every kind of uninterpreted part is shown in its line form in the given order",
			in: observed(github.UninterpretedRuleset(1, "n", []github.UninterpretedPart{
				{Kind: github.UninterpretedTarget, Name: "tag"},
				{Kind: github.UninterpretedConditionsKey, Name: "ref_name.extra"},
				{Kind: github.UninterpretedBypassActorType, Name: "Team"},
				{Kind: github.UninterpretedBypassMode, Name: "never"},
				{Kind: github.UninterpretedBypassActorKey, Name: "note"},
				{Kind: github.UninterpretedRuleType, Name: "tag_name_pattern"},
				{Kind: github.UninterpretedRuleKey, Rule: setting.RulesetRuleDeletion, Name: "extra"},
				{Kind: github.UninterpretedRuleParameter, Rule: setting.RulesetRulePullRequest, Name: "required_reviewers"},
			})),
			want: "    rulesets:\n      - name: \"n\"\n" + unavailableLine +
				"        #   target: \"tag\"\n" +
				"        #   conditions key: \"ref_name.extra\"\n" +
				"        #   bypass actor type: \"Team\"\n" +
				"        #   bypass mode: \"never\"\n" +
				"        #   bypass actor key: \"note\"\n" +
				"        #   rule type: \"tag_name_pattern\"\n" +
				"        #   deletion key: \"extra\"\n" +
				"        #   pull_request parameter: \"required_reviewers\"\n",
		},
		{
			name: "a ruleset name with a quote, a newline, and a key-like text is escaped into one line",
			in:   observed(github.InterpretedRuleset(1, "a\"b\n# x:", active(emptyRefName, nil, nil))),
			want: "    rulesets:\n      - name: \"a\\\"b\\n# x:\"\n" + emptyBody,
		},
		{
			name: "a pattern with a quote and a newline is escaped into one line",
			in:   observed(github.InterpretedRuleset(1, "n", active(setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"\"\n    general:"}}}, nil, nil))),
			want: "    rulesets:\n      - name: \"n\"\n        target: branch\n        enforcement: active\n" +
				"        conditions:\n          ref_name:\n            include:\n              - \"\\\"\\n    general:\"\n" +
				"            exclude: []\n        bypass_actors: []\n        rules: []\n",
		},
		{
			name: "an uninterpreted part name with a newline and a bidirectional control is escaped into one line",
			in: observed(github.UninterpretedRuleset(1, "n", []github.UninterpretedPart{
				{Kind: github.UninterpretedRuleType, Name: "x\n      - name: \"y\"\u202e"},
			})),
			want: "    rulesets:\n      - name: \"n\"\n" + unavailableLine + "        #   rule type: \"x\\n      - name: \\\"y\\\"\\u202e\"\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeRulesets(&b, tc.in)
			if b.String() != tc.want {
				t.Errorf("writeRulesets() wrote %q, want %q", b.String(), tc.want)
			}
		})
	}

	panics := []struct {
		name string
		in   github.Rulesets
	}{
		{
			name: "rulesets unavailable for a reason other than private panics",
			in:   github.Rulesets{Existing: github.Unavailable[[]github.Ruleset](github.UnavailableActionsDisabled)},
		},
		{
			name: "an undefined kind of uninterpreted part panics",
			in:   observed(github.UninterpretedRuleset(1, "n", []github.UninterpretedPart{{Kind: github.UninterpretedKind(99), Name: "x"}})),
		},
		{
			// setting の外からは、ゼロ値だけが定義にないルール型のルールになる
			name: "a rule of an undefined rule type panics",
			in: observed(github.InterpretedRuleset(1, "n", setting.Ruleset{
				Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive, Rules: []setting.RulesetRule{{}},
			})),
		},
		{
			name: "a required_status_checks rule without do_not_enforce_on_create panics",
			in: observed(github.InterpretedRuleset(1, "n", setting.Ruleset{
				Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
				Rules: []setting.RulesetRule{setting.RequiredStatusChecksRule(setting.RequiredStatusChecksParameters{})},
			})),
		},
		{
			name: "a pull_request rule without allowed_merge_methods panics",
			in: observed(github.InterpretedRuleset(1, "n", setting.Ruleset{
				Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
				Rules: []setting.RulesetRule{setting.PullRequestRule(setting.PullRequestParameters{RequireExtraApprovalForUnattributedChanges: new(true)})},
			})),
		},
		{
			name: "a pull_request rule without require_extra_approval_for_unattributed_changes panics",
			in: observed(github.InterpretedRuleset(1, "n", setting.Ruleset{
				Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
				Rules: []setting.RulesetRule{setting.PullRequestRule(setting.PullRequestParameters{AllowedMergeMethods: []setting.AllowedMergeMethod{setting.AllowedMergeMethodMerge}})},
			})),
		},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("writeRulesets() did not panic")
				}
			}()
			writeRulesets(&strings.Builder{}, tc.in)
		})
	}
}
