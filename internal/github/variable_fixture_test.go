package github

import (
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

func mustVariableName(t *testing.T, s string) setting.VariableName {
	t.Helper()
	name, err := setting.ParseVariableName(s)
	if err != nil {
		t.Fatalf("ParseVariableName(%q) error = %v", s, err)
	}
	return name
}
