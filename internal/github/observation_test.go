package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

const testToken = "ghp_test-secret-token"

// repositoryBody は GET /repos/{owner}/{repo} の応答に似せ、reco が読まないフィールドも含める。
const repositoryBody = `{
	"id": 1296269,
	"name": "repo",
	"full_name": "owner/repo",
	"owner": {"login": "owner"},
	"private": false,
	"html_url": "https://github.com/owner/repo",
	"allow_auto_merge": true,
	"delete_branch_on_merge": false
}`

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return New(testToken, WithBaseURL(u))
}

func mustReference(t *testing.T, name string) repository.Reference {
	t.Helper()
	reference, err := repository.ParseReference(name)
	if err != nil {
		t.Fatalf("ParseReference(%q) error = %v", name, err)
	}
	return reference
}

func TestRepositoryURL(t *testing.T) {
	cases := []struct {
		name   string
		client *Client
		want   string
	}{
		{
			name:   "default client targets api.github.com",
			client: New(testToken),
			want:   "https://api.github.com/repos/owner/repo",
		},
		{
			name:   "base URL path and query are ignored",
			client: New(testToken, WithBaseURL(&url.URL{Scheme: "http", Host: "127.0.0.1:1", Path: "/api/v3", RawQuery: "x=1"})),
			want:   "http://127.0.0.1:1/repos/owner/repo",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.client.repositoryURL(mustReference(t, "owner/repo")).String(); got != tc.want {
				t.Errorf("repositoryURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

//trace:checks FR-004
func TestDecodeRepository(t *testing.T) {
	t.Run("all fields are read", func(t *testing.T) {
		got, err := decodeRepository([]byte(repositoryBody))
		if err != nil {
			t.Fatalf("decodeRepository() error = %v", err)
		}
		want := ObservationResult{RepositoryID: 1296269, General: General{AllowAutoMerge: true, DeleteBranchOnMerge: false}}
		if got != want {
			t.Errorf("decodeRepository() = %+v, want %+v", got, want)
		}
	})

	failures := []struct {
		name        string
		body        string
		errContains string
	}{
		{name: "missing id is unexpected response", body: `{"full_name": "o/r", "allow_auto_merge": true, "delete_branch_on_merge": true}`, errContains: "id"},
		{name: "missing allow_auto_merge is unexpected response", body: `{"id": 1, "full_name": "o/r", "delete_branch_on_merge": true}`, errContains: "allow_auto_merge"},
		{name: "missing delete_branch_on_merge is unexpected response", body: `{"id": 1, "full_name": "o/r", "allow_auto_merge": true}`, errContains: "delete_branch_on_merge"},
		{name: "null allow_auto_merge is unexpected response", body: `{"id": 1, "full_name": "o/r", "allow_auto_merge": null, "delete_branch_on_merge": true}`, errContains: "allow_auto_merge"},
		{name: "non-JSON body is unexpected response", body: `<html>`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeRepository([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeRepository() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeRepository() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

//trace:checks FR-004
//trace:checks QR-002
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

// TestObserveRepositoryFailure は、失敗応答が種類を保って呼び出し元に届き、
// エラー文言にトークン、応答本文、転送先を含めないことを確かめる。種類の判定規則は TestClassifyFailure が確かめる。
//
//trace:checks FR-003
//trace:checks FR-004
//trace:checks QR-001
func TestObserveRepositoryFailure(t *testing.T) {
	const bodyMarker = "BODY-MARKER"
	var redirected atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Add(1)
	}))
	t.Cleanup(redirectTarget.Close)
	location := redirectTarget.URL + "/repositories/1296269"

	cases := []struct {
		name    string
		handler http.HandlerFunc
		want    error
	}{
		{
			name: "403 with retry-after is rate limited",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, bodyMarker, http.StatusForbidden)
			},
			want: ErrRateLimited,
		},
		{
			name: "301 is redirect and is not followed",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusMovedPermanently)
			},
			want: ErrRedirect,
		},
		{
			name: "200 without allow_auto_merge is unexpected response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"id": 1, "full_name": "owner/repo", "delete_branch_on_merge": false}`))
			},
			want: ErrUnexpectedResponse,
		},
		{
			// 上限を超えても JSON として正しい本文にし、上限を見ずに読めば成功してしまう形にする
			name: "200 with oversized body is unexpected response",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(repositoryBody + strings.Repeat(" ", maxResponseBytes)))
			},
			want: ErrUnexpectedResponse,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, tc.handler)
			_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"))
			if !errors.Is(err, tc.want) {
				t.Fatalf("ObserveRepository() error = %v, want %v", err, tc.want)
			}
			msg := err.Error()
			if !strings.Contains(msg, "owner/repo") {
				t.Errorf("error = %q, want it to name owner/repo", msg)
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

	t.Run("canceled context is communication failure", func(t *testing.T) {
		c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(repositoryBody))
		})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := c.ObserveRepository(ctx, mustReference(t, "owner/repo"))
		if !errors.Is(err, ErrCommunicationFailure) || !errors.Is(err, context.Canceled) {
			t.Fatalf("ObserveRepository() error = %v, want %v wrapping %v", err, ErrCommunicationFailure, context.Canceled)
		}
		if strings.Contains(err.Error(), testToken) {
			t.Errorf("error = %q, must not contain the token", err)
		}
	})
}

