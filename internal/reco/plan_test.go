package reco

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

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
	target := repository.Reference{Owner: "owner", Repo: "repo"}
	cases := []struct {
		name     string
		settings []settingPlan
		want     string
	}{
		{
			name: "mismatch is shown as a change with a summary",
			settings: []settingPlan{
				{setting: "general.allow_auto_merge", current: false, desired: true, result: mismatch, applicable: true},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n  general.allow_auto_merge: false -> true\n\n" +
				"Plan: 1 repository to update, 1 known change, 0 not-comparable settings\n",
		},
		{
			name: "matched settings are not shown as changes",
			settings: []settingPlan{
				{setting: "general.matched", current: true, desired: true, result: match},
				{setting: "general.allow_auto_merge", current: true, desired: false, result: mismatch, applicable: true},
			},
			want: "Configuration: ./reco.yaml\n\nRepository: owner/repo\n  general.allow_auto_merge: true -> false\n\n" +
				"Plan: 1 repository to update, 1 known change, 0 not-comparable settings\n",
		},
		{
			name: "all matches report no changes",
			settings: []settingPlan{
				{setting: "general.allow_auto_merge", current: false, desired: false, result: match},
			},
			want: "No changes.\nRepositories checked: 1\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			p := changePlan{target: target, repositoryID: 1, settings: tc.settings}
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
		p := changePlan{target: target, settings: []settingPlan{{setting: "general.allow_auto_merge", result: notComparable}}}
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
		p := changePlan{target: target, settings: []settingPlan{{setting: "general.allow_auto_merge", result: notComparable}}}
		if err := writeChangePlan(&out, "./reco.yaml", p); err != nil {
			t.Fatalf("writeChangePlan() error = %v", err)
		}
		if strings.Contains(out.String(), "->") {
			t.Errorf("writeChangePlan() wrote %q, want no known change line", out.String())
		}
	})

	t.Run("write failure is returned", func(t *testing.T) {
		p := changePlan{target: target, settings: []settingPlan{{result: mismatch}}}
		err := writeChangePlan(failingWriter{}, "./reco.yaml", p)
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangePlan() error = %v, want %v", err, errWrite)
		}
	})

	t.Run("no-change write failure is returned", func(t *testing.T) {
		err := writeChangePlan(failingWriter{}, "./reco.yaml", changePlan{target: target})
		if !errors.Is(err, errWrite) {
			t.Errorf("writeChangePlan() error = %v, want %v", err, errWrite)
		}
	})
}

const planConfigSource = `repositories:
  - name: owner/repo
    general:
      allow_auto_merge: true
`

//trace:checks FR-001
//trace:checks FR-007
func TestPlan(t *testing.T) {
	newConfig := func(files fstest.MapFS, out *bytes.Buffer, baseURL github.Option) PlanConfig {
		return PlanConfig{
			Token:         "test-token",
			ConfigDir:     files,
			ConfigName:    "reco.yaml",
			ConfigPath:    "./reco.yaml",
			Out:           out,
			GitHubOptions: []github.Option{baseURL},
		}
	}

	t.Run("declaration is validated, observed, and the plan is written to Out", func(t *testing.T) {
		var gotPath, gotAuthorization string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAuthorization = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"id": 1, "allow_auto_merge": false, "delete_branch_on_merge": false}`))
		})
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(planConfigSource)}}

		if err := Plan(context.Background(), newConfig(files, &out, github.WithBaseURL(baseURL))); err != nil {
			t.Fatalf("Plan() error = %v", err)
		}
		if !strings.Contains(gotPath, "owner/repo") {
			t.Errorf("got request path %q, want it to contain the declared name", gotPath)
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

	t.Run("missing configuration file is reported with the user path before any request", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		var out bytes.Buffer

		err := Plan(context.Background(), newConfig(fstest.MapFS{}, &out, github.WithBaseURL(baseURL)))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Plan() error = %v, want %v", err, fs.ErrNotExist)
		}
		if err == nil || !strings.Contains(err.Error(), "./reco.yaml") {
			t.Errorf("Plan() error = %v, want it to contain the configuration path", err)
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
		if out.Len() != 0 {
			t.Errorf("got Out %q, want empty", out.String())
		}
	})

	t.Run("invalid configuration is rejected before any request", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte("repositories: []\n")}}

		if err := Plan(context.Background(), newConfig(files, &out, github.WithBaseURL(baseURL))); err == nil {
			t.Fatal("Plan() error = nil, want an error")
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
	})

	t.Run("observation failure is returned without writing to Out", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(planConfigSource)}}

		err := Plan(context.Background(), newConfig(files, &out, github.WithBaseURL(baseURL)))
		if !errors.Is(err, github.ErrNotFound) {
			t.Errorf("Plan() error = %v, want %v", err, github.ErrNotFound)
		}
		if out.Len() != 0 {
			t.Errorf("got Out %q, want empty", out.String())
		}
	})

	t.Run("selection is passed to target selection", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		var out bytes.Buffer
		files := fstest.MapFS{"reco.yaml": {Data: []byte(planConfigSource)}}
		cfg := newConfig(files, &out, github.WithBaseURL(baseURL))
		cfg.Selection = &repository.Reference{Owner: "owner", Repo: "other"}

		if err := Plan(context.Background(), cfg); err == nil {
			t.Fatal("Plan() error = nil, want an error")
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
	})
}
