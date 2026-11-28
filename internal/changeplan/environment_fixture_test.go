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

// unprotected は、保護ルールのない Environment の保護設定の観測結果。観測は保護ルールがなければ担当者の集合を空集合の観測値にする。
func unprotected() github.EnvironmentProtection {
	return github.EnvironmentProtection{ReviewerSet: github.Observed(setting.ReviewerSetOfRule(nil))}
}

// protectedByRule は、担当者の集合を作る入力 rule と自己承認の禁止 preventSelfReview の required_reviewers の保護ルールを持つ保護設定の観測結果。
// rule の担当者が空なら、担当者のいない保護ルールになる。
func protectedByRule(rule setting.RequiredReviewers, preventSelfReview bool) github.EnvironmentProtection {
	return github.EnvironmentProtection{ReviewerSet: github.Observed(setting.ReviewerSetOfRule(&rule)), PreventSelfReview: preventSelfReview}
}

// withVariables は、Environment Variables の領域を読んだ観測の Environment として、e に変数の一覧 variables を持たせて返す。
func withVariables(e github.Environment, variables ...github.Variable) github.Environment {
	e.Variables = &github.Variables{Existing: variables}
	return e
}

// readForDeletion は、削除で失われるものまで読んだ観測の Environment として、e に変数の一覧 variables と Secret の一覧 secrets を持たせて返す。
// withVariables と分けるのは、Secret を読んだ状態が、削除で失われるものを読む範囲に含めた Environment にだけ生じる別の状態だからである。
func readForDeletion(e github.Environment, variables []github.Variable, secrets ...github.EnvironmentSecret) github.Environment {
	e.Variables = &github.Variables{Existing: variables}
	e.Secrets = &github.EnvironmentSecrets{Existing: secrets}
	return e
}
