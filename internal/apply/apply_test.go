package apply

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

// failAfterWriter は、n 回の書き込みに成功したあとの書き込みを失敗させる。
type failAfterWriter struct {
	n   int
	buf bytes.Buffer
}

func (w *failAfterWriter) Write(p []byte) (int, error) {
	if w.n == 0 {
		return 0, errWrite
	}
	w.n--
	return w.buf.Write(p)
}

// response は stub が順に返す応答。disconnect が真なら本文を返さずに接続を切る。
type response struct {
	status     int
	body       string
	location   string
	disconnect bool
}

// recorder は stub が受けた要求の method、本文、Authorization を記録する。
type recorder struct {
	mu      sync.Mutex
	methods []string
	// bodies は要求ごとの JSON 本文。本文のない要求では nil。
	bodies        []map[string]any
	authorization string
}

func (r *recorder) Methods() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.methods)
}

func (r *recorder) Bodies() []map[string]any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.bodies)
}

func (r *recorder) Authorization() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.authorization
}

// newSequenceServer は、要求ごとに responses を順に返す。足りなければ 500 を返す。
func newSequenceServer(t *testing.T, responses ...response) (*url.URL, *recorder) {
	t.Helper()
	rec := &recorder{}
	next := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("decode request body %q: %v", raw, err)
			}
		}
		rec.mu.Lock()
		rec.methods = append(rec.methods, r.Method)
		rec.bodies = append(rec.bodies, body)
		rec.authorization = r.Header.Get("Authorization")
		resp := response{status: http.StatusInternalServerError}
		if next < len(responses) {
			resp = responses[next]
			next++
		}
		rec.mu.Unlock()

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
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return u, rec
}

func repositoryJSON(id int64, allowAutoMerge, deleteBranchOnMerge bool) string {
	return fmt.Sprintf(`{"id": %d, "allow_auto_merge": %t, "delete_branch_on_merge": %t}`, id, allowAutoMerge, deleteBranchOnMerge)
}

func okRepository(id int64, allowAutoMerge, deleteBranchOnMerge bool) response {
	return response{status: http.StatusOK, body: repositoryJSON(id, allowAutoMerge, deleteBranchOnMerge)}
}

func okWorkflowPermissions(defaultWorkflowPermissions string, canApprovePullRequestReviews bool) response {
	return response{status: http.StatusOK, body: fmt.Sprintf(
		`{"default_workflow_permissions": %q, "can_approve_pull_request_reviews": %t}`, defaultWorkflowPermissions, canApprovePullRequestReviews)}
}

func newClient(baseURL *url.URL) *github.Client {
	return github.New("test-token", github.WithBaseURL(baseURL))
}

// allowAutoMergeChange は、allow_auto_merge を false から true にする論理的な設定変更。
func allowAutoMergeChange() changeplan.LogicalChange {
	return changeplan.LogicalChange{Key: setting.AllowAutoMerge, Observed: changeplan.BoolValue(false), Desired: changeplan.BoolValue(true)}
}

// allowAutoMergeMismatch は、allowAutoMergeChange を導く適用可能な変更予定。
func allowAutoMergeMismatch() changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Key:           setting.AllowAutoMerge,
		Observed:      changeplan.BoolValue(false),
		Desired:       changeplan.BoolValue(true),
		Result:        changeplan.Mismatch,
		Applicability: changeplan.Applicable,
	}
}

// deleteBranchOnMergeChange は、delete_branch_on_merge を false から true にする論理的な設定変更。
func deleteBranchOnMergeChange() changeplan.LogicalChange {
	return changeplan.LogicalChange{Key: setting.DeleteBranchOnMerge, Observed: changeplan.BoolValue(false), Desired: changeplan.BoolValue(true)}
}

// deleteBranchOnMergeMismatch は、deleteBranchOnMergeChange を導く適用可能な変更予定。
func deleteBranchOnMergeMismatch() changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Key:           setting.DeleteBranchOnMerge,
		Observed:      changeplan.BoolValue(false),
		Desired:       changeplan.BoolValue(true),
		Result:        changeplan.Mismatch,
		Applicability: changeplan.Applicable,
	}
}