const (
	repositoryPath          = "/repos/owner/repo"
	actionsPermissionsPath  = "/repos/owner/repo/actions/permissions"
	selectedActionsPath     = "/repos/owner/repo/actions/permissions/selected-actions"
	workflowPermissionsPath = "/repos/owner/repo/actions/permissions/workflow"
	workflowPermissionsBody = `{"default_workflow_permissions": "read", "can_approve_pull_request_reviews": false}`
	selectedActionsBody     = `{"github_owned_allowed": true, "verified_allowed": false, "patterns_allowed": ["z/*", "a/*"]}`
)

// actionsPermissionsBody は GET .../actions/permissions の Actions が有効のときの応答に似せ、reco が読まないフィールドも含める。
func actionsPermissionsBody(policy string) string {
	return `{"enabled": true, "allowed_actions": "` + policy + `", "selected_actions_url": "https://api.github.com/repos/owner/repo/actions/permissions/selected-actions", "sha_pinning_required": true}`
}

// actionsDisabledBody は Actions が無効のときの応答で、確認した動作のとおり allowed_actions を含まず sha_pinning_required が false である。
const actionsDisabledBody = `{"enabled": false, "sha_pinning_required": false}`

//trace:checks FR-004
//trace:checks QR-002
func TestDecodeActionsPermissions(t *testing.T) {
	successes := []struct {
		name string
		body string
		want ActionsPermissions
	}{
		{
			name: "enabled selected with sha pinning required is read",
			body: actionsPermissionsBody("selected"),
			want: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsSelected), SHAPinningRequired: Observed(true)},
		},
		{
			name: "enabled all without sha pinning required is read",
			body: `{"enabled": true, "allowed_actions": "all", "sha_pinning_required": false}`,
			want: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsAll), SHAPinningRequired: Observed(false)},
		},
		{
			name: "disabled makes the policy and sha pinning unavailable",
			body: actionsDisabledBody,
			want: ActionsPermissions{
				AllowedActions:     Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled),
				SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled),
			},
		},
		{
			name: "disabled ignores an included allowed_actions",
			body: `{"enabled": false, "allowed_actions": "none", "sha_pinning_required": false}`,
			want: ActionsPermissions{
				AllowedActions:     Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled),
				SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled),
			},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeActionsPermissions([]byte(tc.body))
			if err != nil {
				t.Fatalf("decodeActionsPermissions() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("decodeActionsPermissions() = %+v, want %+v", got, tc.want)
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
			name:        "missing enabled is unexpected response",
			body:        `{"allowed_actions": "all", "sha_pinning_required": false}`,
			errContains: []string{"enabled"},
		},
		{
			name:        "null enabled is unexpected response",
			body:        `{"enabled": null, "allowed_actions": "all", "sha_pinning_required": false}`,
			errContains: []string{"enabled"},
		},
		{
			name:        "enabled without allowed_actions is unexpected response",
			body:        `{"enabled": true, "sha_pinning_required": false}`,
			errContains: []string{"allowed_actions"},
		},
		{
			name:        "enabled without sha_pinning_required is unexpected response",
			body:        `{"enabled": true, "allowed_actions": "all"}`,
			errContains: []string{"sha_pinning_required"},
		},
		{
			name:        "enabled with both missing names both fields",
			body:        `{"enabled": true}`,
			errContains: []string{"allowed_actions", "sha_pinning_required"},
		},
		{
			name:           "unknown allowed_actions is unexpected response without the value",
			body:           `{"enabled": true, "allowed_actions": "none", "sha_pinning_required": false}`,
			errContains:    []string{"allowed_actions"},
			errNotContains: "none",
		},
		{name: "non-JSON body is unexpected response", body: `<html>`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeActionsPermissions([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeActionsPermissions() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			for _, s := range tc.errContains {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("decodeActionsPermissions() error = %q, want it to contain %q", err, s)
				}
			}
			if tc.errNotContains != "" && strings.Contains(err.Error(), tc.errNotContains) {
				t.Errorf("decodeActionsPermissions() error = %q, must not contain %q", err, tc.errNotContains)
			}
		})
	}
}

