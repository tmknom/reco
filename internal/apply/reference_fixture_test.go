package apply

import (
	"testing"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}

func mustEnvironmentName(t *testing.T, name string) setting.EnvironmentName {
	t.Helper()
	environment, err := setting.ParseEnvironmentName(name)
	if err != nil {
		t.Fatalf("ParseEnvironmentName(%q) error = %v", name, err)
	}
	return environment
}

func mustRulesetName(t *testing.T, name string) setting.RulesetName {
	t.Helper()
	ruleset, err := setting.ParseRulesetName(name)
	if err != nil {
		t.Fatalf("ParseRulesetName(%q) error = %v", name, err)
	}
	return ruleset
}
