package show

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

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

//trace:checks FR-004
//trace:checks QR-002
func TestWriteObservationResult(t *testing.T) {
	target := mustReference(t, "owner/repo")
	cases := []struct {
		name     string
		observed github.ObservationResult
		want     string
	}{
		{
			name: "general and workflow permissions are shown in the CLI spec order",
			observed: github.ObservationResult{
				RepositoryID:        1,
				General:             github.General{DeleteBranchOnMerge: true, AllowAutoMerge: false},
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: false},
			},
			want: "Repository: owner/repo\n\nGeneral:\n  delete_branch_on_merge: true\n  allow_auto_merge: false\nWorkflow permissions:\n  default_workflow_permissions: read\n  can_approve_pull_request_reviews: false\n",
		},
		{
			name: "opposite values are shown as observed",
			observed: github.ObservationResult{
				RepositoryID:        1,
				General:             github.General{DeleteBranchOnMerge: false, AllowAutoMerge: true},
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: true},
			},
			want: "Repository: owner/repo\n\nGeneral:\n  delete_branch_on_merge: false\n  allow_auto_merge: true\nWorkflow permissions:\n  default_workflow_permissions: write\n  can_approve_pull_request_reviews: true\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := writeObservationResult(&out, target, tc.observed); err != nil {
				t.Fatalf("writeObservationResult() error = %v", err)
			}
			if out.String() != tc.want {
				t.Errorf("writeObservationResult() wrote %q, want %q", out.String(), tc.want)
			}
		})
	}

	t.Run("write failure is returned", func(t *testing.T) {
		observed := github.ObservationResult{WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead}}
		err := writeObservationResult(failingWriter{}, target, observed)
		if !errors.Is(err, errWrite) {
			t.Errorf("writeObservationResult() error = %v, want %v", err, errWrite)
		}
	})
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

//trace:checks FR-004
func TestRun(t *testing.T) {
	target := mustReference(t, "owner/repo")

	t.Run("both areas are observed with GET only and written to Out", func(t *testing.T) {
		var requests []string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.Method+" "+r.URL.Path)
			if !strings.Contains(r.Header.Get("Authorization"), "test-token") {
				t.Errorf("got Authorization %q, want it to contain the token", r.Header.Get("Authorization"))
			}
			switch r.URL.Path {
			case "/repos/owner/repo":
				_, _ = w.Write([]byte(`{"id": 1, "allow_auto_merge": true, "delete_branch_on_merge": false}`))
			case "/repos/owner/repo/actions/permissions/workflow":
				_, _ = w.Write([]byte(`{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": true}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})
		var out bytes.Buffer
		cfg := Config{Token: "test-token", Target: target, Out: &out, GitHubOptions: []github.Option{github.WithBaseURL(baseURL)}}

		if err := Run(context.Background(), cfg); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := []string{"GET /repos/owner/repo", "GET /repos/owner/repo/actions/permissions/workflow"}
		if !slices.Equal(requests, want) {
			t.Errorf("got requests %v, want %v", requests, want)
		}
		if !strings.Contains(out.String(), "default_workflow_permissions: write") {
			t.Errorf("got Out %q, want it to contain the observed default_workflow_permissions", out.String())
		}
	})

	t.Run("workflow permissions failure is returned without writing to Out", func(t *testing.T) {
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/owner/repo" {
				_, _ = w.Write([]byte(`{"id": 1, "allow_auto_merge": true, "delete_branch_on_merge": false}`))
				return
			}
			w.WriteHeader(http.StatusForbidden)
		})
		var out bytes.Buffer
		cfg := Config{Token: "test-token", Target: target, Out: &out, GitHubOptions: []github.Option{github.WithBaseURL(baseURL)}}

		err := Run(context.Background(), cfg)
		if !errors.Is(err, github.ErrForbidden) {
			t.Errorf("Run() error = %v, want %v", err, github.ErrForbidden)
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
