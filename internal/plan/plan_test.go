package plan

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}

func TestPluralize(t *testing.T) {
	cases := []struct {
		name             string
		n                int
		singular, plural string
		want             string
	}{
		{name: "zero is plural", n: 0, singular: "known change", plural: "known changes", want: "0 known changes"},
		{name: "one is singular", n: 1, singular: "known change", plural: "known changes", want: "1 known change"},
		{name: "two uses the given plural form", n: 2, singular: "repository", plural: "repositories", want: "2 repositories"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pluralize(tc.n, tc.singular, tc.plural); got != tc.want {
				t.Errorf("pluralize() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-005
//trace:checks FR-007
//trace:checks QR-002
func TestWriteChangePlan(t *testing.T) {
	target := mustReference(t, "owner/repo")
	cases := []struct {
		name     string
		settings []changeplan.SettingPlan
		want     string
	}{
		{
			name: "mismatch is shown as a change with a summary",
			settings: []changeplan.SettingPlan{
				{Key: "general.allow_auto_merge", Observed: false, Desired: true, Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n  general.allow_auto_merge: false -> true\n\n" +
				"Plan: 1 repository to update, 1 known change, 0 not-comparable settings\n",
		},
		{
			name: "matched settings are not shown as changes",
			settings: []changeplan.SettingPlan{
				{Key: "general.matched", Observed: true, Desired: true, Result: changeplan.Match},
				{Key: "general.allow_auto_merge", Observed: true, Desired: false, Result: changeplan.Mismatch, Applicability: changeplan.Applicable},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n  general.allow_auto_merge: true -> false\n\n" +
				"Plan: 1 repository to update, 1 known change, 0 not-comparable settings\n",
		},
		{
			name: "all matches report no changes",
			settings: []changeplan.SettingPlan{
				{Key: "general.allow_auto_merge", Observed: false, Desired: false, Result: changeplan.Match},
			},
			want: "No changes.\nRepositories checked: 1\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			p := changeplan.Plan{Target: target, RepositoryID: 1, Settings: tc.settings}
			if err := writeChangePlan(&out, "./reco.yaml", p); err != nil {
				t.Fatalf("writeChangePlan() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeChangePlan() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}

	t.Run("not-comparable setting is never reported as no changes", func(t *testing.T) {
		var out bytes.Buffer
		p := changeplan.Plan{Target: target, Settings: []changeplan.SettingPlan{{Key: "general.allow_auto_merge", Result: changeplan.NotComparable}}}
		if err := writeChangePlan(&out, "./reco.yaml", p); err != nil {
			t.Fatalf("writeChangePlan() error = %v", err)
		}
		if strings.HasPrefix(out.String(), "No changes.") {
			t.Errorf("writeChangePlan() wrote %q, want a plan, not no changes", out.String())
		}
	})

	// 比較不能の現在値は確認できていないため、確認済みの差分の形式 (現在値 -> 期待値) で書かない
	t.Run("not-comparable setting is not shown as a known change", func(t *testing.T) {
		var out bytes.Buffer
		p := changeplan.Plan{Target: target, Settings: []changeplan.SettingPlan{{Key: "general.allow_auto_merge", Result: changeplan.NotComparable}}}
		if err := writeChangePlan(&out, "./reco.yaml", p); err != nil {
			t.Fatalf("writeChangePlan() error = %v", err)
		}
		if strings.Contains(out.String(), "->") {
			t.Errorf("writeChangePlan() wrote %q, want no known change line", out.String())
		}
	})

	t.Run("write failure is returned", func(t *testing.T) {
		p := changeplan.Plan{Target: target, Settings: []changeplan.SettingPlan{{Result: changeplan.Mismatch}}}
		err := writeChangePlan(failingWriter{}, "./reco.yaml", p)
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangePlan() error = %v, want %v", err, errWrite)
		}
	})

	t.Run("no-change write failure is returned", func(t *testing.T) {
		err := writeChangePlan(failingWriter{}, "./reco.yaml", changeplan.Plan{Target: target})
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangePlan() error = %v, want %v", err, errWrite)
		}
	})
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

const configSource = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
`

//trace:checks FR-007
func TestRun(t *testing.T) {
	newConfig := func(files fstest.MapFS, out *bytes.Buffer, baseURL *url.URL) Config {
		return Config{
			Token:         "test-token",
			ConfigDir:     files,
			ConfigName:    "reco.yaml",
			ConfigPath:    "./reco.yaml",
			Out:           out,
			GitHubOptions: []github.Option{github.WithBaseURL(baseURL)},
		}
	}

	t.Run("generated change plan is written to Out", func(t *testing.T) {
		var gotAuthorization string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			gotAuthorization = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"id": 1, "allow_auto_merge": false, "delete_branch_on_merge": false}`))
		})
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}

		if err := Run(context.Background(), newConfig(files, &out, baseURL)); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.Contains(gotAuthorization, "test-token") {
			t.Errorf("got Authorization %q, want it to contain the token", gotAuthorization)
		}
		for _, want := range []string{"false -> true", "Configuration: ./reco.yaml"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("got Out %q, want it to contain %q", out.String(), want)
			}
		}
	})

	t.Run("selection is passed to change planning", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}
		cfg := newConfig(files, &out, baseURL)
		selection := mustReference(t, "owner/other")
		cfg.Selection = &selection

		if err := Run(context.Background(), cfg); err == nil {
			t.Fatal("Run() error = nil, want an error")
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
	})

	t.Run("configuration path is passed to change planning", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) {})
		var out bytes.Buffer

		err := Run(context.Background(), newConfig(fstest.MapFS{}, &out, baseURL))
		if err == nil || !strings.Contains(err.Error(), "./reco.yaml") {
			t.Errorf("Run() error = %v, want it to contain the configuration path", err)
		}
	})

	t.Run("change planning failure is returned without writing to Out", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}

		err := Run(context.Background(), newConfig(files, &out, baseURL))
		if !errors.Is(err, github.ErrNotFound) {
			t.Errorf("Run() error = %v, want %v", err, github.ErrNotFound)
		}
		if out.Len() != 0 {
			t.Errorf("got Out %q, want empty", out.String())
		}
	})
}

func newGitHubServer(t *testing.T, handler http.HandlerFunc) *url.URL {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return u
}
