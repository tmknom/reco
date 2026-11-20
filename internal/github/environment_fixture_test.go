package github

import (
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

func mustEnvironmentName(t *testing.T, s string) setting.EnvironmentName {
	t.Helper()
	name, err := setting.ParseEnvironmentName(s)
	if err != nil {
		t.Fatalf("ParseEnvironmentName(%q) error = %v", s, err)
	}
	return name
}
