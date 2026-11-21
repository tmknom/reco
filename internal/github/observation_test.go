package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tmknom/reco/internal/setting"
)

// actionsDisabledBody は Actions が無効のときの応答で、確認した動作のとおり allowed_actions を含まず sha_pinning_required が false である。
const actionsDisabledBody = `{"enabled": false, "sha_pinning_required": false}`

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

// TestObserveRepositoryFailure は、失敗応答が種類を保って呼び出し元に届き、
// エラー文言にトークン、応答本文、転送先を含めないことを確かめる。種類の判定規則は TestClassifyFailure が確かめる。
//
//trace:checks FR-003 FR-004 QR-001
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
				_, _ = w.Write([]byte(`{"id": 1, "full_name": "owner/repo", "delete_branch_on_merge": false, "private": false}`))
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

//trace:checks FR-004 QR-002
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

//trace:checks FR-004 QR-002
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

//trace:checks FR-004 QR-002
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
//trace:checks FR-004 QR-002
func TestObserveRepositoryAreas(t *testing.T) {
	type request struct {
		method        string
		path          string
		contentLength int64
		header        http.Header
	}
	selectedCriteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"z/*", "a/*"}}
	cases := []struct {
		name string
		// repositoryBody は空のとき公開のリポジトリの repositoryBody を使う
		repositoryBody string
		areas          []setting.Area
		actionsBody    string
		wantPaths      []string
		wantAP         *ActionsPermissions
		wantWP         *WorkflowPermissions
		wantAA         *ActionsAccess
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
		{
			name:           "private repository reads all areas in the fixed order with the actions access last",
			repositoryBody: privateRepositoryBody,
			areas:          []setting.Area{setting.AreaActionsAccess, setting.AreaWorkflowPermissions, setting.AreaActionsPermissions},
			actionsBody:    actionsPermissionsBody("all"),
			wantPaths:      []string{repositoryPath, actionsPermissionsPath, workflowPermissionsPath, actionsAccessPath},
			wantAP: &ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Observed(setting.AllowedActionsAll),
				SHAPinningRequired: Observed(true),
				SelectedActions:    Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected),
			},
			wantWP: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead, CanApprovePullRequestReviews: false},
			wantAA: &ActionsAccess{AccessLevel: Observed(setting.AccessUser)},
		},
		{
			name:      "public repository does not send the actions access GET and is unavailable",
			areas:     []setting.Area{setting.AreaActionsAccess},
			wantPaths: []string{repositoryPath},
			wantAA:    &ActionsAccess{AccessLevel: Unavailable[setting.AccessLevel](UnavailablePublicRepository)},
		},
		{
			name:           "private repository without the actions access area does not send its GET",
			repositoryBody: privateRepositoryBody,
			wantPaths:      []string{repositoryPath},
		},
		{
			name:           "private repository with disabled actions still observes the access level",
			repositoryBody: privateRepositoryBody,
			areas:          []setting.Area{setting.AreaActionsPermissions, setting.AreaActionsAccess},
			actionsBody:    actionsDisabledBody,
			wantPaths:      []string{repositoryPath, actionsPermissionsPath, actionsAccessPath},
			wantAP: &ActionsPermissions{
				AllowedActions:     Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled),
				SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled),
				SelectedActions:    Unavailable[setting.SelectedActionsCriteria](UnavailableActionsDisabled),
			},
			wantAA: &ActionsAccess{AccessLevel: Observed(setting.AccessUser)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got []request
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				got = append(got, request{method: r.Method, path: r.URL.Path, contentLength: r.ContentLength, header: r.Header.Clone()})
				switch r.URL.Path {
				case repositoryPath:
					body := repositoryBody
					if tc.repositoryBody != "" {
						body = tc.repositoryBody
					}
					_, _ = w.Write([]byte(body))
				case actionsPermissionsPath:
					_, _ = w.Write([]byte(tc.actionsBody))
				case selectedActionsPath:
					_, _ = w.Write([]byte(selectedActionsBody))
				case workflowPermissionsPath:
					_, _ = w.Write([]byte(workflowPermissionsBody))
				case actionsAccessPath:
					_, _ = w.Write([]byte(actionsAccessBody))
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
			if !reflect.DeepEqual(observed.ActionsAccess, tc.wantAA) {
				t.Errorf("ObserveRepository().ActionsAccess = %+v, want %+v", observed.ActionsAccess, tc.wantAA)
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
//trace:checks FR-003 FR-004 QR-001 QR-002
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
//trace:checks FR-003 FR-004 QR-001 QR-002
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

func TestObservationResult_Setting(t *testing.T) {
	// 観測結果ごとに真偽値の設定を一つだけ真にし、どの設定も観測結果を通した値の並びがほかの設定とも定数とも異なるようにする。
	// 別の設定の値や定数を返す誤りを検出するため。
	observations := []ObservationResult{
		{General: General{DeleteBranchOnMerge: true}, WorkflowPermissions: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite}},
		{General: General{AllowAutoMerge: true}, WorkflowPermissions: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionRead}},
		{WorkflowPermissions: &WorkflowPermissions{DefaultWorkflowPermissions: setting.PermissionWrite, CanApprovePullRequestReviews: true}},
	}
	yes, no := Observed(setting.BoolValue(true)), Observed(setting.BoolValue(false))
	write, read := Observed(setting.PermissionLevelValue(setting.PermissionWrite)), Observed(setting.PermissionLevelValue(setting.PermissionRead))
	cases := []struct {
		key  setting.Key
		want []SettingObservation[setting.Value]
	}{
		{key: setting.DeleteBranchOnMerge, want: []SettingObservation[setting.Value]{yes, no, no}},
		{key: setting.AllowAutoMerge, want: []SettingObservation[setting.Value]{no, yes, no}},
		{key: setting.DefaultWorkflowPermissions, want: []SettingObservation[setting.Value]{write, read, write}},
		{key: setting.CanApprovePullRequestReviews, want: []SettingObservation[setting.Value]{no, no, yes}},
	}
	for _, tc := range cases {
		t.Run(string(tc.key)+" returns its observed value", func(t *testing.T) {
			for i, observed := range observations {
				if got := observed.Setting(setting.Unit{Key: tc.key}); !reflect.DeepEqual(got, tc.want[i]) {
					t.Errorf("observations[%d].Setting(%q) = %v, want %v", i, tc.key, got, tc.want[i])
				}
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObservationResult_SettingOfActionsPermissions(t *testing.T) {
	// 観測値の設定グループの許可パターンは、応答の並びと重複のまま運ばれることを確かめるため、順不同で重複を含める。
	criteria := setting.SelectedActionsCriteria{GitHubOwnedAllowed: true, PatternsAllowed: []string{"b/*", "a/*", "b/*"}}
	cases := []struct {
		name     string
		observed ActionsPermissions
		key      setting.Key
		want     SettingObservation[setting.Value]
	}{
		{
			name:     "enabled returns true observed for enabled actions",
			observed: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsAll), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected)},
			key:      setting.ActionsEnabled,
			want:     Observed(setting.BoolValue(true)),
		},
		{
			name:     "enabled returns false observed, not unavailable, for disabled actions",
			observed: ActionsPermissions{AllowedActions: Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableActionsDisabled)},
			key:      setting.ActionsEnabled,
			want:     Observed(setting.BoolValue(false)),
		},
		{
			name:     "allowed_actions returns the observed policy",
			observed: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsLocalOnly), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected)},
			key:      setting.AllowedActions,
			want:     Observed(setting.AllowedActionsPolicyValue(setting.AllowedActionsLocalOnly)),
		},
		{
			name:     "allowed_actions keeps the unavailable reason",
			observed: ActionsPermissions{AllowedActions: Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableActionsDisabled)},
			key:      setting.AllowedActions,
			want:     Unavailable[setting.Value](UnavailableActionsDisabled),
		},
		{
			name:     "sha_pinning_required returns the observed value",
			observed: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsAll), SHAPinningRequired: Observed(true), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected)},
			key:      setting.SHAPinningRequired,
			want:     Observed(setting.BoolValue(true)),
		},
		{
			name:     "sha_pinning_required keeps the unavailable reason",
			observed: ActionsPermissions{AllowedActions: Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled), SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableActionsDisabled)},
			key:      setting.SHAPinningRequired,
			want:     Unavailable[setting.Value](UnavailableActionsDisabled),
		},
		{
			name:     "selected_actions returns the observed criteria as is",
			observed: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsSelected), SelectedActions: Observed(criteria)},
			key:      setting.SelectedActions,
			want:     Observed(setting.SelectedActionsCriteriaValue(criteria)),
		},
		{
			name:     "selected_actions keeps the unavailable reason",
			observed: ActionsPermissions{Enabled: true, AllowedActions: Observed(setting.AllowedActionsAll), SelectedActions: Unavailable[setting.SelectedActionsCriteria](UnavailableNotSelected)},
			key:      setting.SelectedActions,
			want:     Unavailable[setting.Value](UnavailableNotSelected),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (ObservationResult{ActionsPermissions: &tc.observed}).Setting(setting.Unit{Key: tc.key}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%q) = %+v, want %+v", tc.key, got, tc.want)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObservationResult_SettingOfActionsAccess(t *testing.T) {
	cases := []struct {
		name     string
		observed ActionsAccess
		want     SettingObservation[setting.Value]
	}{
		{name: "access_level returns the observed level", observed: ActionsAccess{AccessLevel: Observed(setting.AccessUser)}, want: Observed(setting.AccessLevelValue(setting.AccessUser))},
		{name: "access_level keeps the unavailable reason", observed: ActionsAccess{AccessLevel: Unavailable[setting.AccessLevel](UnavailablePublicRepository)}, want: Unavailable[setting.Value](UnavailablePublicRepository)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (ObservationResult{ActionsAccess: &tc.observed}).Setting(setting.Unit{Key: setting.ActionsAccessLevel}); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%q) = %+v, want %+v", setting.ActionsAccessLevel, got, tc.want)
			}
		})
	}
}

