package github

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/repository"
)

// repositoryPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type repositoryPayload struct {
	ID                  *int64 `json:"id"`
	AllowAutoMerge      *bool  `json:"allow_auto_merge"`
	DeleteBranchOnMerge *bool  `json:"delete_branch_on_merge"`
	Private             *bool  `json:"private"`
	// Permissions は Dependabot の領域を読まない観測では使わないため、取得契約に含めず、欠落と null を許す。
	Permissions *repositoryPermissionsPayload `json:"permissions"`
	// SecurityAndAnalysis は Secret scanning の領域を読む観測だけが解釈する。ほかの観測が、宣言していない設定の応答の形で失敗しないよう、型を合わせず未解釈のまま受ける。
	SecurityAndAnalysis json.RawMessage `json:"security_and_analysis"`
}

type repositoryPermissionsPayload struct {
	Admin *bool `json:"admin"`
}

// repositoryAttributes は、リポジトリの応答のうち、decodeRepository が観測結果に変換しない値である。
type repositoryAttributes struct {
	private             bool
	admin               bool
	securityAndAnalysis json.RawMessage
}

// decodeRepository はリポジトリの応答本文を、観測結果と、観測結果に変換しない値に変換する。
// 欠落の理由を応答から確認できないため、欠けた値を既定値で補わず、契約外の応答として返す。
func decodeRepository(data []byte) (ObservationResult, repositoryAttributes, error) {
	var p repositoryPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return ObservationResult{}, repositoryAttributes{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.ID == nil {
		missing = append(missing, "id")
	}
	if p.AllowAutoMerge == nil {
		missing = append(missing, "allow_auto_merge")
	}
	if p.DeleteBranchOnMerge == nil {
		missing = append(missing, "delete_branch_on_merge")
	}
	if p.Private == nil {
		missing = append(missing, "private")
	}
	if len(missing) > 0 {
		return ObservationResult{}, repositoryAttributes{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	return ObservationResult{
		RepositoryID: repository.ID(*p.ID),
		General: General{
			AllowAutoMerge:      *p.AllowAutoMerge,
			DeleteBranchOnMerge: *p.DeleteBranchOnMerge,
		},
	}, repositoryAttributes{
		private:             *p.Private,
		admin:               p.Permissions != nil && p.Permissions.Admin != nil && *p.Permissions.Admin,
		securityAndAnalysis: p.SecurityAndAnalysis,
	}, nil
}
