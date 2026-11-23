package changeplan

import (
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

func mustRulesetName(t *testing.T, name string) setting.RulesetName {
	t.Helper()
	rulesetName, err := setting.ParseRulesetName(name)
	if err != nil {
		t.Fatalf("ParseRulesetName(%q) error = %v", name, err)
	}
	return rulesetName
}

func rulesetUnit(t *testing.T, name string) setting.Unit {
	t.Helper()
	return setting.Unit{Key: setting.RulesetResource, Ruleset: mustRulesetName(t, name)}
}

// protectRuleset は、全体を解釈できた Ruleset の値として、既定のブランチの削除を制限し、Repository の admin に常に迂回を許す値を返す。
// 呼ぶたびに新しいスライスを作るため、ケースが値を変えてもほかのケースに及ばない。
func protectRuleset() setting.Ruleset {
	return setting.Ruleset{
		Target:       setting.RulesetTargetBranch,
		Enforcement:  setting.RulesetEnforcementActive,
		Conditions:   setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{}}},
		BypassActors: []setting.BypassActor{{ActorType: setting.BypassActorRepositoryRole, ActorID: 5, BypassMode: setting.BypassModeAlways}},
		Rules:        []setting.RulesetRule{setting.DeletionRule()},
	}
}
