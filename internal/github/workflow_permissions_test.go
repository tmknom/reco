package github

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestDecodeWorkflowPermissions(t *testing.T) {
	successes := []struct {
		name string
		body string
		want WorkflowPermissions
	}{
		{
			name: "read and true are read",
			body: `{"default_workflow_permissions": "read", "can_approve_pull_request_reviews": true}`,
			want: WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: true},
		},
		{
			name: "write and false are read",
			body: `{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": false}`,
			want: WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: false},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeWorkflowPermissions([]byte(tc.body))
			if err != nil {
				t.Fatalf("decodeWorkflowPermissions() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("decodeWorkflowPermissions() = %+v, want %+v", got, tc.want)
			}
		})
	}

	failures := []struct {
		name           string
		body           string
		errContains    []string
		errNotContains string
	}{
		{
			name:        "missing default_workflow_permissions is unexpected response",
			body:        `{"can_approve_pull_request_reviews": true}`,
			errContains: []string{"default_workflow_permissions"},
		},
		{
			name:        "missing can_approve_pull_request_reviews is unexpected response",
			body:        `{"default_workflow_permissions": "read"}`,
			errContains: []string{"can_approve_pull_request_reviews"},
		},
		{
			name:        "null default_workflow_permissions is unexpected response",
			body:        `{"default_workflow_permissions": null, "can_approve_pull_request_reviews": true}`,
			errContains: []string{"default_workflow_permissions"},
		},
		{
			name:        "empty object names both missing fields",
			body:        `{}`,
			errContains: []string{"default_workflow_permissions", "can_approve_pull_request_reviews"},
		},
		{
			name:           "unknown default_workflow_permissions is unexpected response without the value",
			body:           `{"default_workflow_permissions": "admin", "can_approve_pull_request_reviews": true}`,
			errContains:    []string{"default_workflow_permissions"},
			errNotContains: "admin",
		},
		{name: "non-JSON body is unexpected response", body: `<html>`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeWorkflowPermissions([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeWorkflowPermissions() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			for _, s := range tc.errContains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("decodeWorkflowPermissions() error = %q, want it to contain %q", err, s)
				}
			}
			if tc.errNotContains != "" && strings.Contains(err.Error(), tc.errNotContains) {
				t.Errorf("decodeWorkflowPermissions() error = %q, must not contain %q", err, tc.errNotContains)
			}
		})
	}
}

// TestObserveWorkflowPermissionsFailure が Workflow permissions の取得の失敗を複数の種類で置くのは、その失敗を固定の種類で包む誤りを検出するため。
// 種類の判定規則は TestClassifyFailure が確かめる。
//
//trace:checks FR-003 FR-004 QR-001 QR-002
func TestObserveWorkflowPermissionsFailure(t *testing.T) {
	const bodyMarker = "BODY-MARKER"
	var redirected atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Add(1)
	}))
	t.Cleanup(redirectTarget.Close)
	location := redirectTarget.URL + "/repositories/1296269/actions/permissions/workflow"

	cases := []struct {
		name     string
		workflow http.HandlerFunc
		want     error
	}{
		{
			name:     "401 is unauthorized",
			workflow: func(w http.ResponseWriter, _ *http.Request) { http.Error(w, bodyMarker, http.StatusUnauthorized) },
			want:     ErrUnauthorized,
		},
		{
			name:     "403 is forbidden",
			workflow: func(w http.ResponseWriter, _ *http.Request) { http.Error(w, bodyMarker, http.StatusForbidden) },
			want:     ErrForbidden,
		},
		{
			name:     "404 is not found",
			workflow: func(w http.ResponseWriter, _ *http.Request) { http.Error(w, bodyMarker, http.StatusNotFound) },
			want:     ErrNotFound,
		},
		{
			name: "429 is rate limited",
			workflow: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, bodyMarker, http.StatusTooManyRequests)
			},
			want: ErrRateLimited,
		},
		{
			name: "301 is redirect and is not followed",
			workflow: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusMovedPermanently)
			},
			want: ErrRedirect,
		},
		{
			name: "200 without can_approve_pull_request_reviews is unexpected response",
			workflow: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"default_workflow_permissions": "read"}`))
			},
			want: ErrUnexpectedResponse,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == workflowPermissionsPath {
					tc.workflow(w, r)
					return
				}
				_, _ = w.Write([]byte(repositoryBody))
			})
			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaWorkflowPermissions)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ObserveRepository() error = %v, want %v", err, tc.want)
			}
			if observed != (ObservationResult{}) {
				t.Errorf("ObserveRepository() = %+v, want the zero value without the observed General", observed)
			}
			msg := err.Error()
			if !strings.Contains(msg, "fetch workflow permissions owner/repo") {
				t.Errorf("error = %q, want it to name the workflow permissions fetch of owner/repo", msg)
			}
			for _, secret := range []string{testToken, bodyMarker, location} {
				if strings.Contains(msg, secret) {
					t.Errorf("error = %q, must not contain %q", msg, secret)
				}
			}
		})
	}
	if n := redirected.Load(); n != 0 {
		t.Errorf("redirect target received %d requests, want 0", n)
	}

	t.Run("repository failure does not send the workflow permissions GET", func(t *testing.T) {
		var paths []string
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaWorkflowPermissions)
		if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "fetch repository owner/repo") {
			t.Errorf("ObserveRepository() error = %v, want %v from fetch repository owner/repo", err, ErrNotFound)
		}
		if len(paths) != 1 || paths[0] != repositoryPath {
			t.Errorf("got request paths %v, want only %s", paths, repositoryPath)
		}
	})
}