// defaultWorkflowPermissionsChange は、default_workflow_permissions を write から read にする論理的な設定変更。
func defaultWorkflowPermissionsChange() changeplan.LogicalChange {
	return changeplan.LogicalChange{
		Key:      setting.DefaultWorkflowPermissions,
		Observed: changeplan.PermissionLevelValue(setting.PermissionWrite),
		Desired:  changeplan.PermissionLevelValue(setting.PermissionRead),
	}
}

// canApprovePullRequestReviewsChange は、can_approve_pull_request_reviews を true から false にする論理的な設定変更。
func canApprovePullRequestReviewsChange() changeplan.LogicalChange {
	return changeplan.LogicalChange{Key: setting.CanApprovePullRequestReviews, Observed: changeplan.BoolValue(true), Desired: changeplan.BoolValue(false)}
}

func newPlan(t *testing.T, settings ...changeplan.SettingPlan) changeplan.Plan {
	t.Helper()
	return changeplan.Plan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
}

// unknownKey は、実行できる更新操作を持たない設定。
const unknownKey setting.Key = "general.unknown"

// unknownMismatch は、実行できる更新操作を持たない設定の適用可能な変更予定。
func unknownMismatch() changeplan.SettingPlan {
	return changeplan.SettingPlan{Key: unknownKey, Observed: changeplan.BoolValue(false), Desired: changeplan.BoolValue(true), Result: changeplan.Mismatch, Applicability: changeplan.Applicable}
}

// TestExecutableChanges は、承認を求める前に、すべての論理的な設定変更が実行できる更新操作を持つことを確かめる規則を確かめる。
//
//trace:checks INV-002
//trace:checks FR-009
func TestExecutableChanges(t *testing.T) {
	match := changeplan.SettingPlan{Key: setting.DeleteBranchOnMerge, Observed: changeplan.BoolValue(true), Desired: changeplan.BoolValue(true), Result: changeplan.Match}

	t.Run("all matches have no change", func(t *testing.T) {
		got, err := executableChanges(newPlan(t, match))
		if err != nil || len(got) != 0 {
			t.Errorf("executableChanges() = %v, %v, want no change without error", got, err)
		}
	})

	t.Run("logical change without an update operation after an executable one is an error naming the setting", func(t *testing.T) {
		got, err := executableChanges(newPlan(t, allowAutoMergeMismatch(), unknownMismatch()))
		if err == nil || !strings.Contains(err.Error(), string(unknownKey)) {
			t.Errorf("executableChanges() = %v, %v, want an error containing %q", got, err, unknownKey)
		}
	})
}

//trace:checks FR-009
func TestWriteChangePlan(t *testing.T) {
	target := mustReference(t, "owner/repo")

	t.Run("changes are written in the given order with the configuration and the repository", func(t *testing.T) {
		var out bytes.Buffer
		changes := []changeplan.LogicalChange{deleteBranchOnMergeChange(), allowAutoMergeChange()}
		if err := writeChangePlan(&out, "./reco.yaml", target, changes); err != nil {
			t.Fatalf("writeChangePlan() error = %v", err)
		}
		want := "Configuration: ./reco.yaml\nRepository: owner/repo\n" +
			"  general.delete_branch_on_merge: false -> true\n  general.allow_auto_merge: false -> true\n\n"
		if out.String() != want {
			t.Errorf("writeChangePlan() wrote %q, want %q", out.String(), want)
		}
	})

	t.Run("write failure is returned", func(t *testing.T) {
		err := writeChangePlan(failingWriter{}, "./reco.yaml", target, []changeplan.LogicalChange{allowAutoMergeChange()})
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangePlan() error = %v, want %v", err, errWrite)
		}
	})
}

