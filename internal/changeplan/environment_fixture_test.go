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

// lookup は、existing を一覧とする観測結果から、宣言の名前 name の Environment を引く。
func lookup(t *testing.T, name string, existing ...github.Environment) github.EnvironmentObservation {
	t.Helper()
	return github.Environments{Existing: existing}.Lookup(mustEnvironmentName(t, name))
}