//trace:checks FR-004
//trace:checks QR-002
func TestSelectedActionsUnavailability(t *testing.T) {
	cases := []struct {
		name            string
		allowed         SettingObservation[setting.AllowedActionsPolicy]
		wantReason      UnavailableReason
		wantUnavailable bool
	}{
		{
			name:            "disabled keeps the reason of the policy",
			allowed:         Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled),
			wantReason:      UnavailableActionsDisabled,
			wantUnavailable: true,
		},
		{name: "all is not selected", allowed: Observed(setting.AllowedActionsAll), wantReason: UnavailableNotSelected, wantUnavailable: true},
		{name: "local_only is not selected", allowed: Observed(setting.AllowedActionsLocalOnly), wantReason: UnavailableNotSelected, wantUnavailable: true},
		{name: "selected is fetched", allowed: Observed(setting.AllowedActionsSelected)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, unavailable := selectedActionsUnavailability(tc.allowed)
			if reason != tc.wantReason || unavailable != tc.wantUnavailable {
				t.Errorf("selectedActionsUnavailability() = (%v, %t), want (%v, %t)", reason, unavailable, tc.wantReason, tc.wantUnavailable)
			}
		})
	}
}

//trace:checks FR-004
//trace:checks QR-002
func TestDecodeSelectedActions(t *testing.T) {
	successes := []struct {
		name string
		body string
		want setting.SelectedActionsCriteria
	}{
		{
			name: "three fields are read keeping the pattern order",
			body: selectedActionsBody,
			want: setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, VerifiedAllowed: false, PatternsAllowed: []string{"z/*", "a/*"}},
		},
		{
			name: "empty patterns are an observed empty set",
			body: `{"github_owned_allowed": false, "verified_allowed": true, "patterns_allowed": []}`,
			want: setting.SelectedActionsCriteria{GitHubOwnedAllowed: false, VerifiedAllowed: true, PatternsAllowed: []string{}},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeSelectedActions([]byte(tc.body))
			if err != nil {
				t.Fatalf("decodeSelectedActions() error = %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("decodeSelectedActions() = %#v, want %#v", got, tc.want)
			}
		})
	}

	failures := []struct {
		name           string
		body           string
		errContains    string
		errNotContains string
	}{
		{
			name:        "missing github_owned_allowed is unexpected response",
			body:        `{"verified_allowed": false, "patterns_allowed": []}`,
			errContains: "github_owned_allowed",
		},
		{
			name:        "missing verified_allowed is unexpected response",
			body:        `{"github_owned_allowed": true, "patterns_allowed": []}`,
			errContains: "verified_allowed",
		},
		{
			name:        "missing patterns_allowed is unexpected response",
			body:        `{"github_owned_allowed": true, "verified_allowed": false}`,
			errContains: "patterns_allowed",
		},
		{
			name:        "null patterns_allowed is unexpected response",
			body:        `{"github_owned_allowed": true, "verified_allowed": false, "patterns_allowed": null}`,
			errContains: "patterns_allowed",
		},
		{
			name:           "null pattern is unexpected response without the other patterns",
			body:           `{"github_owned_allowed": true, "verified_allowed": false, "patterns_allowed": ["secret/*", null]}`,
			errContains:    "patterns_allowed",
			errNotContains: "secret/*",
		},
		{
			name: "number pattern is unexpected response",
			body: `{"github_owned_allowed": true, "verified_allowed": false, "patterns_allowed": [1]}`,
		},
		{name: "non-JSON body is unexpected response", body: `<html>`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeSelectedActions([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeSelectedActions() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeSelectedActions() error = %q, want it to contain %q", err, tc.errContains)
			}
			if tc.errNotContains != "" && strings.Contains(err.Error(), tc.errNotContains) {
				t.Errorf("decodeSelectedActions() error = %q, must not contain %q", err, tc.errNotContains)
			}
		})
	}
}