//trace:checks INV-002
//trace:checks INV-003
//trace:checks INV-005
//trace:checks FR-008
//trace:checks FR-009
//trace:checks FR-013
func TestApplyPlan(t *testing.T) {
	approve := func(calls *int) func(context.Context) error {
		return func(context.Context) error {
			*calls++
			return nil
		}
	}

	t.Run("no change does not ask for approval or send a request", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0
		match := changeplan.SettingPlan{Key: setting.DeleteBranchOnMerge, Observed: changeplan.BoolValue(true), Desired: changeplan.BoolValue(true), Result: changeplan.Match}

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls)}, newClient(baseURL), newPlan(t, match))

		if err != nil {
			t.Fatalf("applyPlan() error = %v", err)
		}
		if want := "No changes.\nRepositories checked: 1\n"; out.String() != want {
			t.Errorf("got Out %q, want %q", out.String(), want)
		}
		if calls != 0 || len(rec.Methods()) != 0 {
			t.Errorf("approve calls = %d, requests = %v, want none", calls, rec.Methods())
		}
	})

	t.Run("unappliable plan returns an error without approval or output", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0
		notApplicable := changeplan.SettingPlan{Key: setting.AllowAutoMerge, Result: changeplan.NotComparable, Applicability: changeplan.NotApplicable}

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls)}, newClient(baseURL), newPlan(t, notApplicable))

		if err == nil {
			t.Error("applyPlan() error = nil, want an error")
		}
		if calls != 0 || out.Len() != 0 || len(rec.Methods()) != 0 {
			t.Errorf("approve calls = %d, Out = %q, requests = %v, want none", calls, out.String(), rec.Methods())
		}
	})

	t.Run("two changes are shown before approval and both are applied", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t,
			okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
			okRepository(1, false, true), okRepository(1, true, true), okRepository(1, true, true))
		var out bytes.Buffer
		var shown string
		cfg := Config{Out: &out, Approve: func(context.Context) error {
			shown = out.String()
			return nil
		}}

		err := applyPlan(context.Background(), cfg, newClient(baseURL), newPlan(t, deleteBranchOnMergeMismatch(), allowAutoMergeMismatch()))

		if err != nil {
			t.Fatalf("applyPlan() error = %v", err)
		}
		for _, want := range []string{"general.delete_branch_on_merge: false -> true", "general.allow_auto_merge: false -> true"} {
			if !strings.Contains(shown, want) {
				t.Errorf("Out at approval = %q, want it to contain %q", shown, want)
			}
		}
		if want := []string{"GET", "PATCH", "GET", "GET", "PATCH", "GET"}; !slices.Equal(rec.Methods(), want) {
			t.Errorf("requests = %v, want %v", rec.Methods(), want)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("got Out %q, want it to contain %q", out.String(), "owner/repo: updated")
		}
	})

	t.Run("failed second change returns an error without reporting the repository updated", func(t *testing.T) {
		baseURL, _ := newSequenceServer(t,
			okRepository(1, false, false), okRepository(1, false, true), okRepository(1, false, true),
			okRepository(1, false, true), okRepository(1, true, true), okRepository(1, false, true))
		var out bytes.Buffer
		calls := 0

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls)}, newClient(baseURL), newPlan(t, deleteBranchOnMergeMismatch(), allowAutoMergeMismatch()))

		if err == nil {
			t.Error("applyPlan() error = nil, want an error")
		}
		if strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("got Out %q, want no repository updated line", out.String())
		}
	})

	t.Run("approval error cancels without a request", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t)
		var out bytes.Buffer
		errDeclined := errors.New("declined")
		cfg := Config{Out: &out, Approve: func(context.Context) error { return errDeclined }}

		err := applyPlan(context.Background(), cfg, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, errDeclined) {
			t.Fatalf("applyPlan() error = %v, want %v", err, errDeclined)
		}
		for _, want := range []string{"apply cancelled", "no changes made"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("applyPlan() error = %q, want it to contain %q", err, want)
			}
		}
		if len(rec.Methods()) != 0 {
			t.Errorf("requests = %v, want none", rec.Methods())
		}
		if strings.Contains(out.String(), "Applying") {
			t.Errorf("got Out %q, want no Applying line", out.String())
		}
	})

	t.Run("context canceled during approval does not start the change", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t)
		var out bytes.Buffer
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cfg := Config{Out: &out, Approve: func(context.Context) error {
			cancel()
			return nil
		}}

		err := applyPlan(ctx, cfg, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, context.Canceled) {
			t.Errorf("applyPlan() error = %v, want %v", err, context.Canceled)
		}
		if len(rec.Methods()) != 0 {
			t.Errorf("requests = %v, want none", rec.Methods())
		}
		if strings.Contains(out.String(), "Applying") {
			t.Errorf("got Out %q, want no Applying line", out.String())
		}
	})

	t.Run("approved change is applied and reported as complete", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t, okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false))
		var out bytes.Buffer
		calls := 0

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls)}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if err != nil {
			t.Fatalf("applyPlan() error = %v", err)
		}
		if want := []string{"GET", "PATCH", "GET"}; !slices.Equal(rec.Methods(), want) {
			t.Errorf("requests = %v, want %v", rec.Methods(), want)
		}
		for _, want := range []string{"Applying changes...\n", "owner/repo: updated", "Apply complete."} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("got Out %q, want it to contain %q", out.String(), want)
			}
		}
	})

	t.Run("unknown update outcome fails after a single update request", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t,
			okRepository(1, false, false),
			response{status: http.StatusTemporaryRedirect, location: "https://example.com/"},
			okRepository(1, true, false))
		var out bytes.Buffer
		calls := 0

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls)}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, github.ErrRedirect) {
			t.Errorf("applyPlan() error = %v, want %v", err, github.ErrRedirect)
		}
		if want := []string{"GET", "PATCH", "GET"}; !slices.Equal(rec.Methods(), want) {
			t.Errorf("requests = %v, want %v", rec.Methods(), want)
		}
		if strings.Contains(out.String(), ": updated") {
			t.Errorf("got Out %q, want no updated line", out.String())
		}
	})

	t.Run("changed repository ID is reported as not attempted without an update request", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t, okRepository(2, false, false))
		var out bytes.Buffer
		calls := 0

		err := applyPlan(context.Background(), Config{Out: &out, Approve: approve(&calls)}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if err == nil {
			t.Error("applyPlan() error = nil, want an error")
		}
		if want := []string{"GET"}; !slices.Equal(rec.Methods(), want) {
			t.Errorf("requests = %v, want %v", rec.Methods(), want)
		}
		if !strings.Contains(out.String(), "not attempted") {
			t.Errorf("got Out %q, want it to contain %q", out.String(), "not attempted")
		}
	})

	t.Run("change plan write failure skips approval", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t)
		calls := 0

		err := applyPlan(context.Background(), Config{Out: failingWriter{}, Approve: approve(&calls)}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, errWrite) {
			t.Errorf("applyPlan() error = %v, want %v", err, errWrite)
		}
		if calls != 0 || len(rec.Methods()) != 0 {
			t.Errorf("approve calls = %d, requests = %v, want none", calls, rec.Methods())
		}
	})

	t.Run("applying line write failure skips the update", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t)
		calls := 0
		out := &failAfterWriter{n: 1}

		err := applyPlan(context.Background(), Config{Out: out, Approve: approve(&calls)}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, errWrite) {
			t.Errorf("applyPlan() error = %v, want %v", err, errWrite)
		}
		if len(rec.Methods()) != 0 {
			t.Errorf("requests = %v, want none", rec.Methods())
		}
	})

	t.Run("change report write failure is returned after the update", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t, okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false))
		calls := 0
		out := &failAfterWriter{n: 2}

		err := applyPlan(context.Background(), Config{Out: out, Approve: approve(&calls)}, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if !errors.Is(err, errWrite) {
			t.Errorf("applyPlan() error = %v, want %v", err, errWrite)
		}
		if want := []string{"GET", "PATCH", "GET"}; !slices.Equal(rec.Methods(), want) {
			t.Errorf("requests = %v, want %v", rec.Methods(), want)
		}
	})
}

