package github

import (
	"fmt"
	"net/http"

	"github.com/tmknom/reco/internal/repository"
)

// UpdateOutcome は、一つの更新操作について、GitHub の応答から判断した結果である。
// ゼロ値を UpdateUnknown にするのは、結果を設定し損ねた値を成功や拒否と取り違えないため。
type UpdateOutcome int

const (
	UpdateUnknown UpdateOutcome = iota
	UpdateSucceeded
	UpdateRejected
)

// describeUpdateOutcome は更新操作の結果を判定し、失敗と成否不明の error に operation と対象を添える。
func describeUpdateOutcome(operation string, reference repository.Reference, status int, err error) (UpdateOutcome, error) {
	outcome := updateOutcomeOf(status, err)
	if outcome == UpdateSucceeded {
		return outcome, nil
	}
	if outcome == UpdateUnknown {
		err = fmt.Errorf("update outcome unknown: %w", err)
	}
	return outcome, fmt.Errorf("%s %s: %w", operation, reference, err)
}

// updateOutcomeOf は、更新要求の応答のステータスと失敗から更新操作の結果を判定する。
// 拒否とみなすのは 4xx に限る。応答を受け取れなかったときや 2xx・3xx・5xx の失敗は、
// GitHub が更新要求を処理したかを確かめられないため成否不明とする。
func updateOutcomeOf(status int, err error) UpdateOutcome {
	switch {
	case err == nil:
		return UpdateSucceeded
	case http.StatusBadRequest <= status && status < http.StatusInternalServerError:
		return UpdateRejected
	default:
		return UpdateUnknown
	}
}
