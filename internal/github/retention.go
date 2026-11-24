package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/tmknom/reco/internal/repository"
)

// Retention は保持日数の観測値と、同じ取得で GitHub が返す、保持日数の上限である。
// 上限は設定の値ではないため、SettingObservation にせず観測値と並べて持つ。保持日数は取得条件を持たない。
type Retention struct {
	Days               int
	MaximumAllowedDays int
}

// observeRetention は保持日数とその上限を、公開範囲によらず読み取る。
func (c *Client) observeRetention(ctx context.Context, reference repository.Reference) (Retention, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.retentionURL(reference), nil, http.StatusOK)
	if err != nil {
		return Retention{}, err
	}
	return decodeRetention(body)
}

// retentionPayload は欠落と null を検出するため、フィールドをポインタで受ける。
type retentionPayload struct {
	Days               *int `json:"days"`
	MaximumAllowedDays *int `json:"maximum_allowed_days"`
}

// decodeRetention は保持期間の応答本文を、保持日数の観測値とその上限に変換する。
// 上限を超える日数も、上限があとから下がった状態としてあり得るため、範囲を確かめずにそのまま持つ。
func decodeRetention(data []byte) (Retention, error) {
	var p retentionPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return Retention{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.Days == nil {
		missing = append(missing, "days")
	}
	if p.MaximumAllowedDays == nil {
		missing = append(missing, "maximum_allowed_days")
	}
	if len(missing) > 0 {
		return Retention{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	return Retention{Days: *p.Days, MaximumAllowedDays: *p.MaximumAllowedDays}, nil
}
