package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// PrivateForkPullRequestWorkflows は非公開リポジトリの Fork Pull Request ワークフローのうち、reco が観測する設定である。
type PrivateForkPullRequestWorkflows struct {
	Conditions SettingObservation[setting.PrivateForkPullRequestWorkflowConditions]
}

// observePrivateForkPullRequestWorkflows は、公開範囲が取得条件に当たれば要求を送らずに観測不能とし、当たらなければ実行条件を読み取る。
// 非公開と観測したあとの失敗は、二つの要求のあいだに公開範囲が変わった可能性があっても、422 を含めて観測不能にせず error として返す。
func (c *Client) observePrivateForkPullRequestWorkflows(ctx context.Context, reference repository.Reference, private bool) (PrivateForkPullRequestWorkflows, error) {
	if reason, unavailable := privateForkPullRequestWorkflowsUnavailability(private); unavailable {
		return PrivateForkPullRequestWorkflows{Conditions: Unavailable[setting.PrivateForkPullRequestWorkflowConditions](reason)}, nil
	}
	_, body, err := c.send(ctx, http.MethodGet, c.privateForkPullRequestWorkflowsURL(reference), nil, http.StatusOK)
	if err != nil {
		return PrivateForkPullRequestWorkflows{}, err
	}
	conditions, err := decodePrivateForkPullRequestWorkflows(body)
	if err != nil {
		return PrivateForkPullRequestWorkflows{}, err
	}
	return PrivateForkPullRequestWorkflows{Conditions: Observed(conditions)}, nil
}

// privateForkPullRequestWorkflowsUnavailability は、観測した公開範囲から、実行条件が取得条件により観測不能になる理由を返す。
// 読み取り要求を送るかもこの戻り値で決め、表す理由と送る条件を食い違わせない。
func privateForkPullRequestWorkflowsUnavailability(private bool) (reason UnavailableReason, unavailable bool) {
	if !private {
		return UnavailablePublicRepository, true
	}
	return 0, false
}

// privateForkPullRequestWorkflowsPayload は欠落と null を検出するため、フィールドをポインタで受ける。
type privateForkPullRequestWorkflowsPayload struct {
	RunWorkflowsFromForkPullRequests           *bool `json:"run_workflows_from_fork_pull_requests"`
	SendWriteTokensToWorkflows                 *bool `json:"send_write_tokens_to_workflows"`
	SendSecretsAndVariables                    *bool `json:"send_secrets_and_variables"`
	RequireApprovalForForkPullRequestWorkflows *bool `json:"require_approval_for_fork_pr_workflows"`
}

// decodePrivateForkPullRequestWorkflows は実行条件の応答本文を観測値に変換する。
// 4項目は一つでも欠ければ、既定値で補わず契約外の応答とする。
// run_workflows_from_fork_pull_requests が false のときにほかの3項目が false で返る関係は、取得契約にせず、返った値をそのまま持つ。
func decodePrivateForkPullRequestWorkflows(data []byte) (setting.PrivateForkPullRequestWorkflowConditions, error) {
	var p privateForkPullRequestWorkflowsPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return setting.PrivateForkPullRequestWorkflowConditions{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.RunWorkflowsFromForkPullRequests == nil {
		missing = append(missing, "run_workflows_from_fork_pull_requests")
	}
	if p.SendWriteTokensToWorkflows == nil {
		missing = append(missing, "send_write_tokens_to_workflows")
	}
	if p.SendSecretsAndVariables == nil {
		missing = append(missing, "send_secrets_and_variables")
	}
	if p.RequireApprovalForForkPullRequestWorkflows == nil {
		missing = append(missing, "require_approval_for_fork_pr_workflows")
	}
	if len(missing) > 0 {
		return setting.PrivateForkPullRequestWorkflowConditions{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	return setting.PrivateForkPullRequestWorkflowConditions{
		RunWorkflowsFromForkPullRequests:           *p.RunWorkflowsFromForkPullRequests,
		SendWriteTokensToWorkflows:                 *p.SendWriteTokensToWorkflows,
		SendSecretsAndVariables:                    *p.SendSecretsAndVariables,
		RequireApprovalForForkPullRequestWorkflows: *p.RequireApprovalForForkPullRequestWorkflows,
	}, nil
}
