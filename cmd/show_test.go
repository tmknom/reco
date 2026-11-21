package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
)

//trace:checks FR-003 FR-004 QR-001
func TestShow_Success(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "repository settings are shown", args: []string{"show", "--repo", "owner/repo"}},
		{name: "--log-level after the command is accepted", args: []string{"show", "--log-level", "debug", "--repo", "owner/repo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, opts := newGitHubServer(t, githubtest.Routes(t, map[string]githubtest.Response{
				// 非公開のリポジトリにし、show がリポジトリ外のワークフローからのアクセスまで読むことを確かめる
				"GET /repos/owner/repo":                                      githubtest.OK(githubtest.RepositoryBody{ID: 1, AllowAutoMerge: true, Private: true, Admin: true}.JSON()),
				"GET /repos/owner/repo/actions/permissions":                  {Status: http.StatusOK, Body: actionsPermissionsBody},
				"GET /repos/owner/repo/actions/permissions/selected-actions": {Status: http.StatusOK, Body: selectedActionsBody},
				"GET /repos/owner/repo/actions/permissions/workflow":         {Status: http.StatusOK, Body: workflowPermissionsBody},
				"GET /repos/owner/repo/actions/permissions/access":           githubtest.OK(githubtest.ActionsAccessBody{AccessLevel: "none"}.JSON()),
				"GET /repos/owner/repo/environments":                         githubtest.OK(githubtest.EnvironmentsBody{}.JSON()),
				"GET /repos/owner/repo/vulnerability-alerts":                 {Status: http.StatusNoContent},
				"GET /repos/owner/repo/automated-security-fixes":             githubtest.OK(githubtest.AutomatedSecurityFixesBody{Enabled: true}.JSON()),
			}))

			code, stdout, stderr := runExecute(t, Environment{Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue, "GITHUB_TOKEN": githubEnvValue})}, opts, tc.args...)
			if code != 0 {
				t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
			}
			if !strings.Contains(stdout, "allow_auto_merge: true") {
				t.Errorf("got stdout %q, want it to contain the observed allow_auto_merge", stdout)
			}
			if stderr != "" {
				t.Errorf("got stderr %q, want empty", stderr)
			}
			wantPaths := []string{
				"/repos/owner/repo",
				"/repos/owner/repo/actions/permissions",
				"/repos/owner/repo/actions/permissions/selected-actions",
				"/repos/owner/repo/actions/permissions/workflow",
				"/repos/owner/repo/actions/permissions/access",
				"/repos/owner/repo/environments",
				"/repos/owner/repo/vulnerability-alerts",
				"/repos/owner/repo/automated-security-fixes",
			}
			got := server.Requests()
			if len(got) != len(wantPaths) {
				t.Fatalf("got requests %+v, want paths %v", got, wantPaths)
			}
			for i, r := range got {
				if r.Method != http.MethodGet || r.Path != wantPaths[i] || !strings.Contains(r.Authorization, ghEnvValue) {
					t.Errorf("got request %d %+v, want GET %s with GH_TOKEN", i, r, wantPaths[i])
				}
			}
			for _, token := range []string{ghEnvValue, githubEnvValue} {
				if strings.Contains(stdout+stderr, token) {
					t.Errorf("got output containing token %q", token)
				}
			}
		})
	}
}

