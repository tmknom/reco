package configuration

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// rulesetBase は受理される Ruleset の宣言。拒否のケースは、この宣言の条件を 1 つだけ変えて作る。
const rulesetBase = `name: protect
target: branch
enforcement: active
conditions:
  ref_name:
    include: ["~DEFAULT_BRANCH"]
    exclude: []
bypass_actors:
  - actor_type: RepositoryRole
    actor_id: 5
    bypass_mode: always
rules:
  - type: deletion
`

// baseRuleset は rulesetBase を読んだ値。
func baseRuleset() setting.Ruleset {
	return setting.Ruleset{
		Target:       setting.RulesetTargetBranch,
		Enforcement:  setting.RulesetEnforcementActive,
		Conditions:   setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{}}},
		BypassActors: []setting.BypassActor{{ActorType: setting.BypassActorRepositoryRole, ActorID: 5, BypassMode: setting.BypassModeAlways}},
		Rules:        []setting.RulesetRule{setting.DeletionRule()},
	}
}

// rulesetSequence は、名前だけが違う rulesetBase の宣言を names の順に並べたシーケンスを作る。
func rulesetSequence(names ...string) string {
	var b strings.Builder
	for _, name := range names {
		item := strings.Replace(rulesetBase, "name: protect", "name: "+name, 1)
		b.WriteString("- " + strings.ReplaceAll(strings.TrimSuffix(item, "\n"), "\n", "\n  ") + "\n")
	}
	return b.String()
}

// indentLines は、s の各行の先頭に prefix を足す。
func indentLines(s, prefix string) string {
	return prefix + strings.ReplaceAll(strings.TrimSuffix(s, "\n"), "\n", "\n"+prefix) + "\n"
}

func mustRulesetName(t *testing.T, name string) setting.RulesetName {
	t.Helper()
	rulesetName, err := setting.ParseRulesetName(name)
	if err != nil {
		t.Fatalf("ParseRulesetName(%q) error = %v", name, err)
	}
	return rulesetName
}
