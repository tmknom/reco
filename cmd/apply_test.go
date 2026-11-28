package cmd

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tmknom/reco/internal/githubtest"
)

// appliedRepositoryBody は、宣言の期待値 true に更新されたあとの観測結果。
var appliedRepositoryBody = githubtest.RepositoryBody{ID: 1, AllowAutoMerge: true, Admin: true}.JSON()

// streamWrite は、taggedWriter が記録した一回の書き込み。
type streamWrite struct {
	stream, text string
}

//trace:checks FR-008 FR-002
func TestApply_NoChanges(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "sub/custom.yaml", declaredTwoRepositories)
	server, opts := newGitHubServer(t, githubtest.OK(appliedRepositoryBody))

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
	got := server.Requests()
	if len(got) != 1 || got[0].Method != http.MethodGet || got[0].Path != "/repos/owner/second" {
		t.Errorf("got requests %+v, want one GET /repos/owner/second", got)
	}
}

//trace:checks FR-009 FR-011 QR-001
func TestApply_Approved(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredRepository)
	server, opts := newGitHubServer(t, githubtest.Sequence(t,
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: appliedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: appliedRepositoryBody},
	))

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
	got := server.Requests()
	want := []string{http.MethodGet, http.MethodGet, http.MethodPatch, http.MethodGet}
	if !slices.Equal(server.Methods(), want) {
		t.Errorf("got requests %+v, want methods %v", got, want)
	}
	for _, r := range got {
		if !strings.Contains(r.Authorization, ghEnvValue) {
			t.Errorf("got request %+v, want GH_TOKEN in Authorization", r)
		}
	}
	for _, token := range []string{ghEnvValue, githubEnvValue} {
		if strings.Contains(stdout+stderr, token) {
			t.Errorf("got output containing token %q", token)
		}
	}
}

