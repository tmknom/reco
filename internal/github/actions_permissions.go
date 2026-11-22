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

// ActionsPermissions は Actions の実行許可のうち、reco が観測する設定である。
// Actions の有効状態は常に観測値を持ち、ほかの設定は取得条件により観測不能になり得る。
type ActionsPermissions struct {
	Enabled            bool
	AllowedActions     SettingObservation[setting.AllowedActionsPolicy]
	SHAPinningRequired SettingObservation[bool]
	SelectedActions    SettingObservation[setting.SelectedActionsCriteria]
}

// observeActionsPermissions は Actions permissions を読み取る。選択方式の Actions 許可条件は別の要求のため決めない。
func (c *Client) observeActionsPermissions(ctx context.Context, reference repository.Reference) (ActionsPermissions, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.actionsPermissionsURL(reference), nil, http.StatusOK)
	if err != nil {
		return ActionsPermissions{}, err
	}
	return decodeActionsPermissions(body)
}

// observeSelectedActions は、観測した Actions 許可ポリシーが取得条件に当たれば要求を送らずに観測不能とし、当たらなければ Selected actions を読み取る。
// selected と観測したあとの失敗は、二つの要求のあいだにポリシーが変わった可能性があっても観測不能にせず、error として返す。
func (c *Client) observeSelectedActions(ctx context.Context, reference repository.Reference, allowed SettingObservation[setting.AllowedActionsPolicy]) (SettingObservation[setting.SelectedActionsCriteria], error) {
	if reason, unavailable := selectedActionsUnavailability(allowed); unavailable {
		return Unavailable[setting.SelectedActionsCriteria](reason), nil
	}
	_, body, err := c.send(ctx, http.MethodGet, c.selectedActionsURL(reference), nil, http.StatusOK)
	if err != nil {
		return SettingObservation[setting.SelectedActionsCriteria]{}, err
	}
	criteria, err := decodeSelectedActions(body)
	if err != nil {
		return SettingObservation[setting.SelectedActionsCriteria]{}, err
	}
	return Observed(criteria), nil
}

// actionsPermissionsPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type actionsPermissionsPayload struct {
	Enabled            *bool   `json:"enabled"`
	AllowedActions     *string `json:"allowed_actions"`
	SHAPinningRequired *bool   `json:"sha_pinning_required"`
}

// decodeActionsPermissions は Actions permissions の応答本文を、Actions の有効状態、Actions 許可ポリシー、sha_pinning_required の観測結果に変換する。
// Actions が無効のときは、応答に値があっても設定の状態を示さないため読まない。有効のときの欠落は既定値で補わない。
// 定義外の値は応答本文の一部のため、文言に入れない。
func decodeActionsPermissions(data []byte) (ActionsPermissions, error) {
	var p actionsPermissionsPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return ActionsPermissions{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	if p.Enabled == nil {
		return ActionsPermissions{}, fmt.Errorf("%w: missing fields: enabled", ErrUnexpectedResponse)
	}
	if !*p.Enabled {
		return ActionsPermissions{
			AllowedActions:     Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled),
			SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled),
		}, nil
	}
	var missing []string
	if p.AllowedActions == nil {
		missing = append(missing, "allowed_actions")
	}
	if p.SHAPinningRequired == nil {
		missing = append(missing, "sha_pinning_required")
	}
	if len(missing) > 0 {
		return ActionsPermissions{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	policy, ok := setting.ParseAllowedActionsPolicy(*p.AllowedActions)
	if !ok {
		return ActionsPermissions{}, fmt.Errorf("%w: unsupported value: allowed_actions", ErrUnexpectedResponse)
	}
	return ActionsPermissions{
		Enabled:            true,
		AllowedActions:     Observed(policy),
		SHAPinningRequired: Observed(*p.SHAPinningRequired),
	}, nil
}

// selectedActionsUnavailability は、観測した Actions 許可ポリシーから、Selected actions が取得条件により観測不能になる理由を返す。
// 読み取り要求を送るかもこの戻り値で決め、表す理由と送る条件を食い違わせない。
func selectedActionsUnavailability(allowed SettingObservation[setting.AllowedActionsPolicy]) (reason UnavailableReason, unavailable bool) {
	policy, ok := allowed.Value()
	if !ok {
		return allowed.UnavailableReason(), true
	}
	if policy != setting.AllowedActionsSelected {
		return UnavailableNotSelected, true
	}
	return 0, false
}

// selectedActionsPayload は欠落と null を検出するため、各フィールドと許可パターンの要素をポインタで受ける。
// encoding/json は null の要素を string へ読むと空文字列のまま残し、欠けた値を空文字列と区別できなくなるため。
type selectedActionsPayload struct {
	GitHubOwnedAllowed *bool      `json:"github_owned_allowed"`
	VerifiedAllowed    *bool      `json:"verified_allowed"`
	PatternsAllowed    *[]*string `json:"patterns_allowed"`
}

// decodeSelectedActions は Selected actions の応答本文を選択方式の Actions 許可条件に変換する。
// 3項目は一つでも欠ければ、既定値で補わず契約外の応答とする。許可パターンは応答本文の一部のため、文言に入れない。
func decodeSelectedActions(data []byte) (setting.SelectedActionsCriteria, error) {
	var p selectedActionsPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return setting.SelectedActionsCriteria{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.GitHubOwnedAllowed == nil {
		missing = append(missing, "github_owned_allowed")
	}
	if p.VerifiedAllowed == nil {
		missing = append(missing, "verified_allowed")
	}
	if p.PatternsAllowed == nil {
		missing = append(missing, "patterns_allowed")
	}
	if len(missing) > 0 {
		return setting.SelectedActionsCriteria{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	patterns := make([]string, len(*p.PatternsAllowed))
	for i, pattern := range *p.PatternsAllowed {
		if pattern == nil {
			return setting.SelectedActionsCriteria{}, fmt.Errorf("%w: unsupported value: patterns_allowed", ErrUnexpectedResponse)
		}
		patterns[i] = *pattern
	}
	return setting.SelectedActionsCriteria{
		GitHubOwnedAllowed: *p.GitHubOwnedAllowed,
		VerifiedAllowed:    *p.VerifiedAllowed,
		PatternsAllowed:    patterns,
	}, nil
}
