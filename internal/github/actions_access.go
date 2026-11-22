package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// ActionsAccess はリポジトリ外のワークフローからのアクセスのうち、reco が観測する設定である。
type ActionsAccess struct {
	AccessLevel SettingObservation[setting.AccessLevel]
}

// observeActionsAccess は、公開範囲が取得条件に当たれば要求を送らずに観測不能とし、当たらなければ access_level を読み取る。
// 非公開と観測したあとの失敗は、二つの要求のあいだに公開範囲が変わった可能性があっても、422 を含めて観測不能にせず error として返す。
func (c *Client) observeActionsAccess(ctx context.Context, reference repository.Reference, private bool) (ActionsAccess, error) {
	if reason, unavailable := accessLevelUnavailability(private); unavailable {
		return ActionsAccess{AccessLevel: Unavailable[setting.AccessLevel](reason)}, nil
	}
	_, body, err := c.send(ctx, http.MethodGet, c.actionsAccessURL(reference), nil, http.StatusOK)
	if err != nil {
		return ActionsAccess{}, err
	}
	level, err := decodeActionsAccess(body)
	if err != nil {
		return ActionsAccess{}, err
	}
	return ActionsAccess{AccessLevel: Observed(level)}, nil
}

// accessLevelUnavailability は、観測した公開範囲から、access_level が取得条件により観測不能になる理由を返す。
// 読み取り要求を送るかもこの戻り値で決め、表す理由と送る条件を食い違わせない。
func accessLevelUnavailability(private bool) (reason UnavailableReason, unavailable bool) {
	if !private {
		return UnavailablePublicRepository, true
	}
	return 0, false
}

// actionsAccessPayload は欠落と null を検出するため、フィールドをポインタで受ける。
type actionsAccessPayload struct {
	AccessLevel *string `json:"access_level"`
}

// decodeActionsAccess はリポジトリ外のワークフローからのアクセスの応答本文を access_level の観測値に変換する。
// 定義外の値は応答本文の一部のため、文言に入れない。
func decodeActionsAccess(data []byte) (setting.AccessLevel, error) {
	var p actionsAccessPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	if p.AccessLevel == nil {
		return "", fmt.Errorf("%w: missing fields: access_level", ErrUnexpectedResponse)
	}
	level, ok := setting.ParseAccessLevel(*p.AccessLevel)
	if !ok {
		return "", fmt.Errorf("%w: unsupported value: access_level", ErrUnexpectedResponse)
	}
	return level, nil
}