const configSource = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
`

// workflowConfigSource は、allow_auto_merge と default_workflow_permissions を宣言する。
const workflowConfigSource = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
    actions:
      workflow:
        default_workflow_permissions: read
`

//trace:checks INV-002
//trace:checks FR-008
//trace:checks FR-011
func TestRun(t *testing.T) {
	newConfig := func(files fstest.MapFS, out *bytes.Buffer, baseURL *url.URL, calls *int) Config {
		return Config{
			Token:      "test-token",
			ConfigDir:  files,
			ConfigName: "reco.yaml",
			ConfigPath: "./reco.yaml",
			Out:        out,
			Approve: func(context.Context) error {
				*calls++
				return nil
			},
			GitHubOptions: []github.Option{github.WithBaseURL(baseURL)},
		}
	}

	t.Run("general and workflow permissions mismatches are applied after a single approval", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t,
			okRepository(1, false, false), okWorkflowPermissions("write", false),
			okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false),
			okRepository(1, true, false), okWorkflowPermissions("write", false), response{status: http.StatusNoContent},
			okRepository(1, true, false), okWorkflowPermissions("read", false))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(workflowConfigSource)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if calls != 1 {
			t.Errorf("approve calls = %d, want 1", calls)
		}
		want := []string{"GET", "GET", "GET", "PATCH", "GET", "GET", "GET", "PUT", "GET", "GET"}
		if !slices.Equal(rec.Methods(), want) {
			t.Errorf("requests = %v, want %v", rec.Methods(), want)
		}
		if !strings.Contains(out.String(), "actions.workflow.default_workflow_permissions: write -> read") ||
			!strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("got Out %q, want the workflow permissions change and the repository updated", out.String())
		}
	})

	t.Run("matching workflow permissions leave only the general change to apply", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t,
			okRepository(1, false, false), okWorkflowPermissions("read", false),
			okRepository(1, false, false), okRepository(1, true, false), okRepository(1, true, false))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(workflowConfigSource)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL, &calls)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if want := []string{"GET", "GET", "GET", "PATCH", "GET"}; !slices.Equal(rec.Methods(), want) {
			t.Errorf("requests = %v, want %v", rec.Methods(), want)
		}
		if !strings.Contains(out.String(), "owner/repo: updated") {
			t.Errorf("got Out %q, want the repository to be updated", out.String())
		}
	})

	t.Run("missing configuration file stops before approval and requests", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0

		err := Run(context.Background(), newConfig(fstest.MapFS{}, &out, baseURL, &calls))

		if err == nil || !strings.Contains(err.Error(), "./reco.yaml") {
			t.Errorf("Run() error = %v, want it to contain the configuration path", err)
		}
		if calls != 0 || len(rec.Methods()) != 0 {
			t.Errorf("approve calls = %d, requests = %v, want none", calls, rec.Methods())
		}
	})

	t.Run("matching observation reports no changes with the token", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t, okRepository(1, true, false))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}

		err := Run(context.Background(), newConfig(files, &out, baseURL, &calls))

		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.HasPrefix(out.String(), "No changes.") {
			t.Errorf("got Out %q, want it to start with %q", out.String(), "No changes.")
		}
		if !strings.Contains(rec.Authorization(), "test-token") {
			t.Errorf("got Authorization %q, want it to contain the token", rec.Authorization())
		}
		if calls != 0 {
			t.Errorf("approve calls = %d, want none", calls)
		}
	})

	t.Run("selection is passed to change planning", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t)
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}
		cfg := newConfig(files, &out, baseURL, &calls)
		selection := mustReference(t, "owner/other")
		cfg.Selection = &selection

		if err := Run(context.Background(), cfg); err == nil {
			t.Fatal("Run() error = nil, want an error")
		}
		if len(rec.Methods()) != 0 {
			t.Errorf("requests = %v, want none", rec.Methods())
		}
	})
}
