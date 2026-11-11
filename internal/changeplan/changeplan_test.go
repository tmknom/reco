package changeplan

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/tmknom/reco/internal/github"
)

func TestPlan_Count(t *testing.T) {
	p := Plan{Settings: []SettingPlan{{Result: Mismatch}, {Result: Match}, {Result: Mismatch}, {Result: NotComparable}}}
	if got := p.Count(Mismatch); got != 2 {
		t.Errorf("Count(Mismatch) = %d, want 2", got)
	}
}

const twoRepositoriesSource = `repositories:
  - name: owner/first
    general:
      allow_auto_merge: true
  - name: owner/second
    general:
      allow_auto_merge: true
`

//trace:checks FR-001
//trace:checks FR-002
func TestGenerate(t *testing.T) {
	file := func(files fstest.MapFS) ConfigurationFile {
		return ConfigurationFile{Dir: files, Name: "reco.yaml", Path: "./reco.yaml"}
	}
	declared := fstest.MapFS{"reco.yaml": {Data: []byte(twoRepositoriesSource)}}

	t.Run("selected declaration is observed and reconciled", func(t *testing.T) {
		var gotPath string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			_, _ = w.Write([]byte(`{"id": 7, "allow_auto_merge": false, "delete_branch_on_merge": false}`))
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))
		sel := mustReference(t, "Owner/Second")

		got, err := Generate(context.Background(), client, file(declared), &sel)
		if err != nil {
			t.Fatalf("Generate() error = %v", err)
		}
		if !strings.Contains(gotPath, "owner/second") {
			t.Errorf("got request path %q, want it to contain the selected declaration", gotPath)
		}
		if want := mustReference(t, "owner/second"); got.Target != want {
			t.Errorf("Target = %v, want %v", got.Target, want)
		}
		if got.RepositoryID != 7 {
			t.Errorf("RepositoryID = %d, want 7", got.RepositoryID)
		}
		want := SettingPlan{Setting: "general.allow_auto_merge", Current: false, Desired: true, Result: Mismatch, Applicability: Applicable}
		if len(got.Settings) != 1 || got.Settings[0] != want {
			t.Errorf("Settings = %+v, want [%+v]", got.Settings, want)
		}
	})

	t.Run("missing configuration file is reported with the user path before any request", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		client := github.New("test-token", github.WithBaseURL(baseURL))

		_, err := Generate(context.Background(), client, file(fstest.MapFS{}), nil)
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Generate() error = %v, want %v", err, fs.ErrNotExist)
		}
		if err == nil || !strings.Contains(err.Error(), "./reco.yaml") {
			t.Errorf("Generate() error = %v, want it to contain the configuration path", err)
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
	})

	t.Run("invalid configuration is rejected before any request", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		client := github.New("test-token", github.WithBaseURL(baseURL))
		files := fstest.MapFS{"reco.yaml": {Data: []byte("repositories: []\n")}}

		if _, err := Generate(context.Background(), client, file(files), nil); err == nil {
			t.Fatal("Generate() error = nil, want an error")
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
	})

	t.Run("target selection failure is returned before any request", func(t *testing.T) {
		requested := false
		baseURL := newGitHubServer(t, func(http.ResponseWriter, *http.Request) { requested = true })
		client := github.New("test-token", github.WithBaseURL(baseURL))
		sel := mustReference(t, "owner/other")

		if _, err := Generate(context.Background(), client, file(declared), &sel); err == nil {
			t.Fatal("Generate() error = nil, want an error")
		}
		if requested {
			t.Error("GitHub received a request, want none")
		}
	})

	t.Run("observation failure is returned as is", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
		client := github.New("test-token", github.WithBaseURL(baseURL))
		sel := mustReference(t, "owner/first")

		_, err := Generate(context.Background(), client, file(declared), &sel)
		if !errors.Is(err, github.ErrUnauthorized) {
			t.Errorf("Generate() error = %v, want %v", err, github.ErrUnauthorized)
		}
		if err == nil || !strings.HasPrefix(err.Error(), "fetch repository") {
			t.Errorf("Generate() error = %v, want it to start with %q", err, "fetch repository")
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