// TestObserveRepositoryAreas は、選んだ設定の領域に応じて送る要求と、観測結果への詰め方を確かめる。
// Selected actions を送らない3件は selectedActionsUnavailability の単体テストと条件が重なるが、
// 取得条件に当たるときに要求そのものを送らないことは、要求の列でしか確かめられない。
//
//trace:checks FR-004
//trace:checks QR-002
func TestObserveRepositoryAreas(t *testing.T) {
	type request struct {
		method        string
		path          string
		contentLength int64
		header        http.Header
	}
	selectedCriteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"z/*", "a/*"}}
	cases := []struct {
		name        string
		areas       []setting.Area
		actionsBody string
		wantPaths   []string
		wantAP      *ActionsPermissions
		wantWP      *WorkflowPermissions
	}{
		{
			name:        "selected policy sends the selected actions GET after the actions permissions",
			areas:       []setting.Area{setting.AreaActionsPermissions},
			actionsBody: actionsPermissionsBody("selected"),
			wantPaths:   []string{repositoryPath, actionsPermissionsPath, selectedActionsPath},
			wantAP: &ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Observed(setting.AllowedActionsSelected),
				SHAPinningRequired: Observed(true),
				SelectedActions:    Observed(selectedCriteria),
			},
		},
		{
			name:        "all policy does not send the selected actions GET",
			areas:       []setting.Area{setting.AreaActionsPermissions},
			actionsBody: actionsPermissionsBody("all"),
			wantPaths:   []string{repositoryPath, actionsPermissionsPath},
			wantAP: &ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Observed(setting.AllowedActionsAll),
				SHAPinningRequired: Observed(true),
				SelectedActions:    Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected),
			},
		},
		{
			name:        "local_only policy does not send the selected actions GET",
			areas:       []setting.Area{setting.AreaActionsPermissions},
			actionsBody: actionsPermissionsBody("local_only"),
			wantPaths:   []string{repositoryPath, actionsPermissionsPath},
			wantAP: &ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Observed(setting.AllowedActionsLocalOnly),
				SHAPinningRequired: Observed(true),
				SelectedActions:    Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected),
			},
		},
		{
			name:        "disabled actions does not send the selected actions GET",
			areas:       []setting.Area{setting.AreaActionsPermissions},
			actionsBody: actionsDisabledBody,
			wantPaths:   []string{repositoryPath, actionsPermissionsPath},
			wantAP: &ActionsPermissions{
				AllowedActions:     Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled),
				SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled),
				SelectedActions:    Unavailable[setting.SelectedActionsCriteria](UnavailableActionsDisabled),
			},
		},
		{
			name:        "both areas are read in the fixed order regardless of the given order",
			areas:       []setting.Area{setting.AreaWorkflowPermissions, setting.AreaActionsPermissions},
			actionsBody: actionsPermissionsBody("selected"),
			wantPaths:   []string{repositoryPath, actionsPermissionsPath, selectedActionsPath, workflowPermissionsPath},
			wantAP: &ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Observed(setting.AllowedActionsSelected),
				SHAPinningRequired: Observed(true),
				SelectedActions:    Observed(selectedCriteria),
			},
			wantWP: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: false},
		},
		{
			name:      "workflow permissions area sends the second GET after the repository",
			areas:     []setting.Area{setting.AreaWorkflowPermissions},
			wantPaths: []string{repositoryPath, workflowPermissionsPath},
			wantWP:    &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: false},
		},
		{
			name:      "no area observes the repository only",
			wantPaths: []string{repositoryPath},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []request
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				got = append(got, request{method: r.Method, path: r.URL.Path, contentLength: r.ContentLength, header: r.Header.Clone()})
				switch r.URL.Path {
				case repositoryPath:
					_, _ = w.Write([]byte(repositoryBody))
				case actionsPermissionsPath:
					_, _ = w.Write([]byte(tc.actionsBody))
				case selectedActionsPath:
					_, _ = w.Write([]byte(selectedActionsBody))
				case workflowPermissionsPath:
					_, _ = w.Write([]byte(workflowPermissionsBody))
				default:
					t.Errorf("unexpected request path %q", r.URL.Path)
					w.WriteHeader(http.StatusInternalServerError)
				}
			})

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), tc.areas...)
			if err != nil {
				t.Fatalf("ObserveRepository() error = %v", err)
			}
			wantGeneral := General{AllowAutoMerge: true, DeleteBranchOnMerge: false}
			if observed.RepositoryID != 1296269 || observed.General != wantGeneral {
				t.Errorf("ObserveRepository() = %+v, want ID 1296269 and General %+v", observed, wantGeneral)
			}
			if !reflect.DeepEqual(observed.ActionsPermissions, tc.wantAP) {
				t.Errorf("ObserveRepository().ActionsPermissions = %+v, want %+v", observed.ActionsPermissions, tc.wantAP)
			}
			if !reflect.DeepEqual(observed.WorkflowPermissions, tc.wantWP) {
				t.Errorf("ObserveRepository().WorkflowPermissions = %+v, want %+v", observed.WorkflowPermissions, tc.wantWP)
			}

			if len(got) != len(tc.wantPaths) {
				t.Fatalf("got %d requests %+v, want paths %v", len(got), got, tc.wantPaths)
			}
			for i, r := range got {
				if r.method != http.MethodGet || r.path != tc.wantPaths[i] {
					t.Errorf("request %d = %s %s, want GET %s", i, r.method, r.path, tc.wantPaths[i])
				}
				if r.contentLength != 0 || r.header.Get("Content-Type") != "" {
					t.Errorf("request %d has Content-Length %d and Content-Type %q, want no body", i, r.contentLength, r.header.Get("Content-Type"))
				}
				headers := map[string]string{
					"Authorization":        "Bearer " + testToken,
					"Accept":               "application/vnd.github+json",
					"X-GitHub-Api-Version": "2026-03-10",
					"User-Agent":           userAgent,
				}
				for name, value := range headers {
					if v := r.header.Get(name); v != value {
						t.Errorf("request %d header %s = %q, want %q", i, name, v, value)
					}
				}
			}
		})
	}
}