//trace:checks FR-004
func TestObservationResult_SettingOfEnvironmentDeploymentBranchPolicy(t *testing.T) {
	// 別の Environment の方式を返す誤りと、宣言の名前を綴りのまま比べる誤りを検出するため、
	// 方式の異なる二つを並べ、ASCII の大小が違う宣言の名前で引く。
	patterns := []setting.DeploymentPattern{{Type: setting.DeploymentPatternBranch, Name: "main"}, {Type: setting.DeploymentPatternTag, Name: "v*"}}
	selected := setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicySelected, Patterns: patterns}
	observed := ObservationResult{Environments: &Environments{Existing: []Environment{
		{ID: 1, Name: "staging", DeploymentBranchPolicy: setting.DeploymentBranchPolicy{Mode: setting.DeploymentBranchPolicyAll}},
		{ID: 2, Name: "Production", DeploymentBranchPolicy: selected},
	}}}
	cases := []struct {
		name string
		unit setting.Unit
		want SettingObservation[setting.Value]
	}{
		{
			name: "declared name differing in ASCII case returns that environment's policy with its patterns",
			unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: mustEnvironmentName(t, "production")},
			want: Observed(setting.DeploymentBranchPolicyValue(selected)),
		},
		{
			name: "absent environment returns UnavailableEnvironmentAbsent",
			unit: setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: mustEnvironmentName(t, "qa")},
			want: Unavailable[setting.Value](UnavailableEnvironmentAbsent),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := observed.Setting(tc.unit); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("Setting(%v) = %+v, want %+v", tc.unit.String(), got, tc.want)
			}
		})
	}
}

