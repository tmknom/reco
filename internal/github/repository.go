package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// Repository は GitHub から取得したリポジトリの実体と一般設定である。
type Repository struct {
	ID      int64
	General General
}

// General はリポジトリ一般設定のうち、reco が観測する値である。
type General struct {
	AllowAutoMerge      bool
	DeleteBranchOnMerge bool
}

// FetchRepository は owner/repo のリポジトリを取得する。
// owner と repo は呼び出し元が形式を検証済みであることを前提とする。パスのエスケープでは . と .. を防げないため。
func (c *Client) FetchRepository(ctx context.Context, owner, repo string) (Repository, error) {
	r, err := c.fetchRepository(ctx, owner, repo)
	if err != nil {
		return Repository{}, fmt.Errorf("fetch repository %s/%s: %w", owner, repo, err)
	}
	return r, nil
}

func (c *Client) fetchRepository(ctx context.Context, owner, repo string) (Repository, error) {
	body, err := c.get(ctx, c.repositoryURL(owner, repo))
	if err != nil {
		return Repository{}, err
	}
	return decodeRepository(body)
}

// repositoryURL は送信先のオリジンに /repos/{owner}/{repo} を置く。
// url.URL.JoinPath は path が空のオリジンに相対パスを返すため使わない。
func (c *Client) repositoryURL(owner, repo string) *url.URL {
	u := *c.baseURL
	u.Path = "/repos/" + owner + "/" + repo
	u.RawPath = "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	return &u
}

// repositoryPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type repositoryPayload struct {
	ID                  *int64 `json:"id"`
	AllowAutoMerge      *bool  `json:"allow_auto_merge"`
	DeleteBranchOnMerge *bool  `json:"delete_branch_on_merge"`
}

// decodeRepository は応答本文をリポジトリに変換する。
// 欠落の理由を応答から確認できないため、欠けた値を既定値で補わず、観測操作の失敗として返す。
func decodeRepository(data []byte) (Repository, error) {
	var p repositoryPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return Repository{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
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
		return Repository{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	return Repository{
		ID: *p.ID,
		General: General{
			AllowAutoMerge:      *p.AllowAutoMerge,
			DeleteBranchOnMerge: *p.DeleteBranchOnMerge,
		},
	}, nil
}
