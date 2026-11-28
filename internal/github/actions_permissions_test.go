package github

import (
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
			// 無効の判定が先に打ち切るため、知らない値も読まず、理由は Actions が無効であること
			name: "disabled ignores an included unknown allowed_actions",
			body: `{"enabled": false, "allowed_actions": "none", "sha_pinning_required": false}`,
			want: ActionsPermissions{
				AllowedActions:     Unavailable[setting.AllowedActionsPolicy](UnavailableActionsDisabled),
				SHAPinningRequired: Unavailable[bool](UnavailableActionsDisabled),
			},
		},
		{
			// sha_pinning_required は Actions の有効状態だけを取得条件とするため、ポリシーが知らない値でも観測値を持つ
			name: "enabled with an unknown allowed_actions makes only the policy unavailable as an unknown value",
			body: `{"enabled": true, "allowed_actions": "none", "sha_pinning_required": true}`,
			want: ActionsPermissions{
				Enabled:            true,
				AllowedActions:     Unavailable[setting.AllowedActionsPolicy](UnavailableUnknownValue),
				SHAPinningRequired: Observed(true),
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
		name        string
		body        string
		errContains []string
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
			name:        "enabled with a non-string allowed_actions is unexpected response",
			body:        `{"enabled": true, "allowed_actions": 1, "sha_pinning_required": false}`,
			errContains: []string{"allowed_actions"},
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
		{
			// ポリシーが selected かを判断できないため、取得を送らず、ポリシーと同じ理由にする
			name:            "unknown policy keeps the reason of the policy",
			allowed:         Unavailable[setting.AllowedActionsPolicy](UnavailableUnknownValue),
			wantReason:      UnavailableUnknownValue,
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
