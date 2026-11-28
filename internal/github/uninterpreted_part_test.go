package github

import (
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// 解釈しなかったものの種類ごとに、種類と、引用符で囲んでエスケープした名前の一行を確かめる。
//
//trace:checks FR-004 QR-002
func TestUninterpretedPart_String(t *testing.T) {
	cases := []struct {
		part UninterpretedPart
		want string
	}{
		{part: UninterpretedPart{Kind: UninterpretedTarget, Name: "tag"}, want: `target: "tag"`},
		{part: UninterpretedPart{Kind: UninterpretedEnforcement, Name: "enforced\n"}, want: `enforcement: "enforced\n"`},
		{part: UninterpretedPart{Kind: UninterpretedConditionsKey, Name: "ref_name.extra"}, want: `conditions key: "ref_name.extra"`},
		{part: UninterpretedPart{Kind: UninterpretedBypassActorType, Name: "Team"}, want: `bypass actor type: "Team"`},
		{part: UninterpretedPart{Kind: UninterpretedBypassMode, Name: "never"}, want: `bypass mode: "never"`},
		{part: UninterpretedPart{Kind: UninterpretedBypassActorKey, Name: "note"}, want: `bypass actor key: "note"`},
		{part: UninterpretedPart{Kind: UninterpretedRuleType, Name: "tag_name_pattern"}, want: `rule type: "tag_name_pattern"`},
		{part: UninterpretedPart{Kind: UninterpretedRuleKey, Rule: setting.RulesetRuleDeletion, Name: "extra"}, want: `deletion key: "extra"`},
		{part: UninterpretedPart{Kind: UninterpretedRuleParameter, Rule: setting.RulesetRulePullRequest, Name: "required_reviewers"}, want: `pull_request parameter: "required_reviewers"`},
		{part: UninterpretedPart{Kind: UninterpretedProtectionRuleType, Name: "custom_rule"}, want: `protection rule type: "custom_rule"`},
		// GitHub が返した任意の名前を、行の区切りや表示の向きを変えずに一行に収める
		{part: UninterpretedPart{Kind: UninterpretedRuleType, Name: "x\n\"y\"\u202e"}, want: `rule type: "x\n\"y\"\u202e"`},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.part.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("an undefined kind panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("String() did not panic")
			}
		}()
		_ = UninterpretedPart{Kind: UninterpretedKind(99), Name: "x"}.String()
	})
}