// show の表示が設定ファイルとして読め、観測した値を期待値とする宣言になることを、plan が No changes. を返すことで確かめる。
//
//trace:checks FR-004
func TestShow_OutputReadsAsConfiguration(t *testing.T) {
	const environmentName = `prod "a" #1: x`
	selected := &githubtest.DeploymentBranchPolicyBody{CustomBranchPolicies: true}
	waitTimer := 30
	cases := []struct {
		name   string
		routes map[string]githubtest.Response
	}{
		{
			name: "private repository with selected policy and environments reads back with escaped strings",
			routes: map[string]githubtest.Response{
				"GET /repos/owner/repo":                     githubtest.OK(githubtest.RepositoryBody{ID: 1, DeleteBranchOnMerge: true, Private: true, Admin: true}.JSON()),
				"GET /repos/owner/repo/actions/permissions": githubtest.OK(githubtest.ActionsPermissionsBody{AllowedActions: "selected", SHAPinningRequired: true}.JSON()),
				"GET /repos/owner/repo/actions/permissions/selected-actions": githubtest.OK(githubtest.SelectedActionsBody{
					GitHubOwnedAllowed: true, PatternsAllowed: []string{`a"b\c`, "é/*", "#x: y", "\u200b"},
				}.JSON()),
				"GET /repos/owner/repo/actions/permissions/workflow": githubtest.OK(githubtest.WorkflowPermissionsBody{DefaultWorkflowPermissions: "write", CanApprovePullRequestReviews: true}.JSON()),
				"GET /repos/owner/repo/actions/permissions/access":   githubtest.OK(githubtest.ActionsAccessBody{AccessLevel: "user"}.JSON()),
				"GET /repos/owner/repo/environments": githubtest.OK(githubtest.EnvironmentsBody{TotalCount: 3, Environments: []githubtest.EnvironmentBody{
					{ID: 2, Name: environmentName, DeploymentBranchPolicy: selected},
					{ID: 3, Name: "empty", DeploymentBranchPolicy: selected},
					// 保護設定を持たせる。show が保護設定を表示すれば、plan が未知のキーとして拒否する
					{ID: 4, Name: "staging", DeploymentBranchPolicy: &githubtest.DeploymentBranchPolicyBody{ProtectedBranches: true}, WaitTimer: &waitTimer},
				}}.JSON()),
				"GET /repos/owner/repo/environments/" + environmentName + "/deployment-branch-policies": githubtest.OK(githubtest.DeploymentBranchPoliciesBody{TotalCount: 2, Patterns: []githubtest.DeploymentPatternBody{
					{ID: 5, Name: "release/*", Type: "branch"},
					{ID: 6, Name: `v"\*`, Type: "tag"},
				}}.JSON()),
				"GET /repos/owner/repo/environments/empty/deployment-branch-policies": githubtest.OK(githubtest.DeploymentBranchPoliciesBody{}.JSON()),
				"GET /repos/owner/repo/vulnerability-alerts":                          {Status: http.StatusNoContent},
				// 一時停止中の行末のコメントも、設定宣言として読むことを妨げない
				"GET /repos/owner/repo/automated-security-fixes": githubtest.OK(githubtest.AutomatedSecurityFixesBody{Enabled: true, Paused: true}.JSON()),
			},
		},
		{
			name: "public repository with disabled actions and no environments reads back without the commented lines",
			routes: map[string]githubtest.Response{
				"GET /repos/owner/repo":                              githubtest.OK(githubtest.RepositoryBody{ID: 1, Admin: true}.JSON()),
				"GET /repos/owner/repo/actions/permissions":          githubtest.OK(`{"enabled": false}`),
				"GET /repos/owner/repo/actions/permissions/workflow": githubtest.OK(githubtest.WorkflowPermissionsBody{DefaultWorkflowPermissions: "read"}.JSON()),
				"GET /repos/owner/repo/environments":                 githubtest.OK(githubtest.EnvironmentsBody{}.JSON()),
				"GET /repos/owner/repo/vulnerability-alerts":         {Status: http.StatusNotFound},
				"GET /repos/owner/repo/automated-security-fixes":     githubtest.OK(githubtest.AutomatedSecurityFixesBody{}.JSON()),
			},
		},
		{
			name: "local_only policy without selected actions reads back",
			routes: map[string]githubtest.Response{
				"GET /repos/owner/repo":                              githubtest.OK(githubtest.RepositoryBody{ID: 1, Private: true, Admin: true}.JSON()),
				"GET /repos/owner/repo/actions/permissions":          githubtest.OK(githubtest.ActionsPermissionsBody{AllowedActions: "local_only"}.JSON()),
				"GET /repos/owner/repo/actions/permissions/workflow": githubtest.OK(githubtest.WorkflowPermissionsBody{DefaultWorkflowPermissions: "read"}.JSON()),
				"GET /repos/owner/repo/actions/permissions/access":   githubtest.OK(githubtest.ActionsAccessBody{AccessLevel: "none"}.JSON()),
				"GET /repos/owner/repo/environments": githubtest.OK(githubtest.EnvironmentsBody{TotalCount: 1, Environments: []githubtest.EnvironmentBody{
					{ID: 2, Name: "production"},
				}}.JSON()),
				"GET /repos/owner/repo/vulnerability-alerts":     {Status: http.StatusNotFound},
				"GET /repos/owner/repo/automated-security-fixes": githubtest.OK(githubtest.AutomatedSecurityFixesBody{}.JSON()),
			},
		},
		{
			name: "selected policy without patterns reads back with the empty sequence",
			routes: map[string]githubtest.Response{
				"GET /repos/owner/repo":                                      githubtest.OK(githubtest.RepositoryBody{ID: 1, Private: true, Admin: true}.JSON()),
				"GET /repos/owner/repo/actions/permissions":                  githubtest.OK(githubtest.ActionsPermissionsBody{AllowedActions: "selected"}.JSON()),
				"GET /repos/owner/repo/actions/permissions/selected-actions": githubtest.OK(githubtest.SelectedActionsBody{VerifiedAllowed: true}.JSON()),
				"GET /repos/owner/repo/actions/permissions/workflow":         githubtest.OK(githubtest.WorkflowPermissionsBody{DefaultWorkflowPermissions: "read"}.JSON()),
				"GET /repos/owner/repo/actions/permissions/access":           githubtest.OK(githubtest.ActionsAccessBody{AccessLevel: "none"}.JSON()),
				"GET /repos/owner/repo/environments":                         githubtest.OK(githubtest.EnvironmentsBody{}.JSON()),
				"GET /repos/owner/repo/vulnerability-alerts":                 {Status: http.StatusNotFound},
				"GET /repos/owner/repo/automated-security-fixes":             githubtest.OK(githubtest.AutomatedSecurityFixesBody{}.JSON()),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, opts := newGitHubServer(t, githubtest.Routes(t, tc.routes))
			env := Environment{WorkDir: t.TempDir(), Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue})}

			code, shown, stderr := runExecute(t, env, opts, "show", "--repo", "owner/repo")
			if code != 0 {
				t.Fatalf("show got exit code %d, want 0 (stderr %q)", code, stderr)
			}
			writeConfig(t, env.WorkDir, "reco.yaml", shown)

			code, stdout, stderr := runExecute(t, env, opts, "plan")
			if code != 0 {
				t.Errorf("plan got exit code %d, want 0 (show wrote %q, stderr %q)", code, shown, stderr)
			}
			if !strings.Contains(stdout, "No changes.") {
				t.Errorf("plan got stdout %q, want No changes. for the configuration %q", stdout, shown)
			}
		})
	}
}