// TestApply_ApprovedWithWorkflowPermissions は、PATCH と PUT が混ざる承認済みの実行の配線を確かめる。
//
//trace:checks FR-011
func TestApply_ApprovedWithWorkflowPermissions(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
    actions:
      workflow:
        default_workflow_permissions: read
`)
	writePermissionsBody := githubtest.WorkflowPermissionsBody{DefaultWorkflowPermissions: "write"}.JSON()
	server, opts := newGitHubServer(t, githubtest.Sequence(t,
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: writePermissionsBody},
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: appliedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: appliedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: appliedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: writePermissionsBody},
		githubtest.Response{Status: http.StatusNoContent, Body: ""},
		githubtest.Response{Status: http.StatusOK, Body: appliedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: workflowPermissionsBody},
	))

	env := Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}), Stdin: strings.NewReader("y\n"), StdinIsTerminal: true}
	code, stdout, stderr := runExecute(t, env, opts, "apply")
	if code != 0 {
		t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "owner/repo: updated") {
		t.Errorf("got stdout %q, want owner/repo: updated", stdout)
	}
	repo, workflow := "/repos/owner/repo", "/repos/owner/repo/actions/permissions/workflow"
	want := []githubtest.Request{
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodGet, Path: workflow},
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodPatch, Path: repo}, {Method: http.MethodGet, Path: repo},
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodGet, Path: workflow}, {Method: http.MethodPut, Path: workflow},
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodGet, Path: workflow},
	}
	got := server.Requests()
	if len(got) != len(want) {
		t.Fatalf("got requests %+v, want %d requests", got, len(want))
	}
	for i, r := range got {
		if r.Method != want[i].Method || r.Path != want[i].Path {
			t.Errorf("got request %d %s %s, want %s %s", i, r.Method, r.Path, want[i].Method, want[i].Path)
		}
		if !strings.Contains(r.Authorization, ghEnvValue) {
			t.Errorf("got request %d %+v, want GH_TOKEN in Authorization", i, r)
		}
	}
}

// TestApply_ApprovedWithActionsPermissions は、Actions permissions の PUT を含む承認済みの実行の配線を確かめる。
//
//trace:checks FR-011
func TestApply_ApprovedWithActionsPermissions(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", `repositories:
  - name: owner/repo
    actions:
      permissions:
        allowed_actions: local_only
`)
	allBody := githubtest.ActionsPermissionsBody{AllowedActions: "all"}.JSON()
	localOnlyBody := githubtest.ActionsPermissionsBody{AllowedActions: "local_only"}.JSON()
	server, opts := newGitHubServer(t, githubtest.Sequence(t,
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: allBody},
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: allBody},
		githubtest.Response{Status: http.StatusNoContent, Body: ""},
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: localOnlyBody},
	))

	env := Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}), Stdin: strings.NewReader("y\n"), StdinIsTerminal: true}
	code, stdout, stderr := runExecute(t, env, opts, "apply")
	if code != 0 {
		t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "owner/repo: updated") {
		t.Errorf("got stdout %q, want owner/repo: updated", stdout)
	}
	repo, permissions := "/repos/owner/repo", "/repos/owner/repo/actions/permissions"
	want := []githubtest.Request{
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodGet, Path: permissions},
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodGet, Path: permissions}, {Method: http.MethodPut, Path: permissions},
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodGet, Path: permissions},
	}
	got := server.Requests()
	if len(got) != len(want) {
		t.Fatalf("got requests %+v, want %d requests", got, len(want))
	}
	for i, r := range got {
		if r.Method != want[i].Method || r.Path != want[i].Path {
			t.Errorf("got request %d %s %s, want %s %s", i, r.Method, r.Path, want[i].Method, want[i].Path)
		}
		if !strings.Contains(r.Authorization, ghEnvValue) {
			t.Errorf("got request %d %+v, want GH_TOKEN in Authorization", i, r)
		}
	}
}

// TestApply_ApprovedWithTwoUpdates は、Actions permissions と Selected actions の二つの PUT を含む承認済みの実行の配線を確かめる。
// 応答の数は観測し直しを求めない数だけ渡すため、待機が更新要求の間隔の一回だけであることは、待機が apply に届いていることを示す。
//
//trace:checks FR-011
func TestApply_ApprovedWithTwoUpdates(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", `repositories:
  - name: owner/repo
    actions:
      permissions:
        allowed_actions: selected
        selected_actions:
          github_owned_allowed: true
          verified_allowed: false
          patterns_allowed: ["example/*"]
`)
	allBody := githubtest.ActionsPermissionsBody{AllowedActions: "all"}.JSON()
	setBackBody := githubtest.SelectedActionsBody{GitHubOwnedAllowed: true}.JSON()
	server, opts := newGitHubServer(t, githubtest.Sequence(t,
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody}, githubtest.Response{Status: http.StatusOK, Body: allBody},
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody}, githubtest.Response{Status: http.StatusOK, Body: allBody}, githubtest.Response{Status: http.StatusNoContent, Body: ""},
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody}, githubtest.Response{Status: http.StatusOK, Body: actionsPermissionsBody}, githubtest.Response{Status: http.StatusOK, Body: setBackBody},
		githubtest.Response{Status: http.StatusNoContent, Body: ""},
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody}, githubtest.Response{Status: http.StatusOK, Body: actionsPermissionsBody}, githubtest.Response{Status: http.StatusOK, Body: selectedActionsBody},
	))
	var delays []time.Duration
	after := func(d time.Duration) <-chan time.Time {
		delays = append(delays, d)
		return elapsedAtOnce(d)
	}

	var stdout, stderr bytes.Buffer
	env := Environment{
		Stdout: &stdout, Stderr: &stderr, WorkDir: workDir,
		Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}), Stdin: strings.NewReader("y\n"), StdinIsTerminal: true,
	}
	if code := execute(context.Background(), []string{"apply"}, env, dependencies{after: after, githubOptions: opts}); code != 0 {
		t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "owner/repo: updated") {
		t.Errorf("got stdout %q, want owner/repo: updated", stdout.String())
	}
	if want := []time.Duration{time.Second}; !slices.Equal(delays, want) {
		t.Errorf("got delays %v, want %v", delays, want)
	}
	repo, permissions, selected := "/repos/owner/repo", "/repos/owner/repo/actions/permissions", "/repos/owner/repo/actions/permissions/selected-actions"
	want := []githubtest.Request{
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodGet, Path: permissions},
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodGet, Path: permissions}, {Method: http.MethodPut, Path: permissions},
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodGet, Path: permissions}, {Method: http.MethodGet, Path: selected}, {Method: http.MethodPut, Path: selected},
		{Method: http.MethodGet, Path: repo}, {Method: http.MethodGet, Path: permissions}, {Method: http.MethodGet, Path: selected},
	}
	got := server.Requests()
	if len(got) != len(want) {
		t.Fatalf("got requests %+v, want %d requests", got, len(want))
	}
	for i, r := range got {
		if r.Method != want[i].Method || r.Path != want[i].Path {
			t.Errorf("got request %d %s %s, want %s %s", i, r.Method, r.Path, want[i].Method, want[i].Path)
		}
		if !strings.Contains(r.Authorization, ghEnvValue) {
			t.Errorf("got request %d %+v, want GH_TOKEN in Authorization", i, r)
		}
	}
}

//trace:checks FR-009 INV-003
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
			server, opts := newGitHubServer(t, githubtest.OK(mismatchedRepositoryBody))

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
			if got := server.Methods(); slices.Contains(got, http.MethodPatch) {
				t.Errorf("got requests %v, want no PATCH", got)
			}
		})
	}
}

// taggedWriter は、stdout と stderr に渡し、ストリームをまたいだ書き込みの順を一つの並びに残す。
type taggedWriter struct {
	stream string
	writes *[]streamWrite
}

func (w taggedWriter) Write(p []byte) (int, error) {
	*w.writes = append(*w.writes, streamWrite{stream: w.stream, text: string(p)})
	return len(p), nil
}

// TestApply_SideEffectOnStdoutBeforeApprovalPrompt は、副作用を伴う変更計画で、Side effect: を stdout に、
// 承認プロンプトを stderr に、この順で書くことを確かめる。
//
//trace:checks FR-009 INV-001 INV-003
func TestApply_SideEffectOnStdoutBeforeApprovalPrompt(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", `repositories:
  - name: owner/repo
    actions:
      permissions:
        allowed_actions: all
`)
	server, opts := newGitHubServer(t, githubtest.Routes(t, map[string]githubtest.Response{
		"GET /repos/owner/repo":                                      {Status: http.StatusOK, Body: repositoryBody},
		"GET /repos/owner/repo/actions/permissions":                  {Status: http.StatusOK, Body: actionsPermissionsBody},
		"GET /repos/owner/repo/actions/permissions/selected-actions": {Status: http.StatusOK, Body: selectedActionsBody},
	}))
	var writes []streamWrite
	env := Environment{
		WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}), Stdin: strings.NewReader("n\n"), StdinIsTerminal: true,
		Stdout: taggedWriter{stream: "stdout", writes: &writes}, Stderr: taggedWriter{stream: "stderr", writes: &writes},
	}

	if code := execute(context.Background(), []string{"apply"}, env, dependencies{after: elapsedAtOnce, githubOptions: opts}); code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	sideEffect := slices.IndexFunc(writes, func(w streamWrite) bool { return w.stream == "stdout" && strings.Contains(w.text, "Side effect:") })
	prompt := slices.IndexFunc(writes, func(w streamWrite) bool { return w.stream == "stderr" && strings.Contains(w.text, approvalPrompt) })
	if sideEffect < 0 || prompt < 0 || sideEffect > prompt {
		t.Errorf("got writes %+v, want Side effect: on stdout before the approval prompt on stderr", writes)
	}
	if got := server.Methods(); slices.Contains(got, http.MethodPut) {
		t.Errorf("got requests %v, want no PUT", got)
	}
}

// TestApply_PlannedChangeNoteOnStdoutBeforeApprovalPrompt は、変更予定の注記を伴う変更計画で、Note: を stdout に、
// 承認プロンプトを stderr に、この順で書き、承認が拒否されたら更新要求を送らないことを確かめる。
//
//trace:checks FR-009 INV-003
func TestApply_PlannedChangeNoteOnStdoutBeforeApprovalPrompt(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", `repositories:
  - name: owner/repo
    actions:
      fork_pr_workflows_private_repos:
        run_workflows_from_fork_pull_requests: true
        send_write_tokens_to_workflows: false
        send_secrets_and_variables: true
        require_approval_for_fork_pr_workflows: true
`)
	server, opts := newGitHubServer(t, githubtest.Routes(t, map[string]githubtest.Response{
		"GET /repos/owner/repo": {Status: http.StatusOK, Body: githubtest.RepositoryBody{ID: 1, Private: true}.JSON()},
		"GET /repos/owner/repo/actions/permissions/fork-pr-workflows-private-repos": {Status: http.StatusOK, Body: githubtest.PrivateForkPullRequestWorkflowsBody{}.JSON()},
	}))
	var writes []streamWrite
	env := Environment{
		WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}), Stdin: strings.NewReader("n\n"), StdinIsTerminal: true,
		Stdout: taggedWriter{stream: "stdout", writes: &writes}, Stderr: taggedWriter{stream: "stderr", writes: &writes},
	}

	if code := execute(context.Background(), []string{"apply"}, env, dependencies{after: elapsedAtOnce, githubOptions: opts}); code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	note := slices.IndexFunc(writes, func(w streamWrite) bool { return w.stream == "stdout" && strings.Contains(w.text, "Note:") })
	prompt := slices.IndexFunc(writes, func(w streamWrite) bool { return w.stream == "stderr" && strings.Contains(w.text, approvalPrompt) })
	if note < 0 || prompt < 0 || note > prompt {
		t.Errorf("got writes %+v, want Note: on stdout before the approval prompt on stderr", writes)
	}
	if got := server.Methods(); slices.Contains(got, http.MethodPut) {
		t.Errorf("got requests %v, want no PUT", got)
	}
}

//trace:checks INV-003
func TestApply_CanceledWhileWaitingForApproval(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredRepository)
	server, opts := newGitHubServer(t, githubtest.OK(mismatchedRepositoryBody))
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
	if code := execute(ctx, []string{"apply"}, env, dependencies{after: elapsedAtOnce, githubOptions: opts}); code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if got := server.Methods(); slices.Contains(got, http.MethodPatch) {
		t.Errorf("got requests %v, want no PATCH", got)
	}
}

// TestApply_ApprovedMultipleRepositories は、--repo を省略して2件を選んだ apply が、承認を一度だけ求め、
// 宣言順に一つずつ更新し、対象リポジトリのあいだで更新要求の間隔を待つ配線を確かめる。
// 応答の数は観測し直しを求めない数だけ渡すため、待機が一回であることは、間隔が対象リポジトリをまたいで apply に届いていることを示す。
//
//trace:checks FR-009 FR-011 INV-003
func TestApply_ApprovedMultipleRepositories(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredTwoRepositories)
	mismatchedSecond := githubtest.RepositoryBody{ID: 2, Admin: true}.JSON()
	appliedSecond := githubtest.RepositoryBody{ID: 2, AllowAutoMerge: true, Admin: true}.JSON()
	server, opts := newGitHubServer(t, githubtest.Sequence(t,
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody}, githubtest.Response{Status: http.StatusOK, Body: mismatchedSecond},
		githubtest.Response{Status: http.StatusOK, Body: mismatchedRepositoryBody}, githubtest.Response{Status: http.StatusOK, Body: appliedRepositoryBody}, githubtest.Response{Status: http.StatusOK, Body: appliedRepositoryBody},
		githubtest.Response{Status: http.StatusOK, Body: mismatchedSecond}, githubtest.Response{Status: http.StatusOK, Body: appliedSecond}, githubtest.Response{Status: http.StatusOK, Body: appliedSecond},
	))
	var delays []time.Duration
	after := func(d time.Duration) <-chan time.Time {
		delays = append(delays, d)
		return elapsedAtOnce(d)
	}

	var stdout, stderr bytes.Buffer
	env := Environment{
		Stdout: &stdout, Stderr: &stderr, WorkDir: workDir,
		Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}), Stdin: strings.NewReader("y\n"), StdinIsTerminal: true,
	}
	if code := execute(context.Background(), []string{"apply"}, env, dependencies{after: after, githubOptions: opts}); code != 0 {
		t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr.String())
	}
	if stderr.String() != approvalPrompt {
		t.Errorf("got stderr %q, want the approval prompt once", stderr.String())
	}
	for _, want := range []string{"owner/repo: updated\n  owner/second: updated\n", "Repositories updated: 2"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("got stdout %q, want it to contain %q", stdout.String(), want)
		}
	}
	if want := []time.Duration{time.Second}; !slices.Equal(delays, want) {
		t.Errorf("got delays %v, want %v", delays, want)
	}
	first, second := "/repos/owner/repo", "/repos/owner/second"
	want := []githubtest.Request{
		{Method: http.MethodGet, Path: first}, {Method: http.MethodGet, Path: second},
		{Method: http.MethodGet, Path: first}, {Method: http.MethodPatch, Path: first}, {Method: http.MethodGet, Path: first},
		{Method: http.MethodGet, Path: second}, {Method: http.MethodPatch, Path: second}, {Method: http.MethodGet, Path: second},
	}
	got := server.Requests()
	if len(got) != len(want) {
		t.Fatalf("got requests %+v, want %d requests", got, len(want))
	}
	for i, r := range got {
		if r.Method != want[i].Method || r.Path != want[i].Path {
			t.Errorf("got request %d %s %s, want %s %s", i, r.Method, r.Path, want[i].Method, want[i].Path)
		}
		if !strings.Contains(r.Authorization, ghEnvValue) {
			t.Errorf("got request %d %+v, want GH_TOKEN in Authorization", i, r)
		}
	}
}

// 2件目の観測が失敗すると、変更計画を示さず、承認を求めず、更新要求を送らない。
//
//trace:checks INV-002 FR-008
func TestApply_SecondObservationFailureFailsBeforeApproval(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredTwoRepositories)
	server, opts := newGitHubServer(t, githubtest.Routes(t, map[string]githubtest.Response{
		"GET /repos/owner/repo":   {Status: http.StatusOK, Body: mismatchedRepositoryBody},
		"GET /repos/owner/second": {Status: http.StatusInternalServerError, Body: `{"message": "Server Error"}`},
	}))

	env := Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}), Stdin: unreadableReader{t}, StdinIsTerminal: true}
	code, stdout, stderr := runExecute(t, env, opts, "apply")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("got stdout %q, want no change plan", stdout)
	}
	if strings.Contains(stderr, approvalPrompt) {
		t.Errorf("got stderr with the approval prompt, want none")
	}
	if records := errorRecords(t, stderr); len(records) != 1 || records[0]["msg"] != "command failed" {
		t.Errorf("got ERROR records %v, want one command failed record", records)
	}
	if got := server.Methods(); !slices.Equal(got, []string{http.MethodGet, http.MethodGet}) {
		t.Errorf("got requests %v, want the two observation GETs only", got)
	}
}

// --repo を省略して選んだ全リポジトリが変更不要なら、承認を求めずに正常終了する。
//
//trace:checks FR-008
func TestApply_NoChangesForAllRepositories(t *testing.T) {
	workDir := t.TempDir()
	writeConfig(t, workDir, "reco.yaml", declaredTwoRepositories)
	server, opts := newGitHubServer(t, githubtest.OK(appliedRepositoryBody))

	env := Environment{WorkDir: workDir, Getenv: fakeGetenv(map[string]string{"GH_TOKEN": ghEnvValue}), Stdin: unreadableReader{t}, StdinIsTerminal: true}
	code, stdout, stderr := runExecute(t, env, opts, "apply")
	if code != 0 {
		t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
	}
	if want := "No changes.\nRepositories checked: 2\n"; stdout != want {
		t.Errorf("got stdout %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Errorf("got stderr %q, want no approval prompt", stderr)
	}
	if got := server.Methods(); !slices.Equal(got, []string{http.MethodGet, http.MethodGet}) {
		t.Errorf("got requests %v, want the two observation GETs only", got)
	}
}

//trace:checks INV-002 FR-003
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
			server, opts := newGitHubServer(t, githubtest.Response{Status: tc.status, Body: mismatchedRepositoryBody})

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
			if got := server.Methods(); slices.Contains(got, http.MethodPatch) {
				t.Errorf("got requests %v, want no PATCH", got)
			}
		})
	}
}
