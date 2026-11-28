package github

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// secretUpdatedAt は、environmentSecretsBody が返す updated_at。
const secretUpdatedAt = "2026-01-02T03:04:05Z" //nolint:gosec // G101: Secret の更新日時で、認証情報ではない

// unprotectedEnvironment は、保護ルールのない Environment の保護設定の観測結果。観測は保護ルールがなければ担当者の集合を空集合の観測値にする。
func unprotectedEnvironment() EnvironmentProtection {
	return EnvironmentProtection{ReviewerSet: Observed(setting.ReviewerSetOfRule(nil))}
}

// environmentSecretsBody は、Environment Secret の一覧の1ページの応答本文を返す。total は全件数で、このページの要素数とは限らない。
// 要素は GitHub と同じく name、created_at、updated_at を持ち、updated_at は secretUpdatedAt にする。
func environmentSecretsBody(total int, names ...string) string {
	elements := make([]string, len(names))
	for i, name := range names {
		elements[i] = fmt.Sprintf(`{"name": %q, "created_at": "2026-01-01T00:00:00Z", "updated_at": %q}`, name, secretUpdatedAt)
	}
	return fmt.Sprintf(`{"total_count": %d, "secrets": [%s]}`, total, strings.Join(elements, ", "))
}

func mustEnvironmentName(t *testing.T, s string) setting.EnvironmentName {
	t.Helper()
	name, err := setting.ParseEnvironmentName(s)
	if err != nil {
		t.Fatalf("ParseEnvironmentName(%q) error = %v", s, err)
	}
	return name
}
