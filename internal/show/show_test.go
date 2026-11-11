package show

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

//trace:checks FR-004
//trace:checks QR-002
func TestWriteObservation(t *testing.T) {
	target := repository.Reference{Owner: "owner", Repo: "repo"}
	cases := []struct {
		name           string
		allowAutoMerge bool
		want           string
	}{
		{
			name:           "allow_auto_merge true is shown under General",
			allowAutoMerge: true,
			want:           "Repository: owner/repo\n\nGeneral:\n  allow_auto_merge: true\n",
		},
		{
			name:           "allow_auto_merge false is shown under General",
			allowAutoMerge: false,
			want:           "Repository: owner/repo\n\nGeneral:\n  allow_auto_merge: false\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			observed := github.Repository{ID: 1, General: github.General{AllowAutoMerge: tc.allowAutoMerge}}
			if err := writeObservation(&out, target, observed); err != nil {
				t.Fatalf("writeObservation() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeObservation() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}

	t.Run("write failure is returned", func(t *testing.T) {
		err := writeObservation(failingWriter{}, target, github.Repository{})
		if !errors.Is(err, errWrite) {
			t.Errorf("writeObservation() error = %v, want %v", err, errWrite)
		}
	})
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

//trace:checks FR-004
func TestRun(t *testing.T) {
	target := repository.Reference{Owner: "owner", Repo: "repo"}

	t.Run("observed repository is written to Out", func(t *testing.T) {
		var gotPath, gotAuthorization string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAuthorization = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(`{"id": 1, "allow_auto_merge": true, "delete_branch_on_merge": false}`))
		})
		var out bytes.Buffer
		cfg := Config{Token: "test-token", Target: target, Out: &out, GitHubOptions: []github.Option{github.WithBaseURL(baseURL)}}

		if err := Run(context.Background(), cfg); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if !strings.Contains(gotPath, "owner/repo") {
			t.Errorf("got request path %q, want it to contain the target", gotPath)
		}
		if !strings.Contains(gotAuthorization, "test-token") {
			t.Errorf("got Authorization %q, want it to contain the token", gotAuthorization)
		}
		if !strings.Contains(out.String(), "allow_auto_merge: true") {
			t.Errorf("got Out %q, want it to contain the observed allow_auto_merge", out.String())
		}
	})

	t.Run("observation failure is returned without writing to Out", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		var out bytes.Buffer
		cfg := Config{Token: "test-token", Target: target, Out: &out, GitHubOptions: []github.Option{github.WithBaseURL(baseURL)}}

		err := Run(context.Background(), cfg)
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
