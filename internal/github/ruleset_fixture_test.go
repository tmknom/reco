package github

import (
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

func mustRulesetName(t *testing.T, s string) setting.RulesetName {
	t.Helper()
	name, err := setting.ParseRulesetName(s)
	if err != nil {
		t.Fatalf("ParseRulesetName(%q) error = %v", s, err)
	}
	return name
}

// interpretedRulesetValue は、全体を解釈できた Ruleset の値で、GitHub が詳細で返す形と同じく ref_name と空の集合を持つ。
func interpretedRulesetValue() setting.Ruleset {
	return setting.Ruleset{
		Target:       setting.RulesetTargetBranch,
		Enforcement:  setting.RulesetEnforcementActive,
		Conditions:   setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{}, Exclude: []string{}}},
		BypassActors: []setting.BypassActor{},
		Rules:        []setting.RulesetRule{setting.DeletionRule()},
	}
}
