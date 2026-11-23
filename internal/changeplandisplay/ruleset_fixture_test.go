package changeplandisplay

import (
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

func rulesetUnit(t *testing.T, name string) setting.Unit {
	t.Helper()
	ruleset, err := setting.ParseRulesetName(name)
	if err != nil {
		t.Fatalf("ParseRulesetName(%q) error = %v", name, err)
	}
	return setting.Unit{Key: setting.RulesetResource, Ruleset: ruleset}
}

// refName は、include と exclude を持つ対象の条件を作る。宣言の値は必ず ref_name を持つ。
func refName(include, exclude []string) setting.RulesetConditions {
	return setting.RulesetConditions{RefName: &setting.RefNameCondition{Include: include, Exclude: exclude}}
}
