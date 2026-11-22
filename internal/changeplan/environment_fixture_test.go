package changeplan

import (
	"testing"

	"github.com/tmknom/reco/internal/github"
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

// withVariables は、Environment Variables の領域を読んだ観測の Environment として、e に変数の一覧 variables を持たせて返す。
func withVariables(e github.Environment, variables ...github.Variable) github.Environment {
	e.Variables = &github.Variables{Existing: variables}
	return e
}
