package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

// appliedRepositoryBody は、宣言の期待値 true に更新されたあとの観測結果。
const appliedRepositoryBody = `{"id": 1, "allow_auto_merge": true, "delete_branch_on_merge": false}`

func methodsOf(requests []receivedRequest) []string {
	var methods []string
	for _, r := range requests {
		methods = append(methods, r.method)
	}
	return methods
}

//trace:checks FR-008
//trace:checks FR-002
func TestApply_NoChanges(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "sub/custom.yaml", declaredTwoRepositories)
	server, opts := newGitHubServer(t, http.StatusOK, appliedRepositoryBody)

	env := Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}), Stdin: unreadableReader{t}, StdinIsTerminal: true}
	code, stdout, stderr := runExecute(t, env, opts, "apply", "--config", "sub/custom.yaml", "--repo", "owner/second")
	if code != 0 {
		t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "No changes.") {
		t.Errorf("got stdout %q, want No changes.", stdout)
	}
	if stderr != "" {
		t.Errorf("got stderr %q, want no approval prompt", stderr)
	}
	got := server.receivedRequests()
	if len(got) != 1 || got[0].method != http.MethodGet || got[0].path != "/repos/owner/second" {
		t.Errorf("got requests %+v, want one GET /repos/owner/second", got)
	}
}

//trace:checks FR-009
//trace:checks FR-011
//trace:checks QR-001
func TestApply_Approved(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredRepository)
	server, opts := newGitHubServerSequence(t,
		stubResponse{http.StatusOK, mismatchedRepositoryBody},
		stubResponse{http.StatusOK, mismatchedRepositoryBody},
		stubResponse{http.StatusOK, appliedRepositoryBody},
		stubResponse{http.StatusOK, appliedRepositoryBody},
	)

	env := Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue, "GITHUB_TOKEN": githubEnvValue}), Stdin: strings.NewReader("y\n"), StdinIsTerminal: true}
	code, stdout, stderr := runExecute(t, env, opts, "apply")
	if code != 0 {
		t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "owner/repo: updated") {
		t.Errorf("got stdout %q, want owner/repo: updated", stdout)
	}
	if stderr != approvalPrompt {
		t.Errorf("got stderr %q, want only the approval prompt", stderr)
	}
	got := server.receivedRequests()
	want := []string{http.MethodGet, http.MethodGet, http.MethodPatch, http.MethodGet}
	if !slices.Equal(methodsOf(got), want) {
		t.Errorf("got requests %+v, want methods %v", got, want)
	}
	for _, r := range got {
		if !strings.Contains(r.authorization, ghEnvValue) {
			t.Errorf("got request %+v, want GH_TOKEN in Authorization", r)
		}
	}
	for _, token := range []string{ghEnvValue, githubEnvValue} {
		if strings.Contains(stdout+stderr, token) {
			t.Errorf("got output containing token %q", token)
		}
	}
}

//trace:checks FR-009
//trace:checks INV-003
func TestApply_NotApproved(t *testing.T) {
	cases := []struct {
		name       string
		stdin      io.Reader
		terminal   bool
		wantPrompt string
	}{
		{name: "y from standard input that is not a terminal does not update", stdin: strings.NewReader("y\n"), terminal: false, wantPrompt: ""},
		{name: "n from the terminal does not update", stdin: strings.NewReader("n\n"), terminal: true, wantPrompt: approvalPrompt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			writeConfig(t, workDir, "reco.yaml", declaredRepository)
			server, opts := newGitHubServer(t, http.StatusOK, mismatchedRepositoryBody)

			env := Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}), Stdin: tc.stdin, StdinIsTerminal: tc.terminal}
			code, stdout, stderr := runExecute(t, env, opts, "apply")
			if code != 1 {
				t.Errorf("got exit code %d, want 1", code)
			}
			if !strings.Contains(stdout, "general.allow_auto_merge: false -> true") {
				t.Errorf("got stdout %q, want the change plan", stdout)
			}
			log, ok := strings.CutPrefix(stderr, tc.wantPrompt)
			if !ok {
				t.Errorf("got stderr %q, want it to start with %q", stderr, tc.wantPrompt)
			}
			records := errorRecords(t, log)
			if len(records) != 1 || records[0]["msg"] != "command failed" {
				t.Errorf("got error records %v, want one command failed", records)
			}
			if got := methodsOf(server.receivedRequests()); slices.Contains(got, http.MethodPatch) {
				t.Errorf("got requests %v, want no PATCH", got)
			}
		})
	}
}

//trace:checks INV-003
func TestApply_CanceledWhileWaitingForApproval(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredRepository)
	server, opts := newGitHubServer(t, http.StatusOK, mismatchedRepositoryBody)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	t.Cleanup(func() { close(done) })

	var stdout, stderr bytes.Buffer
	env := Environment{
		Stdout: &stdout, Stderr: &stderr, WorkDir: workDir,
		Getenv:          fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}),
		Stdin:           blockingReader{cancel: cancel, done: done},
		StdinIsTerminal: true,
	}
	if code := execute(ctx, []string{"apply"}, env, opts...); code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if got := methodsOf(server.receivedRequests()); slices.Contains(got, http.MethodPatch) {
		t.Errorf("got requests %v, want no PATCH", got)
	}
}

//trace:checks INV-002
//trace:checks FR-003
func TestApply_FailsBeforeApproval(t *testing.T) {
	cases := []struct {
		name   string
		args   []string
		getenv map[string]string
		status int
	}{
		{name: "empty --config", args: []string{"apply", "--config", ""}, getenv: map[string]string{"GH_TOKEN": ghEnvValue}, status: http.StatusOK},
		{name: "positional argument", args: []string{"apply", "extra"}, getenv: map[string]string{"GH_TOKEN": ghEnvValue}, status: http.StatusOK},
		{name: "token missing", args: []string{"apply"}, getenv: map[string]string{}, status: http.StatusOK},
		{name: "observation failure", args: []string{"apply"}, getenv: map[string]string{"GH_TOKEN": ghEnvValue}, status: http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name+" fails without asking for approval", func(t *testing.T) {
			workDir := t.TempDir()
			writeConfig(t, workDir, "reco.yaml", declaredRepository)
			server, opts := newGitHubServer(t, tc.status, mismatchedRepositoryBody)

			env := Environment{WorkDir: workDir, Getenv: fakeGetenv(tc.getenv), Stdin: unreadableReader{t}, StdinIsTerminal: true}
			code, stdout, stderr := runExecute(t, env, opts, tc.args...)
			if code != 1 {
				t.Errorf("got exit code %d, want 1", code)
			}
			if strings.Contains(stdout+stderr, approvalPrompt) {
				t.Errorf("got output with the approval prompt, want none")
			}
			if records := errorRecords(t, stderr); len(records) != 1 || records[0]["msg"] != "command failed" {
				t.Errorf("got error records %v, want one command failed", records)
			}
			if got := methodsOf(server.receivedRequests()); slices.Contains(got, http.MethodPatch) {
				t.Errorf("got requests %v, want no PATCH", got)
			}
		})
	}
}
