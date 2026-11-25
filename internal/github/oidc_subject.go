package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// OIDCSubject は OIDC トークンの subject クレームのうち、reco が観測する設定である。
// subject のカスタマイズは取得条件を持たないため、SettingObservation にしない。
type OIDCSubject struct {
	Customization setting.OIDCSubjectCustomization
}

// observeOIDCSubject は subject のカスタマイズを、公開範囲によらず読み取る。
func (c *Client) observeOIDCSubject(ctx context.Context, reference repository.Reference) (OIDCSubject, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.oidcSubjectURL(reference), nil, http.StatusOK)
	if err != nil {
		return OIDCSubject{}, err
	}
	customization, err := decodeOIDCSubject(body)
	if err != nil {
		return OIDCSubject{}, err
	}
	return OIDCSubject{Customization: customization}, nil
}

// oidcSubjectPayload は二つの真偽値の欠落と null を検出するため、それらをポインタで受ける。
// key の要素もポインタで受けるのは、string で受けると null の要素が空文字列としてデコードされるためである。
// key の並びの欠落と null は、どちらも nil として同じに扱う。
type oidcSubjectPayload struct {
	UseDefault          *bool     `json:"use_default"`
	IncludeClaimKeys    []*string `json:"include_claim_keys"`
	UseImmutableSubject *bool     `json:"use_immutable_subject"`
}

// decodeOIDCSubject は subject のカスタマイズの応答本文を観測値に変換する。
// key の要素は、観測値に含めない use_default が真の応答でも確かめ、未知の形を観測値にしない。
func decodeOIDCSubject(data []byte) (setting.OIDCSubjectCustomization, error) {
	var p oidcSubjectPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return setting.OIDCSubjectCustomization{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.UseDefault == nil {
		missing = append(missing, "use_default")
	}
	if p.UseImmutableSubject == nil {
		missing = append(missing, "use_immutable_subject")
	}
	if len(missing) > 0 {
		return setting.OIDCSubjectCustomization{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	if slices.Contains(p.IncludeClaimKeys, nil) {
		return setting.OIDCSubjectCustomization{}, fmt.Errorf("%w: include_claim_keys has a null element", ErrUnexpectedResponse)
	}
	customization := setting.OIDCSubjectCustomization{UseDefault: *p.UseDefault, UseImmutableSubject: *p.UseImmutableSubject}
	if !customization.UseDefault {
		for _, key := range p.IncludeClaimKeys {
			customization.IncludeClaimKeys = append(customization.IncludeClaimKeys, *key)
		}
	}
	return customization, nil
}
