package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// ObservationResult は、一回の観測で確認できた、リポジトリ指定が指す実体の ID と設定の観測値である。
type ObservationResult struct {
	RepositoryID repository.ID
	General      General
	// WorkflowPermissions は、設定の領域に Workflow permissions を選ばなかった観測では nil になる。
	// 読み取っていない値を false や read として読ませないため、値型にしない。
	WorkflowPermissions *WorkflowPermissions
}

// WorkflowPermissions はワークフロー権限のうち、reco が観測する値である。
type WorkflowPermissions struct {
	DefaultWorkflowPermissions   setting.PermissionLevel
	CanApprovePullRequestReviews bool
}

// General はリポジトリ一般設定のうち、reco が観測する値である。管理範囲を知らないため、未管理設定の値も含む。
type General struct {
	AllowAutoMerge      bool
	DeleteBranchOnMerge bool
}

// ObserveRepository は reference のリポジトリを観測する。areas で、General に加えて読み取る設定の領域を選ぶ。
// General は常に読み取るため、areas に AreaGeneral を含めても結果は変わらない。
func (c *Client) ObserveRepository(ctx context.Context, reference repository.Reference, areas ...setting.Area) (ObservationResult, error) {
	o, err := c.observeRepository(ctx, reference)
	if err != nil {
		return ObservationResult{}, fmt.Errorf("fetch repository %s: %w", reference, err)
	}
	if slices.Contains(areas, setting.AreaWorkflowPermissions) {
		wp, err := c.observeWorkflowPermissions(ctx, reference)
		if err != nil {
			return ObservationResult{}, fmt.Errorf("fetch workflow permissions %s: %w", reference, err)
		}
		o.WorkflowPermissions = &wp
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

func (c *Client) observeWorkflowPermissions(ctx context.Context, reference repository.Reference) (WorkflowPermissions, error) {
	u := c.repositoryURL(reference)
	u.Path += "/actions/permissions/workflow"
	_, body, err := c.send(ctx, http.MethodGet, u, nil)
	if err != nil {
		return WorkflowPermissions{}, err
	}
	return decodeWorkflowPermissions(body)
}

// workflowPermissionsPayload は欠落と null を検出するため、各フィールドをポインタで受ける。
type workflowPermissionsPayload struct {
	DefaultWorkflowPermissions   *string `json:"default_workflow_permissions"`
	CanApprovePullRequestReviews *bool   `json:"can_approve_pull_request_reviews"`
}

// decodeWorkflowPermissions は Workflow permissions の応答本文を観測値に変換する。
// 定義外の値は応答本文の一部のため、文言に入れない。
func decodeWorkflowPermissions(data []byte) (WorkflowPermissions, error) {
	var p workflowPermissionsPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return WorkflowPermissions{}, fmt.Errorf("%w: %w", ErrUnexpectedResponse, err)
	}
	var missing []string
	if p.DefaultWorkflowPermissions == nil {
		missing = append(missing, "default_workflow_permissions")
	}
	if p.CanApprovePullRequestReviews == nil {
		missing = append(missing, "can_approve_pull_request_reviews")
	}
	if len(missing) > 0 {
		return WorkflowPermissions{}, fmt.Errorf("%w: missing fields: %s", ErrUnexpectedResponse, strings.Join(missing, ", "))
	}
	level, ok := setting.ParsePermissionLevel(*p.DefaultWorkflowPermissions)
	if !ok {
		return WorkflowPermissions{}, fmt.Errorf("%w: unsupported value: default_workflow_permissions", ErrUnexpectedResponse)
	}
	return WorkflowPermissions{
		DefaultWorkflowPermissions:   level,
		CanApprovePullRequestReviews: *p.CanApprovePullRequestReviews,
	}, nil
}