// TestObserveRepositoryAreasFailure が Workflow permissions の取得の失敗を複数の種類で置くのは、その失敗を固定の種類で包む誤りを検出するため。
// 種類の判定規則は TestClassifyFailure が確かめる。
//
//trace:checks FR-003
//trace:checks FR-004
//trace:checks QR-001
//trace:checks QR-002
func TestObserveRepositoryAreasFailure(t *testing.T) {
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

// TestObserveActionsPermissionsFailure は、Actions の実行許可の読み取り要求の失敗が、種類を保ったまま
// 取得できた値を含まない観測操作の失敗になり、後続の要求を送らないことを確かめる。種類の判定規則は TestClassifyFailure が確かめる。
//
//trace:checks FR-003
//trace:checks FR-004
//trace:checks QR-001
//trace:checks QR-002
func TestObserveActionsPermissionsFailure(t *testing.T) {
	const bodyMarker = "BODY-MARKER"
	var redirected atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Add(1)
	}))
	t.Cleanup(redirectTarget.Close)
	location := redirectTarget.URL + "/repositories/1296269/actions/permissions"

	fail := func(status int) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { http.Error(w, bodyMarker, status) }
	}
	write := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }
	}
	cases := []struct {
		name      string
		actions   http.HandlerFunc
		selected  http.HandlerFunc
		want      error
		wantFetch string
		wantPaths []string
	}{
		{
			name:      "403 on actions permissions is forbidden",
			actions:   fail(http.StatusForbidden),
			want:      ErrForbidden,
			wantFetch: "fetch actions permissions owner/repo",
			wantPaths: []string{repositoryPath, actionsPermissionsPath},
		},
		{
			name:      "404 on actions permissions is not found",
			actions:   fail(http.StatusNotFound),
			want:      ErrNotFound,
			wantFetch: "fetch actions permissions owner/repo",
			wantPaths: []string{repositoryPath, actionsPermissionsPath},
		},
		{
			name: "301 on actions permissions is redirect and is not followed",
			actions: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusMovedPermanently)
			},
			want:      ErrRedirect,
			wantFetch: "fetch actions permissions owner/repo",
			wantPaths: []string{repositoryPath, actionsPermissionsPath},
		},
		{
			name:      "409 on selected actions after observing selected is conflict",
			actions:   write(actionsPermissionsBody("selected")),
			selected:  fail(http.StatusConflict),
			want:      ErrConflict,
			wantFetch: "fetch selected actions owner/repo",
			wantPaths: []string{repositoryPath, actionsPermissionsPath, selectedActionsPath},
		},
		{
			name:      "selected actions without verified_allowed is unexpected response",
			actions:   write(actionsPermissionsBody("selected")),
			selected:  write(`{"github_owned_allowed": true, "patterns_allowed": []}`),
			want:      ErrUnexpectedResponse,
			wantFetch: "fetch selected actions owner/repo",
			wantPaths: []string{repositoryPath, actionsPermissionsPath, selectedActionsPath},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				switch r.URL.Path {
				case repositoryPath:
					_, _ = w.Write([]byte(repositoryBody))
				case actionsPermissionsPath:
					tc.actions(w, r)
				case selectedActionsPath:
					tc.selected(w, r)
				default:
					_, _ = w.Write([]byte(workflowPermissionsBody))
				}
			})
			areas := []setting.Area{setting.AreaActionsPermissions, setting.AreaWorkflowPermissions}
			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), areas...)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ObserveRepository() error = %v, want %v", err, tc.want)
			}
			if observed != (ObservationResult{}) {
				t.Errorf("ObserveRepository() = %+v, want the zero value without the observed values", observed)
			}
			msg := err.Error()
			if !strings.Contains(msg, tc.wantFetch) {
				t.Errorf("error = %q, want it to contain %q", msg, tc.wantFetch)
			}
			for _, secret := range []string{testToken, bodyMarker, location} {
				if strings.Contains(msg, secret) {
					t.Errorf("error = %q, must not contain %q", msg, secret)
				}
			}
			if !slices.Equal(paths, tc.wantPaths) {
				t.Errorf("got request paths %v, want %v", paths, tc.wantPaths)
			}
		})
	}
	if n := redirected.Load(); n != 0 {
		t.Errorf("redirect target received %d requests, want 0", n)
	}

	t.Run("repository failure does not send the actions permissions GET", func(t *testing.T) {
		var paths []string
		c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		})
		_, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaActionsPermissions)
		if !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "fetch repository owner/repo") {
			t.Errorf("ObserveRepository() error = %v, want %v from fetch repository owner/repo", err, ErrNotFound)
		}
		if len(paths) != 1 || paths[0] != repositoryPath {
			t.Errorf("got request paths %v, want only %s", paths, repositoryPath)
		}
	})
}

