package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// UpdateOutcome は、一つの更新操作について、GitHub の応答から判断した結果である。
// ゼロ値を UpdateUnknown にするのは、結果を設定し損ねた値を成功や拒否と取り違えないため。
type UpdateOutcome int

const (
	UpdateUnknown UpdateOutcome = iota
	UpdateSucceeded
	UpdateRejected
)

// Update は、reference のリポジトリの key の設定を value にする更新要求を、UpdateEndpointOf(key) へ一度だけ送る。再送はしない。
// error が nil になるのは UpdateSucceeded のときだけで、呼び出し側は error から結果を導かない。
func (c *Client) Update(ctx context.Context, reference repository.Reference, key setting.Key, value setting.Value) (UpdateOutcome, error) {
	return updateRequestOf(key).send(c, ctx, reference, value)
}

// updateRequestFunc は、reference のリポジトリの設定を value にする更新要求を一度送る。
// メソッド式 ((*Client).updateAllowAutoMerge など) をそのまま渡せるよう、受け手の Client を先頭に置く。
type updateRequestFunc func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error)

// updateRequest は、一つの設定を変える更新要求の、送り先の更新エンドポイントと送り方の組。
// 送る更新要求と比べる設定の範囲を別々の場所で足し忘れないよう、同じ switch から返す。
type updateRequest struct {
	endpoint UpdateEndpoint
	send     updateRequestFunc
}

// updateRequestOf は、key の設定を変える更新要求を返す。
// 未定義の設定は、不変条件の破れとして panic する。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func updateRequestOf(key setting.Key) updateRequest {
	switch key {
	case setting.DeleteBranchOnMerge:
		return updateRequest{RepositoryEndpoint, boolUpdate((*Client).updateDeleteBranchOnMerge)}
	case setting.AllowAutoMerge:
		return updateRequest{RepositoryEndpoint, boolUpdate((*Client).updateAllowAutoMerge)}
	case setting.ActionsEnabled:
		return updateRequest{ActionsPermissionsEndpoint, boolUpdate((*Client).updateActionsEnabled)}
	case setting.AllowedActions:
		return updateRequest{ActionsPermissionsEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateAllowedActions(ctx, reference, value.AllowedActionsPolicy())
		}}
	case setting.SelectedActions:
		// Actions 許可ポリシーが selected のまま3項目だけを変えるときも、ポリシーの更新要求は送らない。
		// selected を送り直すと、GitHub が github_owned_allowed を true に変えるため。
		return updateRequest{SelectedActionsEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateSelectedActions(ctx, reference, value.SelectedActionsCriteria())
		}}
	case setting.DefaultWorkflowPermissions:
		return updateRequest{WorkflowPermissionsEndpoint, func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
			return c.updateDefaultWorkflowPermissions(ctx, reference, value.PermissionLevel())
		}}
	case setting.CanApprovePullRequestReviews:
		return updateRequest{WorkflowPermissionsEndpoint, boolUpdate((*Client).updateCanApprovePullRequestReviews)}
	case setting.SHAPinningRequired:
		return updateRequest{ActionsPermissionsEndpoint, boolUpdate((*Client).updateSHAPinningRequired)}
	}
	panic("github: no update request for " + string(key))
}

// boolUpdate は、真偽値を受ける更新メソッドを、設定の値を受ける更新要求にする。
func boolUpdate(update func(*Client, context.Context, repository.Reference, bool) (UpdateOutcome, error)) updateRequestFunc {
	return func(c *Client, ctx context.Context, reference repository.Reference, value setting.Value) (UpdateOutcome, error) {
		return update(c, ctx, reference, value.Bool())
	}
}

// updateAllowAutoMerge は reference のリポジトリの allow_auto_merge を allow に更新する。
func (c *Client) updateAllowAutoMerge(ctx context.Context, reference repository.Reference, allow bool) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, fmt.Appendf(nil, `{"allow_auto_merge":%t}`, allow))
}

// updateDeleteBranchOnMerge は reference のリポジトリの delete_branch_on_merge を enabled に更新する。
func (c *Client) updateDeleteBranchOnMerge(ctx context.Context, reference repository.Reference, enabled bool) (UpdateOutcome, error) {
	return c.updateRepository(ctx, reference, fmt.Appendf(nil, `{"delete_branch_on_merge":%t}`, enabled))
}

// updateRepository は body を本文とする更新要求を送る。
// 成功応答の本文は、更新の契約に適合するかを確かめるためだけに解釈する。
func (c *Client) updateRepository(ctx context.Context, reference repository.Reference, body []byte) (UpdateOutcome, error) {
	status, data, err := c.send(ctx, http.MethodPatch, c.repositoryURL(reference), body, http.StatusOK)
	if err == nil {
		_, err = decodeRepository(data)
	}
	return describeUpdateOutcome("update repository", reference, status, err)
}

