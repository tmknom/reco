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

// WorkflowPermissions はワークフロー権限のうち、reco が観測する値である。
type WorkflowPermissions struct {
	DefaultWorkflowPermissions   SettingObservation[setting.PermissionLevel]
	CanApprovePullRequestReviews bool
}

func (c *Client) observeWorkflowPermissions(ctx context.Context, reference repository.Reference) (WorkflowPermissions, error) {
	_, body, err := c.send(ctx, http.MethodGet, c.workflowPermissionsURL(reference), nil, http.StatusOK)
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

// decodeWorkflowPermissions は Workflow permissions の応答本文を観測結果に変換する。
// default_workflow_permissions は GitHub が値を足し得る列挙として読み、知らない値ならその設定だけを観測不能にする。
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
	level := Unavailable[setting.PermissionLevel](UnavailableUnknownValue)
	if parsed, ok := setting.ParsePermissionLevel(*p.DefaultWorkflowPermissions); ok {
		level = Observed(parsed)
	}
	return WorkflowPermissions{
		DefaultWorkflowPermissions:   level,
		CanApprovePullRequestReviews: *p.CanApprovePullRequestReviews,
	}, nil
}
