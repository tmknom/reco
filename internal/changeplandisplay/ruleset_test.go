package changeplandisplay

import (
	"testing"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// 期待値は CLI 仕様 4章の Ruleset の表示例と、値の変更の行の表から導く。
//
//trace:checks FR-005 FR-006 FR-007 QR-002
func TestPlannedChangeText_Rulesets(t *testing.T) {
	actor := func(actorType setting.BypassActorType, id int64, mode setting.BypassMode) setting.BypassActor {
		return setting.BypassActor{ActorType: actorType, ActorID: id, BypassMode: mode}
	}
	role := func(id int64, mode setting.BypassMode) setting.BypassActor {
		return actor(setting.BypassActorRepositoryRole, id, mode)
	}
	pullRequest := func(p setting.PullRequestParameters) setting.RulesetRule { return setting.PullRequestRule(p) }
	deletion := setting.DeletionRule()
	const pullRequestOfDefaults = "pull_request (required_approving_review_count: 0, dismiss_stale_reviews_on_push: false, require_code_owner_review: false, " +
		"require_last_push_approval: false, required_review_thread_resolution: false)"
	// base は、値の変更の各ケースが一つの項目だけを変える基準の値。呼ぶたびにスライスを共有しない値を返す。
	base := func() setting.Ruleset {
		return setting.Ruleset{
			Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
			Conditions:   refName([]string{"~DEFAULT_BRANCH"}, []string{}),
			BypassActors: []setting.BypassActor{role(5, setting.BypassModeAlways)},
			Rules:        []setting.RulesetRule{deletion},
		}
	}
	with := func(change func(*setting.Ruleset)) setting.Ruleset {
		r := base()
		change(&r)
		return r
	}
	protect := rulesetUnit(t, "protect")
	update := func(observed, desired setting.Ruleset) changeplan.SettingPlan {
		return changeplan.SettingPlan{
			Unit: protect, Observed: github.Observed(setting.RulesetValue(observed)), Desired: setting.RulesetValue(desired),
			Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
		}
	}
	const header = "  rulesets.protect:\n"
	cases := []struct {
		name string
		s    changeplan.SettingPlan
		want string
	}{
		{
			name: "matched ruleset is not shown",
			s:    changeplan.SettingPlan{Unit: protect, Result: changeplan.Match},
			want: "",
		},
		{
			name: "creation shows absent -> present and the whole declared value in the declared order",
			s: changeplan.SettingPlan{
				Unit: rulesetUnit(t, "protect-default-branch"), Observed: github.Observed(setting.Absent()),
				Desired: setting.RulesetValue(setting.Ruleset{
					Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
					Conditions:   refName([]string{"~DEFAULT_BRANCH"}, []string{}),
					BypassActors: []setting.BypassActor{role(5, setting.BypassModeAlways)},
					Rules:        []setting.RulesetRule{pullRequest(setting.PullRequestParameters{}), deletion},
				}),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  rulesets.protect-default-branch: absent -> present\n" +
				"    target: branch\n" +
				"    enforcement: active\n" +
				"    include: [\"~DEFAULT_BRANCH\"]\n" +
				"    exclude: []\n" +
				"    bypass_actors: [RepositoryRole 5 always]\n" +
				"    rules: [" + pullRequestOfDefaults + ", deletion]\n",
		},
		{
			name: "creation quotes and escapes each pattern and shows a deploy key without an id",
			s: changeplan.SettingPlan{
				Unit: protect, Observed: github.Observed(setting.Absent()),
				Desired: setting.RulesetValue(setting.Ruleset{
					Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementDisabled,
					Conditions: refName([]string{"refs/heads/main", "~ALL"}, []string{"refs/heads/a\"b\n"}),
					BypassActors: []setting.BypassActor{
						actor(setting.BypassActorDeployKey, 0, setting.BypassModeAlways), actor(setting.BypassActorUser, 583231, setting.BypassModePullRequest),
						actor(setting.BypassActorIntegration, 7, setting.BypassModeExempt),
					},
					Rules: []setting.RulesetRule{},
				}),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
			want: "  rulesets.protect: absent -> present\n" +
				"    target: branch\n" +
				"    enforcement: disabled\n" +
				"    include: [\"refs/heads/main\", \"~ALL\"]\n" +
				"    exclude: [\"refs/heads/a\\\"b\\n\"]\n" +
				"    bypass_actors: [DeployKey always, User 583231 pull_request, Integration 7 exempt]\n" +
				"    rules: []\n",
		},
		{
			name: "value update of the specification example shows only the changed parts",
			s: update(
				setting.Ruleset{
					Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementDisabled,
					Conditions:   refName([]string{"refs/heads/main"}, []string{}),
					BypassActors: []setting.BypassActor{actor(setting.BypassActorDeployKey, 0, setting.BypassModeAlways), role(5, setting.BypassModeAlways)},
					Rules:        []setting.RulesetRule{deletion, pullRequest(setting.PullRequestParameters{})},
				},
				setting.Ruleset{
					Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
					Conditions:   refName([]string{"~DEFAULT_BRANCH"}, []string{}),
					BypassActors: []setting.BypassActor{actor(setting.BypassActorUser, 583231, setting.BypassModePullRequest), role(5, setting.BypassModePullRequest)},
					Rules:        []setting.RulesetRule{pullRequest(setting.PullRequestParameters{RequiredApprovingReviewCount: 1, RequireCodeOwnerReview: true})},
				},
			),
			want: "  rulesets.protect:\n" +
				"    enforcement: disabled -> active\n" +
				"    Include patterns to remove: [\"refs/heads/main\"]\n" +
				"    Include patterns to add: [\"~DEFAULT_BRANCH\"]\n" +
				"    Bypass actors to remove: [DeployKey always]\n" +
				"    Bypass actors to add: [User 583231 pull_request]\n" +
				"    Bypass actor RepositoryRole 5: always -> pull_request\n" +
				"    Rules to remove: [deletion]\n" +
				"    Rule pull_request: required_approving_review_count: 0 -> 1, require_code_owner_review: false -> true\n",
		},
		{
			name: "every kind of line is shown in the order of the specification table",
			s: update(
				setting.Ruleset{
					Target: setting.RulesetTarget("tag"), Enforcement: setting.RulesetEnforcementEvaluate,
					Conditions:   refName([]string{"refs/heads/main"}, []string{"refs/heads/tmp"}),
					BypassActors: []setting.BypassActor{actor(setting.BypassActorUser, 1, setting.BypassModeAlways), role(5, setting.BypassModeAlways)},
					Rules:        []setting.RulesetRule{pullRequest(setting.PullRequestParameters{})},
				},
				setting.Ruleset{
					Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
					Conditions:   refName([]string{"~DEFAULT_BRANCH"}, []string{"refs/heads/wip"}),
					BypassActors: []setting.BypassActor{role(5, setting.BypassModePullRequest), actor(setting.BypassActorIntegration, 9, setting.BypassModeAlways)},
					Rules:        []setting.RulesetRule{pullRequest(setting.PullRequestParameters{RequiredApprovingReviewCount: 1}), deletion},
				},
			),
			want: header +
				"    target: tag -> branch\n" +
				"    enforcement: evaluate -> active\n" +
				"    Include patterns to remove: [\"refs/heads/main\"]\n" +
				"    Include patterns to add: [\"~DEFAULT_BRANCH\"]\n" +
				"    Exclude patterns to remove: [\"refs/heads/tmp\"]\n" +
				"    Exclude patterns to add: [\"refs/heads/wip\"]\n" +
				"    Bypass actors to remove: [User 1 always]\n" +
				"    Bypass actors to add: [Integration 9 always]\n" +
				"    Bypass actor RepositoryRole 5: always -> pull_request\n" +
				"    Rules to add: [deletion]\n" +
				"    Rule pull_request: required_approving_review_count: 0 -> 1\n",
		},
		{
			name: "conditions without ref_name is shown after enforcement and before the patterns to add",
			s: update(
				setting.Ruleset{
					Target: setting.RulesetTarget("tag"), Enforcement: setting.RulesetEnforcementDisabled,
					BypassActors: []setting.BypassActor{role(5, setting.BypassModeAlways)}, Rules: []setting.RulesetRule{deletion},
				},
				with(func(r *setting.Ruleset) {
					r.Conditions = refName([]string{"~DEFAULT_BRANCH"}, []string{"refs/heads/tmp"})
				}),
			),
			want: header +
				"    target: tag -> branch\n" +
				"    enforcement: disabled -> active\n" +
				"    conditions: {} -> ref_name\n" +
				"    Include patterns to add: [\"~DEFAULT_BRANCH\"]\n" +
				"    Exclude patterns to add: [\"refs/heads/tmp\"]\n",
		},
		{
			name: "conditions without ref_name to empty patterns shows only the conditions line",
			s: update(
				with(func(r *setting.Ruleset) { r.Conditions = setting.RulesetConditions{} }),
				with(func(r *setting.Ruleset) { r.Conditions = refName([]string{}, []string{}) }),
			),
			want: header + "    conditions: {} -> ref_name\n",
		},
		{
			name: "bypass modes of two actors changed are shown in the declared order",
			s: update(
				with(func(r *setting.Ruleset) {
					r.BypassActors = []setting.BypassActor{role(5, setting.BypassModeAlways), actor(setting.BypassActorUser, 1, setting.BypassModeAlways)}
				}),
				with(func(r *setting.Ruleset) {
					r.BypassActors = []setting.BypassActor{actor(setting.BypassActorUser, 1, setting.BypassModeExempt), role(5, setting.BypassModePullRequest)}
				}),
			),
			want: header +
				"    Bypass actor User 1: always -> exempt\n" +
				"    Bypass actor RepositoryRole 5: always -> pull_request\n",
		},
		{
			name: "two removed bypass actors of the same actor are not combined into a mode change",
			s: update(
				with(func(r *setting.Ruleset) {
					r.BypassActors = []setting.BypassActor{role(5, setting.BypassModeAlways), role(5, setting.BypassModeExempt)}
				}),
				with(func(r *setting.Ruleset) {
					r.BypassActors = []setting.BypassActor{role(5, setting.BypassModePullRequest)}
				}),
			),
			want: header +
				"    Bypass actors to remove: [RepositoryRole 5 always, RepositoryRole 5 exempt]\n" +
				"    Bypass actors to add: [RepositoryRole 5 pull_request]\n",
		},
		{
			name: "two added bypass actors of the same actor are not combined into a mode change",
			s: update(
				base(),
				with(func(r *setting.Ruleset) {
					r.BypassActors = []setting.BypassActor{role(5, setting.BypassModePullRequest), role(5, setting.BypassModeExempt)}
				}),
			),
			want: header +
				"    Bypass actors to remove: [RepositoryRole 5 always]\n" +
				"    Bypass actors to add: [RepositoryRole 5 pull_request, RepositoryRole 5 exempt]\n",
		},
		{
			name: "removed rule is shown with its parameters",
			s: update(
				with(func(r *setting.Ruleset) {
					r.Rules = []setting.RulesetRule{deletion, pullRequest(setting.PullRequestParameters{})}
				}),
				base(),
			),
			want: header + "    Rules to remove: [" + pullRequestOfDefaults + "]\n",
		},
		{
			name: "removed and added rules of different types are shown as remove then add",
			s: update(
				base(),
				with(func(r *setting.Ruleset) {
					r.Rules = []setting.RulesetRule{pullRequest(setting.PullRequestParameters{})}
				}),
			),
			want: header +
				"    Rules to remove: [deletion]\n" +
				"    Rules to add: [" + pullRequestOfDefaults + "]\n",
		},
		{
			name: "one changed pull_request parameter shows only that parameter",
			s: update(
				with(func(r *setting.Ruleset) {
					r.Rules = []setting.RulesetRule{pullRequest(setting.PullRequestParameters{})}
				}),
				with(func(r *setting.Ruleset) {
					r.Rules = []setting.RulesetRule{pullRequest(setting.PullRequestParameters{RequireLastPushApproval: true})}
				}),
			),
			want: header + "    Rule pull_request: require_last_push_approval: false -> true\n",
		},
		{
			name: "all changed pull_request parameters are shown in the parameter order",
			s: update(
				with(func(r *setting.Ruleset) {
					r.Rules = []setting.RulesetRule{pullRequest(setting.PullRequestParameters{
						RequiredApprovingReviewCount: 3, DismissStaleReviewsOnPush: true, RequireCodeOwnerReview: true, RequireLastPushApproval: true, RequiredReviewThreadResolution: true,
					})}
				}),
				with(func(r *setting.Ruleset) {
					r.Rules = []setting.RulesetRule{pullRequest(setting.PullRequestParameters{})}
				}),
			),
			want: header + "    Rule pull_request: required_approving_review_count: 3 -> 0, dismiss_stale_reviews_on_push: true -> false, " +
				"require_code_owner_review: true -> false, require_last_push_approval: true -> false, required_review_thread_resolution: true -> false\n",
		},
		{
			name: "two removed rules of the same type are not combined into a parameter change",
			s: update(
				with(func(r *setting.Ruleset) {
					r.Rules = []setting.RulesetRule{
						pullRequest(setting.PullRequestParameters{}), pullRequest(setting.PullRequestParameters{RequiredApprovingReviewCount: 2}),
					}
				}),
				with(func(r *setting.Ruleset) {
					r.Rules = []setting.RulesetRule{pullRequest(setting.PullRequestParameters{RequiredApprovingReviewCount: 1})}
				}),
			),
			want: header +
				"    Rules to remove: [" + pullRequestOfDefaults + ", pull_request (required_approving_review_count: 2, dismiss_stale_reviews_on_push: false, " +
				"require_code_owner_review: false, require_last_push_approval: false, required_review_thread_resolution: false)]\n" +
				"    Rules to add: [pull_request (required_approving_review_count: 1, dismiss_stale_reviews_on_push: false, " +
				"require_code_owner_review: false, require_last_push_approval: false, required_review_thread_resolution: false)]\n",
		},
		{
			name: "two added rules of the same type are not combined into a parameter change",
			s: update(
				with(func(r *setting.Ruleset) {
					r.Rules = []setting.RulesetRule{pullRequest(setting.PullRequestParameters{})}
				}),
				with(func(r *setting.Ruleset) {
					r.Rules = []setting.RulesetRule{
						pullRequest(setting.PullRequestParameters{RequiredApprovingReviewCount: 1}), pullRequest(setting.PullRequestParameters{RequiredApprovingReviewCount: 2}),
					}
				}),
			),
			want: header +
				"    Rules to remove: [" + pullRequestOfDefaults + "]\n" +
				"    Rules to add: [pull_request (required_approving_review_count: 1, dismiss_stale_reviews_on_push: false, " +
				"require_code_owner_review: false, require_last_push_approval: false, required_review_thread_resolution: false), " +
				"pull_request (required_approving_review_count: 2, dismiss_stale_reviews_on_push: false, " +
				"require_code_owner_review: false, require_last_push_approval: false, required_review_thread_resolution: false)]\n",
		},
		{
			name: "ruleset with uninterpreted parts shows the state, the whole declared value, the applicability, and each part in the observed order",
			s: changeplan.SettingPlan{
				Unit: rulesetUnit(t, "strict-review"), Observed: github.Unavailable[setting.Value](github.UnavailableUninterpretedParts),
				Desired: setting.RulesetValue(setting.Ruleset{
					Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
					Conditions: refName([]string{"~DEFAULT_BRANCH"}, []string{}), BypassActors: []setting.BypassActor{}, Rules: []setting.RulesetRule{deletion},
				}),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicableUninterpretedParts),
				UninterpretedParts: []github.UninterpretedPart{
					{Kind: github.UninterpretedRuleParameter, Rule: setting.RulesetRulePullRequest, Name: "allowed_merge_methods"},
					{Kind: github.UninterpretedRuleType, Name: "non_fast_forward"},
				},
			},
			want: "  rulesets.strict-review: (uninterpreted parts) -> present\n" +
				"    target: branch\n" +
				"    enforcement: active\n" +
				"    include: [\"~DEFAULT_BRANCH\"]\n" +
				"    exclude: []\n" +
				"    bypass_actors: []\n" +
				"    rules: [deletion]\n" +
				"    Applicability: not applicable (the ruleset on GitHub contains parts that reco does not interpret; remove this ruleset from the configuration)\n" +
				"      pull_request parameter: \"allowed_merge_methods\"\n" +
				"      rule type: \"non_fast_forward\"\n",
		},
		{
			name: "ruleset whose existence is unknown shows the state, the whole declared value, and why it is not applicable",
			s: changeplan.SettingPlan{
				Unit: rulesetUnit(t, "legacy"), Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined),
				Desired: setting.RulesetValue(setting.Ruleset{
					Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementDisabled,
					Conditions: refName([]string{"~ALL"}, []string{}), BypassActors: []setting.BypassActor{}, Rules: []setting.RulesetRule{},
				}),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicableNameUndetermined),
			},
			want: "  rulesets.legacy: (existence unknown) -> present\n" +
				"    target: branch\n" +
				"    enforcement: disabled\n" +
				"    include: [\"~ALL\"]\n" +
				"    exclude: []\n" +
				"    bypass_actors: []\n" +
				"    rules: []\n" +
				"    Applicability: not applicable (a ruleset on GitHub has a name reco cannot compare, such as one with non-ASCII characters; remove this ruleset from the configuration)\n",
		},
		{
			name: "ruleset of a private repository shows the state, the whole declared value, and the rulesets key to remove",
			s: changeplan.SettingPlan{
				Unit: rulesetUnit(t, "protect-default-branch"), Observed: github.Unavailable[setting.Value](github.UnavailablePrivateRepository),
				Desired: setting.RulesetValue(setting.Ruleset{
					Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
					Conditions: refName([]string{"~DEFAULT_BRANCH"}, []string{}), BypassActors: []setting.BypassActor{}, Rules: []setting.RulesetRule{deletion},
				}),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicablePrivateRepository),
			},
			want: "  rulesets.protect-default-branch: (private repository) -> present\n" +
				"    target: branch\n" +
				"    enforcement: active\n" +
				"    include: [\"~DEFAULT_BRANCH\"]\n" +
				"    exclude: []\n" +
				"    bypass_actors: []\n" +
				"    rules: [deletion]\n" +
				"    Applicability: not applicable (the repository is private; remove rulesets, which reco manages only on public repositories)\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PlannedChangeText(tc.s); got != tc.want {
				t.Errorf("PlannedChangeText() =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}

// 不一致の表示は必ず差の行を持ち、解釈しなかったものは理由が解釈しない部分のときだけ、一つ以上を伴う。
// 破れたまま表示すると、変更の中身や扱えない部分を示さない変更予定を利用者が承認し得るため、panic で検出する。
func TestPlannedChangeText_RulesetInvariantsPanic(t *testing.T) {
	protect := rulesetUnit(t, "protect")
	value := setting.Ruleset{
		Target: setting.RulesetTargetBranch, Enforcement: setting.RulesetEnforcementActive,
		Conditions:   refName([]string{"~DEFAULT_BRANCH"}, []string{}),
		BypassActors: []setting.BypassActor{}, Rules: []setting.RulesetRule{setting.DeletionRule(), setting.PullRequestRule(setting.PullRequestParameters{})},
	}
	reordered := value
	reordered.Rules = []setting.RulesetRule{setting.PullRequestRule(setting.PullRequestParameters{}), setting.DeletionRule()}
	withoutRefName := value
	withoutRefName.Conditions = setting.RulesetConditions{}
	withUndefinedRule := value
	withUndefinedRule.Rules = []setting.RulesetRule{{}}
	parts := []github.UninterpretedPart{{Kind: github.UninterpretedRuleType, Name: "non_fast_forward"}}
	cases := []struct {
		name string
		s    changeplan.SettingPlan
	}{
		{
			name: "mismatch whose values differ only in order has no difference to show",
			s: changeplan.SettingPlan{
				Unit: protect, Observed: github.Observed(setting.RulesetValue(value)), Desired: setting.RulesetValue(reordered),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
		},
		{
			name: "value update to a declared value without ref_name has no line in the specification",
			s: changeplan.SettingPlan{
				Unit: protect, Observed: github.Observed(setting.RulesetValue(value)), Desired: setting.RulesetValue(withoutRefName),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
		},
		{
			name: "creation of a declared value without ref_name has no line in the specification",
			s: changeplan.SettingPlan{
				Unit: protect, Observed: github.Observed(setting.Absent()), Desired: setting.RulesetValue(withoutRefName),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
		},
		{
			name: "creation with an undefined rule type has no format",
			s: changeplan.SettingPlan{
				Unit: protect, Observed: github.Observed(setting.Absent()), Desired: setting.RulesetValue(withUndefinedRule),
				Result: changeplan.Mismatch, Applicability: changeplan.Applicable(),
			},
		},
		{
			name: "uninterpreted parts as the reason without any part",
			s: changeplan.SettingPlan{
				Unit: protect, Observed: github.Unavailable[setting.Value](github.UnavailableUninterpretedParts), Desired: setting.RulesetValue(value),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicableUninterpretedParts),
			},
		},
		{
			name: "uninterpreted parts with another reason",
			s: changeplan.SettingPlan{
				Unit: protect, Observed: github.Unavailable[setting.Value](github.UnavailableNameUndetermined), Desired: setting.RulesetValue(value),
				Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable(changeplan.NotApplicableNameUndetermined),
				UninterpretedParts: parts,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("PlannedChangeText() did not panic")
				}
			}()
			_ = PlannedChangeText(tc.s)
		})
	}
}
