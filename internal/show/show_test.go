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
	// sha_pinning_required は CLI 仕様の show に含まれないため、true にしても出力に現れないことを確かめる
	allPolicy := &github.ActionsPermissions{
		Enabled:            true,
		AllowedActions:     github.Observed(setting.AllowedActionsAll),
		SHAPinningRequired: github.Observed(true),
		SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableNotSelected),
	}
	readOnly := &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: false}
	cases := []struct {
		name     string
		observed github.ObservationResult
		want     string
	}{
		{
			name: "all areas are shown in the CLI spec order",
			observed: github.ObservationResult{
				RepositoryID:        1,
				General:             github.General{DeleteBranchOnMerge: true, AllowAutoMerge: false},
				ActionsPermissions:  allPolicy,
				WorkflowPermissions: readOnly,
			},
			want: "Repository: owner/repo\n\nGeneral:\n  delete_branch_on_merge: true\n  allow_auto_merge: false\n" +
				"Actions permissions:\n  enabled: true\n  allowed_actions: all\n  selected_actions: unavailable (allowed_actions is not \"selected\")\n" +
				"Workflow permissions:\n  default_workflow_permissions: read\n  can_approve_pull_request_reviews: false\n",
		},
		{
			name: "opposite values are shown as observed",
			observed: github.ObservationResult{
				RepositoryID:        1,
				General:             github.General{DeleteBranchOnMerge: false, AllowAutoMerge: true},
				ActionsPermissions:  allPolicy,
				WorkflowPermissions: &github.WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: true},
			},
			want: "Repository: owner/repo\n\nGeneral:\n  delete_branch_on_merge: false\n  allow_auto_merge: true\n" +
				"Actions permissions:\n  enabled: true\n  allowed_actions: all\n  selected_actions: unavailable (allowed_actions is not \"selected\")\n" +
				"Workflow permissions:\n  default_workflow_permissions: write\n  can_approve_pull_request_reviews: true\n",
		},
		{
			name: "selected policy shows the three criteria indented",
			observed: github.ObservationResult{
				RepositoryID: 1,
				ActionsPermissions: &github.ActionsPermissions{
					Enabled:            true,
					AllowedActions:     github.Observed(setting.AllowedActionsSelected),
					SHAPinningRequired: github.Observed(false),
					SelectedActions: github.Observed(setting.SelectedActionsCriteria{
						GitHubOwnedAllowed: true,
						VerifiedAllowed:    false,
						PatternsAllowed:    []string{"actions/checkout@v4", "example/*"},
					}),
				},
				WorkflowPermissions: readOnly,
			},
			want: "Repository: owner/repo\n\nGeneral:\n  delete_branch_on_merge: false\n  allow_auto_merge: false\n" +
				"Actions permissions:\n  enabled: true\n  allowed_actions: selected\n  selected_actions:\n" +
				"    github_owned_allowed: true\n    verified_allowed: false\n    patterns_allowed: [\"actions/checkout@v4\", \"example/*\"]\n" +
				"Workflow permissions:\n  default_workflow_permissions: read\n  can_approve_pull_request_reviews: false\n",
		},
		{
			name: "disabled actions shows the policy and criteria as unavailable",
			observed: github.ObservationResult{
				RepositoryID: 1,
				ActionsPermissions: &github.ActionsPermissions{
					AllowedActions:     github.Unavailable[setting.AllowedActionsPolicy](github.UnavailableActionsDisabled),
					SHAPinningRequired: github.Unavailable[bool](github.UnavailableActionsDisabled),
					SelectedActions:    github.Unavailable[setting.SelectedActionsCriteria](github.UnavailableActionsDisabled),
				},
				WorkflowPermissions: readOnly,
			},
			want: "Repository: owner/repo\n\nGeneral:\n  delete_branch_on_merge: false\n  allow_auto_merge: false\n" +
				"Actions permissions:\n  enabled: false\n  allowed_actions: unavailable (GitHub Actions is disabled)\n  selected_actions: unavailable (GitHub Actions is disabled)\n" +
				"Workflow permissions:\n  default_workflow_permissions: read\n  can_approve_pull_request_reviews: false\n",
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
		observed := github.ObservationResult{ActionsPermissions: allPolicy, WorkflowPermissions: readOnly}
		err := writeObservationResult(failingWriter{}, target, observed)
		if !errors.Is(err, errWrite) {
			t.Errorf("writeObservationResult() error = %v, want %v", err, errWrite)
		}
	})
}

//trace:checks QR-002
func TestUnavailableText(t *testing.T) {
	cases := []struct {
		reason github.UnavailableReason
		want   string
	}{
		{reason: github.UnavailableActionsDisabled, want: "unavailable (GitHub Actions is disabled)"},
		{reason: github.UnavailableNotSelected, want: `unavailable (allowed_actions is not "selected")`},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := unavailableText(tc.reason); got != tc.want {
				t.Errorf("unavailableText() = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("undefined reason panics", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Error("unavailableText() did not panic for an undefined reason")
			}
		}()
		_ = unavailableText(0)
	})
}

var errWrite = errors.New("write failed")

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

//trace:checks FR-004
func TestRun(t *testing.T) {
	target := mustReference(t, "owner/repo")

	t.Run("all areas are observed with GET only and written to Out", func(t *testing.T) {
		var requests []string
		baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.Method+" "+r.URL.Path)
			if !strings.Contains(r.Header.Get("Authorization"), "test-token") {
				t.Errorf("got Authorization %q, want it to contain the token", r.Header.Get("Authorization"))
			}
			switch r.URL.Path {
			case repositoryPath:
				_, _ = w.Write([]byte(repositoryBody))
			case actionsPermissionsPath:
				_, _ = w.Write([]byte(`{"enabled": true, "allowed_actions": "local_only", "sha_pinning_required": false}`))
			case workflowPermissionsPath:
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
		want := []string{"GET " + repositoryPath, "GET " + actionsPermissionsPath, "GET " + workflowPermissionsPath}
		if !slices.Equal(requests, want) {
			t.Errorf("got requests %v, want %v", requests, want)
		}
		if !strings.Contains(out.String(), "default_workflow_permissions: write") {
			t.Errorf("got Out %q, want it to contain the observed default_workflow_permissions", out.String())
		}
	})

	failures := []struct {
		name       string
		failedPath string
	}{
		{name: "actions permissions failure is returned instead of showing unavailable", failedPath: actionsPermissionsPath},
		{name: "workflow permissions failure is returned without writing to Out", failedPath: workflowPermissionsPath},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			baseURL := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case tc.failedPath:
					w.WriteHeader(http.StatusForbidden)
				case repositoryPath:
					_, _ = w.Write([]byte(repositoryBody))
				case actionsPermissionsPath:
					_, _ = w.Write([]byte(`{"enabled": true, "allowed_actions": "all", "sha_pinning_required": false}`))
				default:
					t.Errorf("unexpected request path %q", r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
				}
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
}

const (
	repositoryPath          = "/repos/owner/repo"
	actionsPermissionsPath  = "/repos/owner/repo/actions/permissions"
	workflowPermissionsPath = "/repos/owner/repo/actions/permissions/workflow"
	repositoryBody          = `{"id": 1, "allow_auto_merge": true, "delete_branch_on_merge": false}`
)

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