func TestObservationResult_SettingOfEnvironmentResourcePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Setting() did not panic for the environment itself")
		}
	}()
	_ = ObservationResult{Environments: &Environments{}}.Setting(setting.Unit{Key: setting.EnvironmentResource, Environment: mustEnvironmentName(t, "production")})
}

func TestObservationResult_SettingOfUnknownSettingPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Setting() did not panic for an unknown setting")
		}
	}()
	_ = ObservationResult{}.Setting(setting.Unit{Key: "general.unknown"})
}

//trace:checks FR-004 QR-002
func TestAccessLevelUnavailability(t *testing.T) {
	cases := []struct {
		name            string
		private         bool
		wantReason      UnavailableReason
		wantUnavailable bool
	}{
		{name: "public repository is unavailable as public", private: false, wantReason: UnavailablePublicRepository, wantUnavailable: true},
		{name: "private repository is available", private: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, unavailable := accessLevelUnavailability(tc.private)
			if reason != tc.wantReason || unavailable != tc.wantUnavailable {
				t.Errorf("accessLevelUnavailability(%t) = (%v, %t), want (%v, %t)", tc.private, reason, unavailable, tc.wantReason, tc.wantUnavailable)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestDecodeActionsAccess(t *testing.T) {
	// 受け付ける値の規則は setting.ParseAccessLevel が持つため、ここでは読み取りの経路を1つの値で確かめる
	t.Run("defined access_level is read", func(t *testing.T) {
		got, err := decodeActionsAccess([]byte(`{"access_level": "user"}`))
		if err != nil {
			t.Fatalf("decodeActionsAccess() error = %v", err)
		}
		if got != setting.AccessUser {
			t.Errorf("decodeActionsAccess() = %q, want %q", got, setting.AccessUser)
		}
	})

	failures := []struct {
		name           string
		body           string
		errContains    string
		errNotContains string
	}{
		{name: "missing access_level is unexpected response", body: `{}`, errContains: "access_level"},
		{
			name:           "undefined access_level is unexpected response without the value",
			body:           `{"access_level": "enterprise"}`,
			errContains:    "access_level",
			errNotContains: "enterprise",
		},
		{name: "non-JSON body is unexpected response", body: `<html>`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeActionsAccess([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeActionsAccess() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeActionsAccess() error = %q, want it to contain %q", err, tc.errContains)
			}
			if tc.errNotContains != "" && strings.Contains(err.Error(), tc.errNotContains) {
				t.Errorf("decodeActionsAccess() error = %q, must not contain %q", err, tc.errNotContains)
			}
		})
	}
}

// TestObserveActionsAccessFailure は、非公開と観測したあとのリポジトリ外のワークフローからのアクセスの取得の失敗が、
// 422 を含めて観測不能にならず、種類を保った観測操作の失敗になることを確かめる。種類の判定規則は TestClassifyFailure が確かめる。
//
//trace:checks FR-003 FR-004 QR-001 QR-002
func TestObserveActionsAccessFailure(t *testing.T) {
	const bodyMarker = "BODY-MARKER"
	var redirected atomic.Int32
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		redirected.Add(1)
	}))
	t.Cleanup(redirectTarget.Close)
	location := redirectTarget.URL + "/repositories/1296269/actions/permissions/access"

	cases := []struct {
		name   string
		access http.HandlerFunc
		want   error
	}{
		{
			name: "422 after observing private is unprocessable",
			access: func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, bodyMarker, http.StatusUnprocessableEntity)
			},
			want: ErrUnprocessable,
		},
		{
			name: "301 is redirect and is not followed",
			access: func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, location, http.StatusMovedPermanently)
			},
			want: ErrRedirect,
		},
		{
			name: "200 with an undefined access_level is unexpected response",
			access: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"access_level": "enterprise"}`))
			},
			want: ErrUnexpectedResponse,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == actionsAccessPath {
					tc.access(w, r)
					return
				}
				_, _ = w.Write([]byte(privateRepositoryBody))
			})
			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaActionsAccess)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ObserveRepository() error = %v, want %v", err, tc.want)
			}
			if observed != (ObservationResult{}) {
				t.Errorf("ObserveRepository() = %+v, want the zero value without the observed General", observed)
			}
			msg := err.Error()
			if !strings.Contains(msg, "fetch actions access owner/repo") {
				t.Errorf("error = %q, want it to name the actions access fetch of owner/repo", msg)
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
}

// actionsPermissionsBody は GET .../actions/permissions の Actions が有効のときの応答に似せ、reco が読まないフィールドも含める。
func actionsPermissionsBody(policy string) string {
	return `{"enabled": true, "allowed_actions": "` + policy + `", "selected_actions_url": "https://api.github.com/repos/owner/repo/actions/permissions/selected-actions", "sha_pinning_required": true}`
}
