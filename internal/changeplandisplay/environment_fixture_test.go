package changeplandisplay

import (
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

func mustEnvironmentName(t *testing.T, name string) setting.EnvironmentName {
	t.Helper()
	environment, err := setting.ParseEnvironmentName(name)
	if err != nil {
		t.Fatalf("ParseEnvironmentName(%q) error = %v", name, err)
	}
	return environment
}
