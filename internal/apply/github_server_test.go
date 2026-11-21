package apply

import (
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
)

// response は stub が順に返す応答。disconnect が真なら本文を返さずに接続を切る。
type response struct {
	status     int
	body       string
	location   string
	disconnect bool
}

// newSequenceServer は、要求ごとに responses を順に返す。足りなければ 500 を返す。
func newSequenceServer(t *testing.T, responses ...response) (*url.URL, *githubtest.Server) {
	t.Helper()
	var mu sync.Mutex
	next := 0
	server := githubtest.NewServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		resp := response{status: http.StatusInternalServerError}
		if next < len(responses) {
			resp = responses[next]
			next++
		}
		mu.Unlock()

		if resp.disconnect {
			conn, _, err := http.NewResponseController(w).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			_ = conn.Close()
			return
		}
		if resp.location != "" {
			w.Header().Set("Location", resp.location)
		}
		w.WriteHeader(resp.status)
		_, _ = w.Write([]byte(resp.body))
	}))
	return server.URL(), server
}

func okRepository(id int64, allowAutoMerge, deleteBranchOnMerge bool) response {
	return response{status: http.StatusOK, body: githubtest.RepositoryBody{ID: id, AllowAutoMerge: allowAutoMerge, DeleteBranchOnMerge: deleteBranchOnMerge}.JSON()}
}

// okPrivateRepository は、非公開のリポジトリの取得の正常応答。okRepository は公開のリポジトリを返す。
func okPrivateRepository(id int64) response {
	return response{status: http.StatusOK, body: githubtest.RepositoryBody{ID: id, Private: true}.JSON()}
}

// okActionsAccess は、リポジトリ外のワークフローからのアクセスの取得の正常応答。
func okActionsAccess(level string) response {
	return response{status: http.StatusOK, body: githubtest.ActionsAccessBody{AccessLevel: level}.JSON()}
}

func okWorkflowPermissions(defaultWorkflowPermissions string, canApprovePullRequestReviews bool) response {
	return response{status: http.StatusOK, body: githubtest.WorkflowPermissionsBody{
		DefaultWorkflowPermissions: defaultWorkflowPermissions, CanApprovePullRequestReviews: canApprovePullRequestReviews}.JSON()}
}

// okAllowedActions は、Actions が有効で、Actions 許可ポリシーが policy の Actions permissions の取得の正常応答。
func okAllowedActions(policy string, shaPinningRequired bool) response {
	return response{status: http.StatusOK, body: githubtest.ActionsPermissionsBody{AllowedActions: policy, SHAPinningRequired: shaPinningRequired}.JSON()}
}

// okActionsDisabled は、Actions が無効のリポジトリの Actions permissions の取得の正常応答。
// GitHub は無効のときも sha_pinning_required を返すが、保存された値を示さない。
func okActionsDisabled() response {
	return response{status: http.StatusOK, body: githubtest.ActionsDisabledBody}
}

// okSelectedActions は、Selected actions の取得の正常応答。
func okSelectedActions(githubOwnedAllowed, verifiedAllowed bool, patternsAllowed ...string) response {
	return response{status: http.StatusOK, body: githubtest.SelectedActionsBody{
		GitHubOwnedAllowed: githubOwnedAllowed, VerifiedAllowed: verifiedAllowed, PatternsAllowed: patternsAllowed}.JSON()}
}

func newClient(baseURL *url.URL) *github.Client {
	return github.New("test-token", github.WithBaseURL(baseURL))
}

// okEnvironments は、environments を一覧の全体とする Environment の一覧の取得の正常応答。
func okEnvironments(environments ...githubtest.EnvironmentBody) response {
	return response{status: http.StatusOK, body: githubtest.EnvironmentsBody{TotalCount: len(environments), Environments: environments}.JSON()}
}

// okEnvironment は、Environment の作成と更新の正常応答。本文は個別取得と同じ形である。
func okEnvironment(environment githubtest.EnvironmentBody) response {
	return response{status: http.StatusOK, body: environment.JSON()}
}

// protectedBranchesBody は、方式が protected_branches の deployment_branch_policy。
func protectedBranchesBody() *githubtest.DeploymentBranchPolicyBody {
	return &githubtest.DeploymentBranchPolicyBody{ProtectedBranches: true}
}

// selectedBody は、方式が selected の deployment_branch_policy。
func selectedBody() *githubtest.DeploymentBranchPolicyBody {
	return &githubtest.DeploymentBranchPolicyBody{CustomBranchPolicies: true}
}

// okDeploymentPatterns は、patterns を一覧の全体とする個別パターンの一覧の取得の正常応答。
func okDeploymentPatterns(patterns ...githubtest.DeploymentPatternBody) response {
	return response{status: http.StatusOK, body: githubtest.DeploymentBranchPoliciesBody{TotalCount: len(patterns), Patterns: patterns}.JSON()}
}

// okDeploymentPattern は、個別パターンを加える要求の正常応答。
func okDeploymentPattern(pattern githubtest.DeploymentPatternBody) response {
	return response{status: http.StatusOK, body: pattern.JSON()}
}

// noContent は、個別パターンを取り除く要求の正常応答。
func noContent() response {
	return response{status: http.StatusNoContent}
}
