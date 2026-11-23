package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// writeRulesets は rulesets を書く。
// 非公開以外の理由は Rulesets の一覧に当たらず、非公開の文言で黙って示さないよう不変条件の破れとして panic する。
func writeRulesets(b *strings.Builder, r github.Rulesets) {
	rulesets, ok := r.Existing.Value()
	if !ok {
		if reason := r.Existing.UnavailableReason(); reason != github.UnavailablePrivateRepository {
			panic(fmt.Sprintf("show: undefined unavailable reason %d for rulesets", int(reason)))
		}
		b.WriteString("    # rulesets: unavailable (the repository is private)\n")
		return
	}
	if len(rulesets) == 0 {
		b.WriteString("    # rulesets: none\n")
		return
	}
	b.WriteString("    rulesets:\n")
	for _, ruleset := range rulesets {
		fmt.Fprintf(b, "      - name: %s\n", quote(ruleset.Name))
		if value, ok := ruleset.Value(); ok {
			writeRulesetValue(b, value)
			continue
		}
		writeUninterpretedParts(b, ruleset.UninterpretedParts())
	}
}

// writeRulesetValue は、解釈できた Ruleset の name 以外のキーを書く。
func writeRulesetValue(b *strings.Builder, v setting.Ruleset) {
	fmt.Fprintf(b, "        target: %s\n        enforcement: %s\n", v.Target, v.Enforcement)
	if ref := v.Conditions.RefName; ref != nil {
		b.WriteString("        conditions:\n          ref_name:\n")
		writeRefNamePatterns(b, "include", ref.Include)
		writeRefNamePatterns(b, "exclude", ref.Exclude)
	} else {
		b.WriteString("        conditions: {}\n")
	}
	if len(v.BypassActors) == 0 {
		b.WriteString("        bypass_actors: []\n")
	} else {
		b.WriteString("        bypass_actors:\n")
		for _, actor := range v.BypassActors {
			fmt.Fprintf(b, "          - actor_type: %s\n", actor.ActorType)
			// DeployKey は ID を持たず、0 を ID として示さない
			if actor.ActorType != setting.BypassActorDeployKey {
				fmt.Fprintf(b, "            actor_id: %d\n", actor.ActorID)
			}
			fmt.Fprintf(b, "            bypass_mode: %s\n", actor.BypassMode)
		}
	}
	if len(v.Rules) == 0 {
		b.WriteString("        rules: []\n")
		return
	}
	b.WriteString("        rules:\n")
	for _, rule := range v.Rules {
		writeRulesetRule(b, rule)
	}
}

// writeRulesetRule はルールを一つ書く。
// パラメータを持つルール型を足して case を足し忘れ、パラメータを黙って省かないよう、lint の exhaustive が検出する形にし、default を置かず、定義にない型は panic する。
func writeRulesetRule(b *strings.Builder, rule setting.RulesetRule) {
	switch rule.Type() {
	case setting.RulesetRulePullRequest:
		p := rule.PullRequest()
		fmt.Fprintf(b, "          - type: %s\n            parameters:\n              required_approving_review_count: %d\n              dismiss_stale_reviews_on_push: %t\n"+
			"              require_code_owner_review: %t\n              require_last_push_approval: %t\n              required_review_thread_resolution: %t\n",
			rule.Type(), p.RequiredApprovingReviewCount, p.DismissStaleReviewsOnPush, p.RequireCodeOwnerReview, p.RequireLastPushApproval, p.RequiredReviewThreadResolution)
		return
	case setting.RulesetRuleDeletion:
		fmt.Fprintf(b, "          - type: %s\n", rule.Type())
		return
	}
	panic(fmt.Sprintf("show: undefined ruleset rule type %q", rule.Type()))
}

// writeRefNamePatterns は ref_name の include か exclude を、空なら [] で、そうでなければ一行に一つずつ書く。
func writeRefNamePatterns(b *strings.Builder, key string, patterns []string) {
	if len(patterns) == 0 {
		fmt.Fprintf(b, "            %s: []\n", key)
		return
	}
	fmt.Fprintf(b, "            %s:\n", key)
	for _, pattern := range patterns {
		fmt.Fprintf(b, "              - %s\n", quote(pattern))
	}
}

// writeUninterpretedParts は、解釈しなかったものを YAML のコメントで一行に一つずつ書く。値として読ませない。
func writeUninterpretedParts(b *strings.Builder, parts []github.UninterpretedPart) {
	b.WriteString("        # unavailable (the ruleset contains parts that reco does not interpret)\n")
	for _, part := range parts {
		fmt.Fprintf(b, "        #   %s\n", uninterpretedPartText(part))
	}
}

// uninterpretedPartText は解釈しなかったもの一つの行の形を返す。
// 種類を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かず、定義にない値は panic する。
func uninterpretedPartText(p github.UninterpretedPart) string {
	switch p.Kind {
	case github.UninterpretedTarget:
		return "target: " + quote(p.Name)
	case github.UninterpretedConditionsKey:
		return "conditions key: " + quote(p.Name)
	case github.UninterpretedBypassActorType:
		return "bypass actor type: " + quote(p.Name)
	case github.UninterpretedBypassMode:
		return "bypass mode: " + quote(p.Name)
	case github.UninterpretedBypassActorKey:
		return "bypass actor key: " + quote(p.Name)
	case github.UninterpretedRuleType:
		return "rule type: " + quote(p.Name)
	case github.UninterpretedRuleKey:
		return string(p.Rule) + " key: " + quote(p.Name)
	case github.UninterpretedRuleParameter:
		return string(p.Rule) + " parameter: " + quote(p.Name)
	}
	panic(fmt.Sprintf("show: undefined uninterpreted kind %d", int(p.Kind)))
}