func TestObservationResult_Value(t *testing.T) {
	// 観測結果ごとに真偽値の設定を一つだけ真にし、どの設定も観測結果を通した値の並びがほかの設定とも定数とも異なるようにする。
	// 別の設定の値や定数を返す誤りを検出するため。
	observations := []ObservationResult{
		{General: General{DeleteBranchOnMerge: true}, WorkflowPermissions: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite}},
		{General: General{AllowAutoMerge: true}, WorkflowPermissions: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead}},
		{WorkflowPermissions: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: true}},
	}
	yes, no := setting.BoolValue(true), setting.BoolValue(false)
	write, read := setting.PermissionLevelValue(setting.PermissionWrite), setting.PermissionLevelValue(setting.PermissionRead)
	cases := []struct {
		key  setting.Key
		want []setting.Value
	}{
		{key: setting.DeleteBranchOnMerge, want: []setting.Value{yes, no, no}},
		{key: setting.AllowAutoMerge, want: []setting.Value{no, yes, no}},
		{key: setting.DefaultWorkflowPermissions, want: []setting.Value{write, read, write}},
		{key: setting.CanApprovePullRequestReviews, want: []setting.Value{no, no, yes}},
	}
	for _, tc := range cases {
		t.Run(string(tc.key)+" returns its observed value", func(t *testing.T) {
			for i, observed := range observations {
				if got := observed.Value(tc.key); got != tc.want[i] {
					t.Errorf("observations[%d].Value(%q) = %v, want %v", i, tc.key, got, tc.want[i])
				}
			}
		})
	}
}

func TestObservationResult_ValueOfUnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Value() did not panic for an unknown setting")
		}
	}()
	_ = ObservationResult{}.Value("general.unknown")
}
