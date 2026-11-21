package github

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const testToken = "ghp_test-secret-token"

// repositoryBody は GET /repos/{owner}/{repo} の応答に似せ、reco が読まないフィールドも含める。
const repositoryBody = `{
	"id": 1296269,
	"name": "repo",
	"full_name": "owner/repo",
	"owner": {"login": "owner"},
	"private": false,
	"html_url": "https://github.com/owner/repo",
	"allow_auto_merge": true,
	"delete_branch_on_merge": false,
	"permissions": {"admin": true, "push": true, "pull": true},
	"security_and_analysis": ` + repositorySecurityAndAnalysis + `
}`

// repositorySecurityAndAnalysis は repositoryBody の security_and_analysis で、スキーマにない secret_scanning_validity_checks も GitHub と同じく含める。
const repositorySecurityAndAnalysis = `{
		"secret_scanning": {"status": "enabled"},
		"secret_scanning_push_protection": {"status": "disabled"},
		"dependabot_security_updates": {"status": "enabled"},
		"secret_scanning_non_provider_patterns": {"status": "disabled"},
		"secret_scanning_validity_checks": {"status": "disabled"}
	}`

const (
	repositoryPath          = "/repos/owner/repo"
	actionsPermissionsPath  = "/repos/owner/repo/actions/permissions"
	selectedActionsPath     = "/repos/owner/repo/actions/permissions/selected-actions"
	workflowPermissionsPath = "/repos/owner/repo/actions/permissions/workflow"
	actionsAccessPath       = "/repos/owner/repo/actions/permissions/access"
	actionsAccessBody       = `{"access_level": "user"}`
	workflowPermissionsBody = `{"default_workflow_permissions": "read", "can_approve_pull_request_reviews": false}`
	selectedActionsBody     = `{"github_owned_allowed": true, "verified_allowed": false, "patterns_allowed": ["z/*", "a/*"]}`
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return New(testToken, WithBaseURL(u))
}
