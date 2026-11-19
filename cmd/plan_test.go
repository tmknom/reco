package cmd

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/githubtest"
)

const declaredRepository = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
`

const declaredTwoRepositories = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
  - name: owner/second
    general:
      allow_auto_merge: true
`

// mismatchedRepositoryBody は、宣言の期待値 true と異なる現在値 false を返す観測結果。
var mismatchedRepositoryBody = githubtest.RepositoryBody{ID: 1}.JSON()

func writeConfig(t *testing.T, dir, name, src string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

//trace:checks FR-002
//trace:checks FR-003
//trace:checks FR-007
//trace:checks QR-001
func TestPlan_Success(t *testing.T) {
	cases := []struct {
		name       string
		configName string
		src        string
		args       []string
		wantPaths  []string
	}{
		{name: "omitted --config reads reco.yaml in the working directory", configName: "reco.yaml", src: declaredRepository, args: []string{"plan"}, wantPaths: []string{"/repos/owner/repo"}},
		{name: "relative --config is resolved against the working directory", configName: "sub/custom.yaml", src: declaredRepository, args: []string{"plan", "--config", "sub/custom.yaml"}, wantPaths: []string{"/repos/owner/repo"}},
		{name: "--repo selects one of the declared repositories", configName: "reco.yaml", src: declaredTwoRepositories, args: []string{"plan", "--repo", "owner/second"}, wantPaths: []string{"/repos/owner/second"}},
		{name: "omitted --repo observes all declared repositories in the declared order", configName: "reco.yaml", src: declaredTwoRepositories, args: []string{"plan"}, wantPaths: []string{"/repos/owner/repo", "/repos/owner/second"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			writeConfig(t, workDir, tc.configName, tc.src)
			server, opts := newGitHubServer(t, githubtest.OK(mismatchedRepositoryBody))

			code, stdout, stderr := runExecute(t, Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue, "GITHUB_TOKEN": githubEnvValue})}, opts, tc.args...)
			if code != 0 {
				t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
			}
			if !strings.Contains(stdout, "general.allow_auto_merge: false -> true") {
				t.Errorf("got stdout %q, want the change plan", stdout)
			}
			if stderr != "" {
				t.Errorf("got stderr %q, want empty", stderr)
			}
			// plan は GitHub の設定を更新しないため、受け取る要求は観測の GET だけになる
			got := server.Requests()
			if len(got) != len(tc.wantPaths) {
				t.Fatalf("got requests %+v, want GET %v", got, tc.wantPaths)
			}
			for i, r := range got {
				if r.Method != http.MethodGet || r.Path != tc.wantPaths[i] || !strings.Contains(r.Authorization, ghEnvValue) {
					t.Errorf("got request %d %+v, want GET %s with GH_TOKEN", i, r, tc.wantPaths[i])
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

// 変更に伴う副作用は変更計画の一部として stdout に出し、Actions の実行許可の観測は宣言した対象でだけ送る。
//
//trace:checks FR-004
//trace:checks FR-007
func TestPlan_ActionsPermissions(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", "repositories:\n  - name: owner/repo\n    actions:\n      permissions:\n        allowed_actions: all\n")
	server, opts := newGitHubServer(t, githubtest.Routes(t, map[string]githubtest.Response{
		"GET /repos/owner/repo":                                      {Status: http.StatusOK, Body: repositoryBody},
		"GET /repos/owner/repo/actions/permissions":                  {Status: http.StatusOK, Body: actionsPermissionsBody},
		"GET /repos/owner/repo/actions/permissions/selected-actions": {Status: http.StatusOK, Body: selectedActionsBody},
	}))

	code, stdout, stderr := runExecute(t, Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue})}, opts, "plan")
	if code != 0 {
		t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
	}
	for _, want := range []string{"actions.permissions.allowed_actions: selected -> all\n", "    Side effect: GitHub discards the current selected_actions ("} {
		if !strings.Contains(stdout, want) {
			t.Errorf("got stdout %q, want it to contain %q", stdout, want)
		}
	}
	if stderr != "" {
		t.Errorf("got stderr %q, want empty", stderr)
	}
	wantPaths := []string{"/repos/owner/repo", "/repos/owner/repo/actions/permissions", "/repos/owner/repo/actions/permissions/selected-actions"}
	got := server.Requests()
	if len(got) != len(wantPaths) {
		t.Fatalf("got requests %+v, want paths %v", got, wantPaths)
	}
	for i, r := range got {
		if r.Method != http.MethodGet || r.Path != wantPaths[i] {
			t.Errorf("got request %d %+v, want GET %s", i, r, wantPaths[i])
		}
	}
}

// 適用不能な変更予定は、変更計画を stdout に出したうえで、終了コード 1 と 1 件のエラーで知らせる。
//
//trace:checks FR-006
//trace:checks FR-007
func TestPlan_NotApplicable(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", "repositories:\n  - name: owner/repo\n    actions:\n      permissions:\n        sha_pinning_required: true\n")
	server, opts := newGitHubServer(t, githubtest.Routes(t, map[string]githubtest.Response{
		"GET /repos/owner/repo":                     {Status: http.StatusOK, Body: repositoryBody},
		"GET /repos/owner/repo/actions/permissions": {Status: http.StatusOK, Body: githubtest.ActionsDisabledBody},
	}))

	code, stdout, stderr := runExecute(t, Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue})}, opts, "plan")
	if code != 1 {
		t.Errorf("got exit code %d, want 1 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "Repository: owner/repo\n") {
		t.Errorf("got stdout %q, want the change plan", stdout)
	}
	if records := errorRecords(t, stderr); len(records) != 1 || records[0]["msg"] != "command failed" {
		t.Errorf("got ERROR records %v, want one command failed", records)
	}
	if strings.Contains(stdout, `"level"`) {
		t.Errorf("got stdout %q, want no log records", stdout)
	}
	for _, r := range server.Requests() {
		if r.Method != http.MethodGet {
			t.Errorf("got request %+v, want only GET", r)
		}
	}
}

