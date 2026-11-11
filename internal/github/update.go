package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tmknom/reco/internal/repository"
)

// UpdateAllowAutoMerge は reference のリポジトリの allow_auto_merge を allow に更新する。
// 返す error は、ErrUpdateOutcomeUnknown を含めば成否不明、含まなければ GitHub による拒否である。
func (c *Client) UpdateAllowAutoMerge(ctx context.Context, reference repository.Reference, allow bool) error {
	body := fmt.Appendf(nil, `{"allow_auto_merge":%t}`, allow)
	if err := c.updateRepository(ctx, reference, body); err != nil {
		return fmt.Errorf("update repository %s: %w", reference, err)
	}
	return nil
}

// updateRepository は body を本文とする更新要求を送る。
// 成功応答の本文は、更新の契約に適合するかを確かめるためだけに解釈する。
func (c *Client) updateRepository(ctx context.Context, reference repository.Reference, body []byte) error {
	status, data, err := c.send(ctx, http.MethodPatch, c.repositoryURL(reference), body)
	if err == nil {
		_, err = decodeRepository(data)
	}
	if err != nil && isOutcomeUnknown(status) {
		return fmt.Errorf("%w: %w", ErrUpdateOutcomeUnknown, err)
	}
	return err
}

// isOutcomeUnknown は、更新要求が失敗に終わったときのステータスから、成否を確認できないかを判定する。
func isOutcomeUnknown(status int) bool {
	return status < http.StatusBadRequest || status >= http.StatusInternalServerError
}
