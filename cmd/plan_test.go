package cmd

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
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
const mismatchedRepositoryBody = `{"id": 1, "allow_auto_merge": false, "delete_branch_on_merge": false}`

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
		wantPath   string
	}{
		{name: "omitted --config reads reco.yaml in the working directory", configName: "reco.yaml", src: declaredRepository, args: []string{"plan"}, wantPath: "/repos/owner/repo"},
		{name: "relative --config is resolved against the working directory", configName: "sub/custom.yaml", src: declaredRepository, args: []string{"plan", "--config", "sub/custom.yaml"}, wantPath: "/repos/owner/repo"},
		{name: "--repo selects one of the declared repositories", configName: "reco.yaml", src: declaredTwoRepositories, args: []string{"plan", "--repo", "owner/second"}, wantPath: "/repos/owner/second"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			writeConfig(t, workDir, tc.configName, tc.src)
			server, opts := newGitHubServer(t, http.StatusOK, mismatchedRepositoryBody)

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
			got := server.receivedRequests()
			if len(got) != 1 || got[0].method != http.MethodGet || got[0].path != tc.wantPath || !strings.Contains(got[0].authorization, ghEnvValue) {
				t.Errorf("got requests %+v, want one GET %s with GH_TOKEN", got, tc.wantPath)
			}
			for _, token := range []string{ghEnvValue, githubEnvValue} {
				if strings.Contains(stdout+stderr, token) {
					t.Errorf("got output containing token %q", token)
				}
			}
		})
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
			server, opts := newGitHubServer(t, http.StatusOK, mismatchedRepositoryBody)

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
			if got := server.receivedRequests(); len(got) != 0 {
				t.Errorf("got %d requests, want none", len(got))
			}
		})
	}
}

//trace:checks FR-003
func TestPlan_TokenMissing(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredRepository)
	server, opts := newGitHubServer(t, http.StatusOK, mismatchedRepositoryBody)

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
	if got := server.receivedRequests(); len(got) != 0 {
		t.Errorf("got %d requests, want none", len(got))
	}
}

//trace:checks FR-001
//trace:checks QR-002
func TestPlan_ConfigNotFound(t *testing.T) {
	server, opts := newGitHubServer(t, http.StatusOK, mismatchedRepositoryBody)

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
	if got := server.receivedRequests(); len(got) != 0 {
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
	server, opts := newGitHubServer(t, http.StatusUnauthorized, `{"message": "Bad credentials"}`)

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
	if got := server.receivedRequests(); len(got) != 1 || !strings.Contains(got[0].authorization, ghEnvValue) {
		t.Errorf("got requests %+v, want one request with GH_TOKEN and no retry with GITHUB_TOKEN", got)
	}
	for _, token := range []string{ghEnvValue, githubEnvValue} {
		if strings.Contains(stdout+stderr, token) {
			t.Errorf("got output containing token %q", token)
		}
	}
}