func TestPlan_InvalidInput(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "empty --config", args: []string{"plan", "--config", ""}},
		{name: "malformed --repo", args: []string{"plan", "--repo", "owner"}},
		{name: "extra positional argument", args: []string{"plan", "extra"}},
		{name: "unknown flag", args: []string{"plan", "--unknown"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			writeConfig(t, workDir, "reco.yaml", declaredRepository)
			server, opts := newGitHubServer(t, githubtest.OK(mismatchedRepositoryBody))

			code, stdout, stderr := runExecute(t, Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue})}, opts, tc.args...)
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
func TestPlan_TokenMissing(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredRepository)
	server, opts := newGitHubServer(t, githubtest.OK(mismatchedRepositoryBody))

	code, stdout, stderr := runExecute(t, Environment{WorkDir: workDir, Getenv: fakeGetenv(nil)}, opts, "plan")
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

//trace:checks FR-001
//trace:checks QR-002
func TestPlan_ConfigNotFound(t *testing.T) {
	server, opts := newGitHubServer(t, githubtest.OK(mismatchedRepositoryBody))

	code, stdout, stderr := runExecute(t, Environment{WorkDir: t.TempDir(), Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue})}, opts, "plan")
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
	if msg, _ := records[0]["error"].(string); !strings.Contains(msg, "./reco.yaml") {
		t.Errorf("got error %q, want it to contain the configuration path", msg)
	}
	if got := server.Requests(); len(got) != 0 {
		t.Errorf("got %d requests, want none", len(got))
	}
}

// 観測操作の失敗の種類ごとの文言は internal/github が決めて検証しているため、ここでは失敗が計画として表示されず、
// 対象と理由が stderr に届くことを 1 種類で確かめる。
//
//trace:checks FR-004
//trace:checks QR-001
func TestPlan_ObservationFailure(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredRepository)
	server, opts := newGitHubServer(t, githubtest.Response{Status: http.StatusUnauthorized, Body: `{"message": "Bad credentials"}`})

	code, stdout, stderr := runExecute(t, Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue, "GITHUB_TOKEN": githubEnvValue})}, opts, "plan")
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

// 3件目は stub に登録せず、要求が届けば stub がテストを失敗させる。
//
//trace:checks FR-008
func TestPlan_SecondObservationFailure(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredTwoRepositories+"  - name: owner/third\n    general:\n      allow_auto_merge: true\n")
	server, opts := newGitHubServer(t, githubtest.Routes(t, map[string]githubtest.Response{
		"GET /repos/owner/repo":   {Status: http.StatusOK, Body: mismatchedRepositoryBody},
		"GET /repos/owner/second": {Status: http.StatusUnauthorized, Body: `{"message": "Bad credentials"}`},
	}))

	code, stdout, stderr := runExecute(t, Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue})}, opts, "plan")
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
	if msg, _ := records[0]["error"].(string); !strings.Contains(msg, "owner/second") {
		t.Errorf("got error %q, want it to contain the failed repository", msg)
	}
	if got := server.Requests(); len(got) != 2 {
		t.Errorf("got requests %+v, want the first two GETs only", got)
	}
}
