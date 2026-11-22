package changeplandisplay

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
