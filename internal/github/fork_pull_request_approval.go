package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// ForkPullRequestApproval は Fork Pull Request ワークフローの承認のうち、reco が観測する設定である。
type ForkPullRequestApproval struct {
	ApprovalPolicy SettingObservation[setting.ForkPullRequestApprovalPolicy]
}

// observeForkPullRequestApproval は、公開範囲が取得条件に当たれば要求を送らずに観測不能とし、当たらなければ承認ポリシーを読み取る。
// 公開と観測したあとの失敗は、二つの要求のあいだに公開範囲が変わった可能性があっても、422 を含めて観測不能にせず error として返す。
func (c *Client) observeForkPullRequestApproval(ctx context.Context, reference repository.Reference, private bool) (ForkPullRequestApproval, error) {
	if reason, unavailable := forkPullRequestApprovalUnavailability(private); unavailable {
		return ForkPullRequestApproval{ApprovalPolicy: Unavailable[setting.ForkPullRequestApprovalPolicy](reason)}, nil
	}
	_, body, err := c.send(ctx, http.MethodGet, c.forkPullRequestApprovalURL(reference), nil, http.StatusOK)
	if err != nil {
		return ForkPullRequestApproval{}, err
	}
	policy, err := decodeForkPullRequestApproval(body)
	if err != nil {
		return ForkPullRequestApproval{}, err
	}
	return ForkPullRequestApproval{ApprovalPolicy: policy}, nil
}

// forkPullRequestApprovalUnavailability は、観測した公開範囲から、承認ポリシーが取得条件により観測不能になる理由を返す。
// 読み取り要求を送るかもこの戻り値で決め、表す理由と送る条件を食い違わせない。
func forkPullRequestApprovalUnavailability(private bool) (reason UnavailableReason, unavailable bool) {
	if private {
		return UnavailablePrivateRepository, true
	}
	return 0, false
}

// forkPullRequestApprovalPayload は欠落と null を検出するため、フィールドをポインタで受ける。
type forkPullRequestApprovalPayload struct {
	ApprovalPolicy *string `json:"approval_policy"`
}

// decodeForkPullRequestApproval は承認ポリシーの応答本文を観測結果に変換する。
// approval_policy は GitHub が値を足し得る列挙として読み、知らない値なら観測不能にする。
func decodeForkPullRequestApproval(data []byte) (SettingObservation[setting.ForkPullRequestApprovalPolicy], error) {
	var p forkPullRequestApprovalPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return SettingObservation[setting.ForkPullRequestApprovalPolicy]{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	if p.ApprovalPolicy == nil {
		return SettingObservation[setting.ForkPullRequestApprovalPolicy]{}, fmt.Errorf("%w: missing fields: approval_policy", ErrUnexpectedResponse)
	}
	policy, ok := setting.ParseForkPullRequestApprovalPolicy(*p.ApprovalPolicy)
	if !ok {
		return Unavailable[setting.ForkPullRequestApprovalPolicy](UnavailableUnknownValue), nil
	}
	return Observed(policy), nil
}
