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
			name: "private repository body has id, the six general fields, the visibility, and the admin access, without security_and_analysis",
			body: RepositoryBody{ID: 7, AllowAutoMerge: true, AllowSquashMerge: true, AllowUpdateBranch: true, Private: true, Admin: true, SecretScanning: true}.JSON(),
			want: map[string]any{
				"id": float64(7), "allow_auto_merge": true, "delete_branch_on_merge": false,
				"allow_squash_merge": true, "allow_merge_commit": false, "allow_rebase_merge": false, "allow_update_branch": true, "private": true,
				"permissions": map[string]any{"admin": true},
			},
		},
		{
			name: "public repository body without admin access has no security_and_analysis",
			body: RepositoryBody{ID: 7, AllowMergeCommit: true, SecretScanning: true}.JSON(),
			want: map[string]any{
				"id": float64(7), "allow_auto_merge": false, "delete_branch_on_merge": false,
				"allow_squash_merge": false, "allow_merge_commit": true, "allow_rebase_merge": false, "allow_update_branch": false, "private": false,
				"permissions": map[string]any{"admin": false},
			},
		},
		{
			name: "public repository body with admin access has the secret scanning statuses in security_and_analysis",
			body: RepositoryBody{ID: 7, AllowRebaseMerge: true, Admin: true, SecretScanning: true}.JSON(),
			want: map[string]any{
				"id": float64(7), "allow_auto_merge": false, "delete_branch_on_merge": false,
				"allow_squash_merge": false, "allow_merge_commit": false, "allow_rebase_merge": true, "allow_update_branch": false, "private": false,
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
			name: "variables body has the names and values as sent, including control and non-ASCII characters",
			body: VariablesBody{TotalCount: 2, Variables: []VariableBody{{Name: "A", Value: "x\x00\x01\r\n \u3000\"\\"}}}.JSON(),
			want: map[string]any{"total_count": float64(2), "variables": []any{
				map[string]any{"name": "A", "value": "x\x00\x01\r\n \u3000\"\\"},
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

// Ruleset の一覧の応答本文は配列のため、オブジェクトの本文とは別に確かめる。
func TestRulesetBodies(t *testing.T) {
	cases := []struct {
		name string
		body string
		want any
	}{
		{
			name: "rulesets body is an array of id and name without total_count",
			body: RulesetsBody{{ID: 1, Name: "protect"}, {ID: 2, Name: "tags"}}.JSON(),
			want: []any{
				map[string]any{"id": float64(1), "name": "protect"},
				map[string]any{"id": float64(2), "name": "tags"},
			},
		},
		{
			name: "empty rulesets body is an empty array",
			body: RulesetsBody{}.JSON(),
			want: []any{},
		},
		{
			name: "ruleset body has the fields, ref_name, bypass actors with null actor_id for DeployKey, and the rules as given",
			body: RulesetBody{
				ID: 1, Name: "protect", Target: "branch", Enforcement: "active",
				Include: []string{"~DEFAULT_BRANCH"},
				BypassActors: []BypassActorBody{
					{ActorType: "RepositoryRole", ActorID: 5, BypassMode: "always"},
					{ActorType: "DeployKey", BypassMode: "always"},
				},
				Rules: []string{DeletionRuleBody},
			}.JSON(),
			want: map[string]any{
				"id": float64(1), "name": "protect", "target": "branch", "enforcement": "active",
				"conditions": map[string]any{"ref_name": map[string]any{"include": []any{"~DEFAULT_BRANCH"}, "exclude": []any{}}},
				"bypass_actors": []any{
					map[string]any{"actor_type": "RepositoryRole", "actor_id": float64(5), "bypass_mode": "always"},
					map[string]any{"actor_type": "DeployKey", "actor_id": nil, "bypass_mode": "always"},
				},
				"rules": []any{map[string]any{"type": "deletion"}},
			},
		},
		{
			name: "pull_request rule body has the five parameters and the three optional parameters at their defaults",
			body: PullRequestRuleBody{RequiredApprovingReviewCount: 1, RequireCodeOwnerReview: true}.JSON(),
			want: map[string]any{"type": "pull_request", "parameters": map[string]any{
				"required_approving_review_count": float64(1), "dismiss_stale_reviews_on_push": false, "require_code_owner_review": true,
				"require_last_push_approval": false, "required_review_thread_resolution": false,
				"allowed_merge_methods": []any{"merge", "squash", "rebase"}, "require_extra_approval_for_unattributed_changes": true, "required_reviewers": []any{},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got any
			if err := json.Unmarshal([]byte(tc.body), &got); err != nil {
				t.Fatalf("Unmarshal(%q) error = %v", tc.body, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
