package githubtest

import (
	"encoding/json"
	"reflect"
	"testing"
)

// 期待値の項目名は、GitHub REST API の各取得エンドポイントの応答の項目名から書く。
func TestBodies(t *testing.T) {
	cases := []struct {
		name string
		body string
		want map[string]any
	}{
		{
			name: "repository body has id, the two merge-related fields, and the visibility",
			body: RepositoryBody{ID: 7, AllowAutoMerge: true, Private: true}.JSON(),
			want: map[string]any{"id": float64(7), "allow_auto_merge": true, "delete_branch_on_merge": false, "private": true},
		},
		{
			name: "actions permissions body is enabled with the policy and SHA pinning",
			body: ActionsPermissionsBody{AllowedActions: "selected", SHAPinningRequired: true}.JSON(),
			want: map[string]any{"enabled": true, "allowed_actions": "selected", "sha_pinning_required": true},
		},
		{
			name: "selected actions body keeps patterns in the written order",
			body: SelectedActionsBody{GitHubOwnedAllowed: true, PatternsAllowed: []string{"b/*", "a/*"}}.JSON(),
			want: map[string]any{"github_owned_allowed": true, "verified_allowed": false, "patterns_allowed": []any{"b/*", "a/*"}},
		},
		{
			name: "selected actions body without patterns has an empty array",
			body: SelectedActionsBody{VerifiedAllowed: true}.JSON(),
			want: map[string]any{"github_owned_allowed": false, "verified_allowed": true, "patterns_allowed": []any{}},
		},
		{
			name: "workflow permissions body has the default permissions and approval",
			body: WorkflowPermissionsBody{DefaultWorkflowPermissions: "write", CanApprovePullRequestReviews: true}.JSON(),
			want: map[string]any{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": true},
		},
		{
			name: "actions access body has the access level",
			body: ActionsAccessBody{AccessLevel: "user"}.JSON(),
			want: map[string]any{"access_level": "user"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]any
			if err := json.Unmarshal([]byte(tc.body), &got); err != nil {
				t.Fatalf("Unmarshal(%q) error = %v", tc.body, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
