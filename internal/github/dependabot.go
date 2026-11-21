package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// Dependabot は Dependabot の領域のうち、reco が観測する設定である。どちらも取得条件を持たない。
type Dependabot struct {
	Alerts          bool
	SecurityUpdates setting.DependabotSecurityUpdates
}

// observeDependabotAlerts は、ステータスで値を返す Dependabot alerts の取得を送る。
// リダイレクトやレート制限の分類をほかの取得と揃えるため send を通す。
func (c *Client) observeDependabotAlerts(ctx context.Context, reference repository.Reference, admin bool) (bool, error) {
	_, _, err := c.send(ctx, http.MethodGet, c.vulnerabilityAlertsURL(reference), nil, http.StatusNoContent)
	return dependabotAlertsEnabled(err, admin)
}

// observeDependabotSecurityUpdates は Dependabot security updates を読み取る。無効も 200 で返るため、404 は読み替えない。
func (c *Client) observeDependabotSecurityUpdates(ctx context.Context, reference repository.Reference) (setting.DependabotSecurityUpdates, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.automatedSecurityFixesURL(reference), nil, http.StatusOK)
	if err != nil {
		return 0, err
	}
	return decodeAutomatedSecurityFixes(body)
}

// dependabotAlertsEnabled は、Dependabot alerts の取得の結果を有効状態に読む。
// 404 は無効のときにも権限の不足やリポジトリの不存在のときにも返るため、admin のアクセスを確かめたときに限り無効と読む。
func dependabotAlertsEnabled(err error, admin bool) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrNotFound) && admin {
		return false, nil
	}
	return false, err
}

// automatedSecurityFixesPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type automatedSecurityFixesPayload struct {
	Enabled *bool `json:"enabled"`
	Paused  *bool `json:"paused"`
}

// decodeAutomatedSecurityFixes は Dependabot security updates の応答本文を観測値に変換する。
// 無効で一時停止中という組は観測しておらず意味を根拠づけられないため、契約外の応答とする。
func decodeAutomatedSecurityFixes(data []byte) (setting.DependabotSecurityUpdates, error) {
	var p automatedSecurityFixesPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.Enabled == nil {
		missing = append(missing, "enabled")
	}
	if p.Paused == nil {
		missing = append(missing, "paused")
	}
	if len(missing) > 0 {
		return 0, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	switch {
	case *p.Enabled && *p.Paused:
		return setting.DependabotSecurityUpdatesPaused, nil
	case *p.Enabled:
		return setting.DependabotSecurityUpdatesEnabled, nil
	case *p.Paused:
		return 0, fmt.Errorf("%w: unsupported combination: enabled, paused", ErrUnexpectedResponse)
	default:
		return setting.DependabotSecurityUpdatesDisabled, nil
	}
}
