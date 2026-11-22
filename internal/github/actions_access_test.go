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
