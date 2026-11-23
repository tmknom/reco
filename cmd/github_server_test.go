package cmd

import (
	"net/http"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
)

const (
	ghEnvValue     = "gh-token-value"
	githubEnvValue = "github-token-value"
)

var (
	repositoryBody          = githubtest.RepositoryBody{ID: 1, AllowAutoMerge: true}.JSON()
	workflowPermissionsBody = githubtest.WorkflowPermissionsBody{DefaultWorkflowPermissions: "read"}.JSON()
	actionsPermissionsBody  = githubtest.ActionsPermissionsBody{AllowedActions: "selected"}.JSON()
	selectedActionsBody     = githubtest.SelectedActionsBody{GitHubOwnedAllowed: true, PatternsAllowed: []string{"example/*"}}.JSON()
)

// mismatchedRepositoryBody は、宣言の期待値 true と異なる現在値 false を返す観測結果。
var mismatchedRepositoryBody = githubtest.RepositoryBody{ID: 1, Admin: true}.JSON()

// newGitHubServer は、handler に応答させる GitHub の stub を立て、execute に渡す接続先の Option を組む。
func newGitHubServer(t *testing.T, handler http.Handler) (*githubtest.Server, []github.Option) {
	t.Helper()
	server := githubtest.NewServer(t, handler)
	return server, []github.Option{github.WithBaseURL(server.URL())}
}
