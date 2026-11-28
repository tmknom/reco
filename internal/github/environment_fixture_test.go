package github

import (
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// unprotectedEnvironment は、保護ルールのない Environment の保護設定の観測結果。観測は保護ルールがなければ担当者の集合を空集合の観測値にする。
func unprotectedEnvironment() EnvironmentProtection {
	return EnvironmentProtection{ReviewerSet: Observed(setting.ReviewerSetOfRule(nil))}
}

func mustEnvironmentName(t *testing.T, s string) setting.EnvironmentName {
	t.Helper()
	name, err := setting.ParseEnvironmentName(s)
	if err != nil {
		t.Fatalf("ParseEnvironmentName(%q) error = %v", s, err)
	}
	return name
}
