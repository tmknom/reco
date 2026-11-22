package github

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
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
	"allow_squash_merge": true,
	"allow_merge_commit": false,
	"allow_rebase_merge": true,
	"allow_update_branch": false,
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

// escapedPathRoutes は、パスごとに登録した応答を返す。パスは r.URL.EscapedPath() で比べ、%2F を区切りと取り違えない。
// 検索条件を含めた要求の列を、届いた順に requests へ記録する。登録のない要求は 500 を返し、テストの失敗として報告する。
type escapedPathRoutes struct {
	t        *testing.T
	routes   map[string]githubtest.Response
	mu       sync.Mutex
	requests []string
}

func (e *escapedPathRoutes) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.mu.Lock()
	e.requests = append(e.requests, r.URL.EscapedPath()+"?"+r.URL.RawQuery)
	e.mu.Unlock()
	response, ok := e.routes[r.URL.EscapedPath()]
	if !ok {
		e.t.Errorf("got request %s, want one of the registered routes", r.URL)
		response = githubtest.Response{Status: http.StatusInternalServerError}
	}
	response.ServeHTTP(w, r)
}

func (e *escapedPathRoutes) Requests() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.requests...)
}