// updateDefaultWorkflowPermissions は reference のリポジトリの default_workflow_permissions を level に更新する。
func (c *Client) updateDefaultWorkflowPermissions(ctx context.Context, reference repository.Reference, level setting.PermissionLevel) (UpdateOutcome, error) {
	return c.updateWorkflowPermissions(ctx, reference, fmt.Appendf(nil, `{"default_workflow_permissions":%q}`, level))
}

// updateCanApprovePullRequestReviews は reference のリポジトリの can_approve_pull_request_reviews を canApprove に更新する。
func (c *Client) updateCanApprovePullRequestReviews(ctx context.Context, reference repository.Reference, canApprove bool) (UpdateOutcome, error) {
	return c.updateWorkflowPermissions(ctx, reference, fmt.Appendf(nil, `{"can_approve_pull_request_reviews":%t}`, canApprove))
}

// updateWorkflowPermissions は body を本文とする更新要求を送る。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updateWorkflowPermissions(ctx context.Context, reference repository.Reference, body []byte) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, http.MethodPut, c.workflowPermissionsURL(reference), body, http.StatusNoContent)
	return describeUpdateOutcome("update workflow permissions", reference, status, err)
}

// updateActionsEnabled は reference のリポジトリの Actions の有効状態を enabled に更新する。
// allowed_actions と sha_pinning_required は送らず、GitHub が省略した値を保持することに依拠する（アーキテクチャ設計の未管理設定を保全する更新方式）。
func (c *Client) updateActionsEnabled(ctx context.Context, reference repository.Reference, enabled bool) (UpdateOutcome, error) {
	return c.updateActionsPermissions(ctx, reference, fmt.Appendf(nil, `{"enabled":%t}`, enabled))
}

// updateAllowedActions は reference のリポジトリの Actions 許可ポリシーを policy に更新する。
// Actions が無効のリポジトリでは、同じ要求が Actions を有効にする。
func (c *Client) updateAllowedActions(ctx context.Context, reference repository.Reference, policy setting.AllowedActionsPolicy) (UpdateOutcome, error) {
	// policy は英小文字と _ だけの定義済みの値で、%q の表記が JSON の文字列と一致する
	return c.updateActionsPermissions(ctx, reference, fmt.Appendf(nil, `{"enabled":true,"allowed_actions":%q}`, policy))
}

// updateSHAPinningRequired は reference のリポジトリの sha_pinning_required を required に更新する。
// 必須の enabled には true を送り、allowed_actions は送らない（アーキテクチャ設計の未管理設定を保全する更新方式）。
// 無効のリポジトリへ送ると Actions を有効にするため、呼び出し側が更新直前に有効と確かめる。
func (c *Client) updateSHAPinningRequired(ctx context.Context, reference repository.Reference, required bool) (UpdateOutcome, error) {
	return c.updateActionsPermissions(ctx, reference, fmt.Appendf(nil, `{"enabled":true,"sha_pinning_required":%t}`, required))
}

// updateActionsPermissions は body を本文とする更新要求を送る。
// 正常応答の 204 は本文を持たず、成功の判定に本文を使わない。
func (c *Client) updateActionsPermissions(ctx context.Context, reference repository.Reference, body []byte) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, http.MethodPut, c.actionsPermissionsURL(reference), body, http.StatusNoContent)
	return describeUpdateOutcome("update actions permissions", reference, status, err)
}

// updateSelectedActions は reference のリポジトリの選択方式の Actions 許可条件を criteria に更新する。
// GitHub は、要求の前提となる設定の状態が成り立たないとき 409 で拒否する（GitHub Actions 権限 API 仕様の Selected actions）。
func (c *Client) updateSelectedActions(ctx context.Context, reference repository.Reference, criteria setting.SelectedActionsCriteria) (UpdateOutcome, error) {
	status, _, err := c.send(ctx, http.MethodPut, c.selectedActionsURL(reference), encodeSelectedActions(criteria), http.StatusNoContent)
	return describeUpdateOutcome("update selected actions", reference, status, err)
}

// encodeSelectedActions は criteria を Selected actions の更新要求の本文にする。
// 許可パターンは任意の文字列のため、Go の %q ではなく JSON の規則でエスケープする。
func encodeSelectedActions(criteria setting.SelectedActionsCriteria) []byte {
	patterns := criteria.PatternsAllowed
	// encoding/json は nil のスライスを null にするため、空の集合を [] として送るよう長さ 0 のスライスに替える
	if patterns == nil {
		patterns = []string{}
	}
	return mustMarshalJSON(struct {
		GitHubOwnedAllowed bool     `json:"github_owned_allowed"`
		VerifiedAllowed    bool     `json:"verified_allowed"`
		PatternsAllowed    []string `json:"patterns_allowed"`
	}{criteria.GitHubOwnedAllowed, criteria.VerifiedAllowed, patterns})
}

// mustMarshalJSON は v を JSON にする。
// 本文の型は bool と []string だけを持ち、json.Marshal がどの値でも失敗しないため、失敗を不変条件の破れとして panic する。
// encoding/json に Must の関数がないため、この包みを置く。
func mustMarshalJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("github: marshal request body: %v", err))
	}
	return data
}

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
