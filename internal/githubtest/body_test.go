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
			name: "private repository body has id, the two merge-related fields, the visibility, and the admin access, without security_and_analysis",
			body: RepositoryBody{ID: 7, AllowAutoMerge: true, Private: true, Admin: true, SecretScanning: true}.JSON(),
			want: map[string]any{
				"id": float64(7), "allow_auto_merge": true, "delete_branch_on_merge": false, "private": true,
				"permissions": map[string]any{"admin": true},
			},
		},
		{
			name: "public repository body without admin access has no security_and_analysis",
			body: RepositoryBody{ID: 7, SecretScanning: true}.JSON(),
			want: map[string]any{
				"id": float64(7), "allow_auto_merge": false, "delete_branch_on_merge": false, "private": false,
				"permissions": map[string]any{"admin": false},
			},
		},
		{
			name: "public repository body with admin access has the secret scanning statuses in security_and_analysis",
			body: RepositoryBody{ID: 7, Admin: true, SecretScanning: true}.JSON(),
			want: map[string]any{
				"id": float64(7), "allow_auto_merge": false, "delete_branch_on_merge": false, "private": false,
				"permissions": map[string]any{"admin": true},
				"security_and_analysis": map[string]any{
					"secret_scanning":                 map[string]any{"status": "enabled"},
					"secret_scanning_push_protection": map[string]any{"status": "disabled"},
				},
			},
		},
		{
			name: "automated security fixes body has enabled and paused",
			body: AutomatedSecurityFixesBody{Enabled: true, Paused: true}.JSON(),
			want: map[string]any{"enabled": true, "paused": true},
		},
		{
			name: "private vulnerability reporting body has enabled",
			body: PrivateVulnerabilityReportingBody{Enabled: true}.JSON(),
			want: map[string]any{"enabled": true},
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
		{
			name: "environments body without protection rules has a null policy and an empty rule array",
			body: EnvironmentsBody{TotalCount: 3, Environments: []EnvironmentBody{{ID: 1, Name: "dev", CanAdminsBypass: true}}}.JSON(),
			want: map[string]any{"total_count": float64(3), "environments": []any{
				map[string]any{"id": float64(1), "name": "dev", "can_admins_bypass": true, "protection_rules": []any{}, "deployment_branch_policy": nil},
			}},
		},
		{
			name: "environments body with every protection rule has them in the GitHub order",
			body: EnvironmentsBody{TotalCount: 1, Environments: []EnvironmentBody{{
				ID:                     2,
				Name:                   "prod",
				DeploymentBranchPolicy: &DeploymentBranchPolicyBody{CustomBranchPolicies: true},
				WaitTimer:              new(5),
				RequiredReviewers:      &RequiredReviewersBody{PreventSelfReview: true, Reviewers: []ReviewerBody{{Type: "Team", ID: 9}}},
			}}}.JSON(),
			want: map[string]any{"total_count": float64(1), "environments": []any{
				map[string]any{
					"id":                float64(2),
					"name":              "prod",
					"can_admins_bypass": false,
					"protection_rules": []any{
						map[string]any{"type": "required_reviewers", "prevent_self_review": true, "reviewers": []any{
							map[string]any{"type": "Team", "reviewer": map[string]any{"id": float64(9)}},
						}},
						map[string]any{"type": "wait_timer", "wait_timer": float64(5)},
						map[string]any{"type": "branch_policy"},
					},
					"deployment_branch_policy": map[string]any{"protected_branches": false, "custom_branch_policies": true},
				},
			}},
		},
		{
			name: "single environment body is the element object itself",
			body: EnvironmentBody{ID: 3, Name: "stg", DeploymentBranchPolicy: &DeploymentBranchPolicyBody{ProtectedBranches: true}}.JSON(),
			want: map[string]any{
				"id":                       float64(3),
				"name":                     "stg",
				"can_admins_bypass":        false,
				"protection_rules":         []any{map[string]any{"type": "branch_policy"}},
				"deployment_branch_policy": map[string]any{"protected_branches": true, "custom_branch_policies": false},
			},
		},
		{
			name: "deployment branch policies body has the patterns with their type",
			body: DeploymentBranchPoliciesBody{TotalCount: 2, Patterns: []DeploymentPatternBody{{ID: 4, Name: "main", Type: "tag"}}}.JSON(),
			want: map[string]any{"total_count": float64(2), "branch_policies": []any{
				map[string]any{"id": float64(4), "name": "main", "type": "tag"},
			}},
		},
		{
			name: "single deployment pattern body is the element object itself",
			body: DeploymentPatternBody{ID: 5, Name: "release/*", Type: "branch"}.JSON(),
			want: map[string]any{"id": float64(5), "name": "release/*", "type": "branch"},
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
