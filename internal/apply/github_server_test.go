package apply

import (
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
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

// okRepository は、トークンの所有者が admin のアクセスを持つ、公開のリポジトリの取得の正常応答。Secret scanning の2設定は無効である。
// 設定を更新できるのは admin のアクセスを持つときだけのため、apply の観測は admin のアクセスを前提にする。
// Dependabot alerts の取得の 404 を無効と読むにも、同じ観測でこのアクセスを確かめている必要がある。
func okRepository(id int64, allowAutoMerge, deleteBranchOnMerge bool) response {
	return response{status: http.StatusOK, body: githubtest.RepositoryBody{ID: id, AllowAutoMerge: allowAutoMerge, DeleteBranchOnMerge: deleteBranchOnMerge, Admin: true}.JSON()}
}

// okSecretScanningRepository は、okRepository の Secret scanning の2設定を enabled と pushProtection にした応答。
func okSecretScanningRepository(id int64, enabled, pushProtection bool) response {
	return okRepositoryBody(githubtest.RepositoryBody{ID: id, SecretScanning: enabled, SecretScanningPushProtection: pushProtection})
}

// okRepositoryBody は、admin のアクセスを持つ公開のリポジトリについて、body の値を返す取得と更新の正常応答。
func okRepositoryBody(body githubtest.RepositoryBody) response {
	body.Admin = true
	return response{status: http.StatusOK, body: body.JSON()}
}

// okPrivateRepository は、非公開のリポジトリの取得の正常応答。okRepository は公開のリポジトリを返す。
func okPrivateRepository(id int64) response {
	return response{status: http.StatusOK, body: githubtest.RepositoryBody{ID: id, Private: true}.JSON()}
}

// okActionsAccess は、リポジトリ外のワークフローからのアクセスの取得の正常応答。
func okActionsAccess(level string) response {
	return response{status: http.StatusOK, body: githubtest.ActionsAccessBody{AccessLevel: level}.JSON()}
}

// okForkPullRequestApproval は、Fork Pull Request ワークフローの承認ポリシーの取得の正常応答。
func okForkPullRequestApproval(policy string) response {
	return response{status: http.StatusOK, body: githubtest.ForkPullRequestApprovalBody{ApprovalPolicy: policy}.JSON()}
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

// okVariables は、variables を一覧の全体とする変数の一覧の取得の正常応答。Repository Variables と Environment Variables で同じ形である。
func okVariables(variables ...githubtest.VariableBody) response {
	return response{status: http.StatusOK, body: githubtest.VariablesBody{TotalCount: len(variables), Variables: variables}.JSON()}
}

// createdVariable は、Repository Variable と Environment Variable の作成の正常応答。GitHub は 201 で、変数を含まない空のオブジェクトを返す。
func createdVariable() response {
	return response{status: http.StatusCreated, body: `{}`}
}

// okRulesets は、rulesets を一覧の全体とする Ruleset の一覧の取得の正常応答。
func okRulesets(rulesets ...githubtest.RulesetSummaryBody) response {
	return response{status: http.StatusOK, body: githubtest.RulesetsBody(rulesets).JSON()}
}

// okRuleset は、Ruleset の詳細の取得と値の変更の正常応答。GitHub は値の変更にも 200 で、Ruleset の全体を詳細の取得と同じ形で返す。
func okRuleset(ruleset githubtest.RulesetBody) response {
	return response{status: http.StatusOK, body: ruleset.JSON()}
}

// createdRuleset は、Ruleset の作成の正常応答。GitHub は 201 で、作った Ruleset の全体を詳細の取得と同じ形で返す。
func createdRuleset(ruleset githubtest.RulesetBody) response {
	return response{status: http.StatusCreated, body: ruleset.JSON()}
}

// protectRulesetBody は、protectRuleset(enforcement) と同じ値を持つ、ID が id で名前が name の Ruleset の詳細の本文。
func protectRulesetBody(id int64, name string, enforcement setting.RulesetEnforcement) githubtest.RulesetBody {
	return githubtest.RulesetBody{
		ID: id, Name: name, Target: "branch", Enforcement: string(enforcement),
		Include: []string{"~DEFAULT_BRANCH"}, Exclude: []string{},
		BypassActors: []githubtest.BypassActorBody{{ActorType: "RepositoryRole", ActorID: 5, BypassMode: "always"}},
		Rules:        []string{githubtest.DeletionRuleBody},
	}
}

// okDeploymentPatterns は、patterns を一覧の全体とする個別パターンの一覧の取得の正常応答。
func okDeploymentPatterns(patterns ...githubtest.DeploymentPatternBody) response {
	return response{status: http.StatusOK, body: githubtest.DeploymentBranchPoliciesBody{TotalCount: len(patterns), Patterns: patterns}.JSON()}
}

// okDeploymentPattern は、個別パターンを加える要求の正常応答。
func okDeploymentPattern(pattern githubtest.DeploymentPatternBody) response {
	return response{status: http.StatusOK, body: pattern.JSON()}
}

// dependabotAlerts は、Dependabot alerts の取得の応答。GitHub は有効を 204、無効を 404 で返す。
func dependabotAlerts(enabled bool) response {
	if enabled {
		return noContent()
	}
	return response{status: http.StatusNotFound, body: `{"message": "Vulnerability alerts are disabled."}`}
}

// okSecurityUpdates は、Dependabot security updates の取得の正常応答。
func okSecurityUpdates(enabled, paused bool) response {
	return response{status: http.StatusOK, body: githubtest.AutomatedSecurityFixesBody{Enabled: enabled, Paused: paused}.JSON()}
}

// noContent は、本文のない正常応答。個別パターンを取り除く要求と、Dependabot の更新と、有効な Dependabot alerts の取得と、変数の値の変更が返す。
func noContent() response {
	return response{status: http.StatusNoContent}
}
