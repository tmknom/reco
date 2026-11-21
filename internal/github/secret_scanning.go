package github

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SecretScanning は Secret scanning の領域のうち、reco が観測する設定である。どちらも非公開リポジトリでは観測不能になる。
// PushProtection はリポジトリの push protection で、利用者が自分のアカウントに設定する push protection ではない。
type SecretScanning struct {
	Enabled        SettingObservation[bool]
	PushProtection SettingObservation[bool]
}

// securityAndAnalysisPayload は欠落と null を検出するため、各項目をポインタで受ける。
type securityAndAnalysisPayload struct {
	SecretScanning               *securityStatusPayload `json:"secret_scanning"`
	SecretScanningPushProtection *securityStatusPayload `json:"secret_scanning_push_protection"`
}

type securityStatusPayload struct {
	Status *string `json:"status"`
}

// decodeSecretScanning は、リポジトリの応答の security_and_analysis を Secret scanning の観測結果に変換する。
// 定義外の値は応答本文の一部のため、文言に入れない。
func decodeSecretScanning(private bool, securityAndAnalysis json.RawMessage) (SecretScanning, error) {
	if private {
		return SecretScanning{
			Enabled:        Unavailable[bool](UnavailablePrivateRepository),
			PushProtection: Unavailable[bool](UnavailablePrivateRepository),
		}, nil
	}
	if securityAndAnalysis == nil {
		return SecretScanning{}, fmt.Errorf("%w: missing fields: security_and_analysis", ErrUnexpectedResponse)
	}
	var p *securityAndAnalysisPayload
	if err := json.Unmarshal(securityAndAnalysis, &p); err != nil {
		return SecretScanning{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	if p == nil {
		return SecretScanning{}, fmt.Errorf("%w: missing fields: security_and_analysis", ErrUnexpectedResponse)
	}
	const (
		secretScanningField = "security_and_analysis.secret_scanning.status"
		pushProtectionField = "security_and_analysis.secret_scanning_push_protection.status"
	)
	var missing []string
	if p.SecretScanning == nil || p.SecretScanning.Status == nil {
		missing = append(missing, secretScanningField)
	}
	if p.SecretScanningPushProtection == nil || p.SecretScanningPushProtection.Status == nil {
		missing = append(missing, pushProtectionField)
	}
	if len(missing) > 0 {
		return SecretScanning{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	enabled, ok := parseSecurityStatus(*p.SecretScanning.Status)
	if !ok {
		return SecretScanning{}, fmt.Errorf("%w: unsupported value: %s", ErrUnexpectedResponse, secretScanningField)
	}
	pushProtection, ok := parseSecurityStatus(*p.SecretScanningPushProtection.Status)
	if !ok {
		return SecretScanning{}, fmt.Errorf("%w: unsupported value: %s", ErrUnexpectedResponse, pushProtectionField)
	}
	return SecretScanning{Enabled: Observed(enabled), PushProtection: Observed(pushProtection)}, nil
}

// securityStatus は、有効状態を security_and_analysis の項目の status にする。parseSecurityStatus の逆である。
func securityStatus(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

// parseSecurityStatus は security_and_analysis の項目の status を有効状態に読む。GitHub の定義に合わせて大文字と小文字を区別する。
func parseSecurityStatus(status string) (enabled, ok bool) {
	switch status {
	case "enabled":
		return true, true
	case "disabled":
		return false, true
	}
	return false, false
}
