package apply

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

// recorder は stub が受けた要求の method と Authorization を記録する。
type recorder struct {
	mu            sync.Mutex
	methods       []string
	authorization string
}

func (r *recorder) Methods() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.methods)
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
		rec.mu.Lock()
		rec.methods = append(rec.methods, r.Method)
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

func newClient(baseURL *url.URL) *github.Client {
	return github.New("test-token", github.WithBaseURL(baseURL))
}

// allowAutoMergeChange は、allow_auto_merge を false から true にする論理的な設定変更。
func allowAutoMergeChange() changeplan.LogicalChange {
	return changeplan.LogicalChange{Key: setting.AllowAutoMerge, Observed: false, Desired: true}
}

// allowAutoMergeMismatch は、allowAutoMergeChange を導く適用可能な変更予定。
func allowAutoMergeMismatch() changeplan.SettingPlan {
	return changeplan.SettingPlan{
		Key:           setting.AllowAutoMerge,
		Observed:      false,
		Desired:       true,
		Result:        changeplan.Mismatch,
		Applicability: changeplan.Applicable,
	}
}

func newPlan(t *testing.T, settings ...changeplan.SettingPlan) changeplan.Plan {
	t.Helper()
	return changeplan.Plan{Target: mustReference(t, "owner/repo"), RepositoryID: 1, Settings: settings}
}

// TestSingleLogicalChange は、変更の実行が扱える論理的な設定変更に限る規則を確かめる。
//
//trace:checks INV-002
func TestSingleLogicalChange(t *testing.T) {
	match := changeplan.SettingPlan{Key: setting.DeleteBranchOnMerge, Observed: true, Desired: true, Result: changeplan.Match}

	t.Run("all matches have no change", func(t *testing.T) {
		_, ok, err := singleLogicalChange(newPlan(t, match))
		if err != nil || ok {
			t.Errorf("singleLogicalChange() ok = %t, error = %v, want no change without error", ok, err)
		}
	})

	t.Run("applicable mismatch of allow_auto_merge is returned as the logical change", func(t *testing.T) {
		got, ok, err := singleLogicalChange(newPlan(t, match, allowAutoMergeMismatch()))
		if err != nil || !ok {
			t.Fatalf("singleLogicalChange() ok = %t, error = %v, want a change without error", ok, err)
		}
		if want := allowAutoMergeChange(); got != want {
			t.Errorf("singleLogicalChange() = %+v, want %+v", got, want)
		}
	})

	errorCases := []struct {
		name     string
		settings []changeplan.SettingPlan
	}{
		{
			name:     "two logical changes are an error",
			settings: []changeplan.SettingPlan{allowAutoMergeMismatch(), allowAutoMergeMismatch()},
		},
		{
			name: "logical change of another setting is an error",
			settings: []changeplan.SettingPlan{
				{Key: setting.DeleteBranchOnMerge, Observed: false, Desired: true, Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
			},
		},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok, err := singleLogicalChange(newPlan(t, tc.settings...))
			if err == nil {
				t.Errorf("singleLogicalChange() ok = %t, error = nil, want an error", ok)
			}
		})
	}
}

//trace:checks FR-009
func TestWriteChangePlan(t *testing.T) {
	target := mustReference(t, "owner/repo")

	t.Run("change is written with the configuration and the repository", func(t *testing.T) {
		var out bytes.Buffer
		if err := writeChangePlan(&out, "./reco.yaml", target, allowAutoMergeChange()); err != nil {
			t.Fatalf("writeChangePlan() error = %v", err)
		}
		want := "Configuration: ./reco.yaml\nRepository: owner/repo\n  general.allow_auto_merge: false -> true\n\n"
		if out.String() != want {
			t.Errorf("writeChangePlan() wrote %q, want %q", out.String(), want)
		}
	})

	t.Run("write failure is returned", func(t *testing.T) {
		err := writeChangePlan(failingWriter{}, "./reco.yaml", target, allowAutoMergeChange())
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
		match := changeplan.SettingPlan{Key: setting.DeleteBranchOnMerge, Observed: true, Desired: true, Result: changeplan.Match}

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

	t.Run("change plan is written to Out before approval is requested", func(t *testing.T) {
		baseURL, _ := newSequenceServer(t)
		var out bytes.Buffer
		var shown string
		cfg := Config{Out: &out, ConfigPath: "./reco.yaml", Approve: func(context.Context) error {
			shown = out.String()
			return errors.New("declined")
		}}

		_ = applyPlan(context.Background(), cfg, newClient(baseURL), newPlan(t, allowAutoMergeMismatch()))

		if shown == "" {
			t.Error("Out at approval is empty, want the change plan")
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

const bothGeneralSource = `repositories:
  - name: owner/repo
    general:
      delete_branch_on_merge: true
      allow_auto_merge: true
`

//trace:checks INV-002
//trace:checks FR-008
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

	t.Run("plan with a delete_branch_on_merge change stops before showing the plan, approval and update", func(t *testing.T) {
		baseURL, rec := newSequenceServer(t, okRepository(1, true, false))
		var out bytes.Buffer
		calls := 0
		files := fstest.MapFS{"reco.yaml": {Data: []byte(bothGeneralSource)}}

		err := Run(context.Background(), newConfig(files, &out, baseURL, &calls))

		if err == nil {
			t.Fatal("Run() error = nil, want an error")
		}
		if out.Len() != 0 || calls != 0 {
			t.Errorf("got Out %q, approve calls = %d, want neither the plan nor approval", out.String(), calls)
		}
		if want := []string{http.MethodGet}; !slices.Equal(rec.Methods(), want) {
			t.Errorf("requests = %v, want %v", rec.Methods(), want)
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
