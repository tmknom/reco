package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/tmknom/reco/internal/repository"
)

// ObservationResult は、一回の観測で確認できた、リポジトリ指定が指す実体の ID と設定の観測値である。
type ObservationResult struct {
	RepositoryID repository.ID
	General      General
}

// General はリポジトリ一般設定のうち、reco が観測する値である。管理範囲を知らないため、未管理設定の値も含む。
type General struct {
	AllowAutoMerge      bool
	DeleteBranchOnMerge bool
}

// ObserveRepository は reference のリポジトリを観測する。
func (c *Client) ObserveRepository(ctx context.Context, reference repository.Reference) (ObservationResult, error) {
	o, err := c.observeRepository(ctx, reference)
	if err != nil {
		return ObservationResult{}, fmt.Errorf("fetch repository %s: %w", reference, err)
	}
	return o, nil
}

func (c *Client) observeRepository(ctx context.Context, reference repository.Reference) (ObservationResult, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.repositoryURL(reference), nil)
	if err != nil {
		return ObservationResult{}, err
	}
	return decodeRepository(body)
}

// repositoryURL は送信先のオリジンに /repos/{owner}/{repo} を置く。
// url.URL.JoinPath は path が空のオリジンに相対パスを返すため使わない。
func (c *Client) repositoryURL(reference repository.Reference) *url.URL {
	u := *c.baseURL
	u.Path = "/repos/" + reference.Owner() + "/" + reference.Repo()
	return &u
}

// repositoryPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type repositoryPayload struct {
	ID                  *int64 `json:"id"`
	AllowAutoMerge      *bool  `json:"allow_auto_merge"`
	DeleteBranchOnMerge *bool  `json:"delete_branch_on_merge"`
}

// decodeRepository はリポジトリの応答本文を観測結果に変換する。
// 欠落の理由を応答から確認できないため、欠けた値を既定値で補わず、契約外の応答として返す。
func decodeRepository(data []byte) (ObservationResult, error) {
	var p repositoryPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return ObservationResult{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
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
	if len(missing) > 0 {
		return ObservationResult{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	return ObservationResult{
		RepositoryID: repository.ID(*p.ID),
		General: General{
			AllowAutoMerge:      *p.AllowAutoMerge,
			DeleteBranchOnMerge: *p.DeleteBranchOnMerge,
		},
	}, nil
}