func TestShow_InvalidInput(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "missing --repo", args: []string{"show"}},
		{name: "empty --repo", args: []string{"show", "--repo", ""}},
		{name: "--config is not defined for show", args: []string{"show", "--repo", "owner/repo", "--config", "reco.yaml"}},
		{name: "extra positional argument", args: []string{"show", "--repo", "owner/repo", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, opts := newGitHubServer(t, githubtest.OK(repositoryBody))

			code, stdout, stderr := runExecute(t, Environment{Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue})}, opts, tc.args...)
			if code != 1 {
				t.Errorf("got exit code %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("got stdout %q, want empty", stdout)
			}
			records := errorRecords(t, stderr)
			if len(records) != 1 || records[0]["msg"] != "command failed" {
				t.Errorf("got ERROR records %v, want one command failed record", records)
			}
			if got := server.Requests(); len(got) != 0 {
				t.Errorf("got %d requests, want none", len(got))
			}
		})
	}
}

//trace:checks FR-003
func TestShow_TokenMissing(t *testing.T) {
	server, opts := newGitHubServer(t, githubtest.OK(repositoryBody))

	code, stdout, stderr := runExecute(t, Environment{Getenv: fakeGetenv(nil)}, opts, "show", "--repo", "owner/repo")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("got stdout %q, want empty", stdout)
	}
	records := errorRecords(t, stderr)
	if len(records) != 1 || records[0]["msg"] != "command failed" {
		t.Errorf("got ERROR records %v, want one command failed record", records)
	}
	if got := server.Requests(); len(got) != 0 {
		t.Errorf("got %d requests, want none", len(got))
	}
}

// 観測操作の失敗の種類ごとの文言は internal/github が決めて検証しているため、ここでは失敗が結果として表示されず、
// 対象と理由が stderr に届くことを 1 種類で確かめる。
//
//trace:checks FR-003 FR-004 QR-001
func TestShow_ObservationFailure(t *testing.T) {
	server, opts := newGitHubServer(t, githubtest.Response{Status: http.StatusUnauthorized, Body: `{"message": "Bad credentials"}`})

	code, stdout, stderr := runExecute(t, Environment{Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue, "GITHUB_TOKEN": githubEnvValue})}, opts, "show", "--repo", "owner/repo")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("got stdout %q, want empty", stdout)
	}
	records := errorRecords(t, stderr)
	if len(records) != 1 || records[0]["msg"] != "command failed" {
		t.Fatalf("got ERROR records %v, want one command failed record", records)
	}
	if msg, _ := records[0]["error"].(string); !strings.Contains(msg, "owner/repo") || !strings.Contains(msg, github.ErrUnauthorized.Error()) {
		t.Errorf("got error %q, want it to contain the target and the authentication failure", msg)
	}
	if got := server.Requests(); len(got) != 1 || !strings.Contains(got[0].Authorization, ghEnvValue) {
		t.Errorf("got requests %+v, want one request with GH_TOKEN and no retry with GITHUB_TOKEN", got)
	}
	for _, token := range []string{ghEnvValue, githubEnvValue} {
		if strings.Contains(stdout+stderr, token) {
			t.Errorf("got output containing token %q", token)
		}
	}
}
