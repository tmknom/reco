package github

import (
	"context"
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

// UpdateAllowAutoMerge は reference のリポジトリの allow_auto_merge を allow に更新する。
// error が nil になるのは UpdateSucceeded のときだけで、呼び出し側は error から結果を導かない。
func (c *Client) UpdateAllowAutoMerge(ctx context.Context, reference repository.Reference, allow bool) (UpdateOutcome, error) {
	body := fmt.Appendf(nil, `{"allow_auto_merge":%t}`, allow)
	outcome, err := c.updateRepository(ctx, reference, body)
	if err != nil {
		return outcome, fmt.Errorf("update repository %s: %w", reference, err)
	}
	return outcome, nil
}

// updateRepository は body を本文とする更新要求を送る。
// 成功応答の本文は、更新の契約に適合するかを確かめるためだけに解釈する。
func (c *Client) updateRepository(ctx context.Context, reference repository.Reference, body []byte) (UpdateOutcome, error) {
	status, data, err := c.send(ctx, http.MethodPatch, c.repositoryURL(reference), body)
	if err == nil {
		_, err = decodeRepository(data)
	}
	outcome := updateOutcomeOf(status, err)
	if outcome == UpdateUnknown {
		return outcome, fmt.Errorf("update outcome unknown: %w", err)
	}
	return outcome, err
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
