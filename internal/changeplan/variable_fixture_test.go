package changeplan

import (
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

func mustVariableName(t *testing.T, name string) setting.VariableName {
	t.Helper()
	variable, err := setting.ParseVariableName(name)
	if err != nil {
		t.Fatalf("ParseVariableName(%q) error = %v", name, err)
	}
	return variable
}

func variableUnit(t *testing.T, name string) setting.Unit {
	t.Helper()
	return setting.Unit{Key: setting.RepositoryVariable, Variable: mustVariableName(t, name)}
}
