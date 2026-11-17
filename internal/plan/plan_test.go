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

	"github.com/tmknom/reco/internal/configuration"
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
			Configuration: configuration.File{Dir: files, Name: "reco.yaml", Path: "./reco.yaml"},
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

	t.Run("write failure is returned", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"id": 1, "allow_auto_merge": false, "delete_branch_on_merge": false}`))
		})
		files := fstest.MapFS{"reco.yaml": {Data: []byte(configSource)}}
		cfg := newConfig(files, nil, baseURL)
		cfg.Out = failingWriter{}

		if err := Run(context.Background(), cfg); !errors.Is(err, errWrite) {
			t.Errorf("Run() error = %v, want %v", err, errWrite)
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
