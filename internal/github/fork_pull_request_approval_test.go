package github

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/githubtest"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestForkPullRequestApprovalUnavailability(t *testing.T) {
	cases := []struct {
		name            string
		private         bool
		wantReason      UnavailableReason
		wantUnavailable bool
	}{
		{name: "private repository is unavailable as private", private: true, wantReason: UnavailablePrivateRepository, wantUnavailable: true},
		{name: "public repository is available", private: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, unavailable := forkPullRequestApprovalUnavailability(tc.private)
			if reason != tc.wantReason || unavailable != tc.wantUnavailable {
				t.Errorf("forkPullRequestApprovalUnavailability(%t) = (%v, %t), want (%v, %t)", tc.private, reason, unavailable, tc.wantReason, tc.wantUnavailable)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestDecodeForkPullRequestApproval(t *testing.T) {
	// 受け付ける値の規則は setting.ParseForkPullRequestApprovalPolicy が持つため、ここでは読み取りの経路を1つの値で確かめる
	successes := []struct {
		name string
		body string
		want SettingObservation[setting.ForkPullRequestApprovalPolicy]
	}{
		{name: "defined approval_policy is read", body: `{"approval_policy": "all_external_contributors"}`, want: Observed(setting.ForkPullRequestApprovalAllExternalContributors)},
		// GitHub が列挙に値を足しても、観測操作を失敗させず、この設定だけを観測不能にする
		{
			name: "unknown approval_policy is unavailable as an unknown value",
			body: `{"approval_policy": "everyone"}`,
			want: Unavailable[setting.ForkPullRequestApprovalPolicy](UnavailableUnknownValue),
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeForkPullRequestApproval([]byte(tc.body))
			if err != nil {
				t.Fatalf("decodeForkPullRequestApproval() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("decodeForkPullRequestApproval() = %+v, want %+v", got, tc.want)
			}
		})
	}

	failures := []struct {
		name        string
		body        string
		errContains string
	}{
		{name: "missing approval_policy is unexpected response", body: `{}`, errContains: "missing fields: approval_policy"},
		{name: "null approval_policy is unexpected response", body: `{"approval_policy": null}`, errContains: "missing fields: approval_policy"},
		{name: "non-string approval_policy is unexpected response", body: `{"approval_policy": 1}`},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeForkPullRequestApproval([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodeForkPullRequestApproval() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodeForkPullRequestApproval() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObserveRepositoryForkPullRequestApproval(t *testing.T) {
	t.Run("public repository reads the approval policy without reading the other fork pull request area", func(t *testing.T) {
		s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath:              githubtest.OK(repositoryBody),
			"GET " + forkPullRequestApprovalPath: githubtest.OK(githubtest.ForkPullRequestApprovalBody{ApprovalPolicy: "first_time_contributors"}.JSON()),
		}))
		c := New(testToken, WithBaseURL(s.URL()))

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaForkPullRequestApproval)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := &ForkPullRequestApproval{ApprovalPolicy: Observed(setting.ForkPullRequestApprovalFirstTimeContributors)}
		if !reflect.DeepEqual(observed.ForkPullRequestApproval, want) {
			t.Errorf("ObserveRepository().ForkPullRequestApproval = %+v, want %+v", observed.ForkPullRequestApproval, want)
		}
		wantLines := []string{"GET " + repositoryPath, "GET " + forkPullRequestApprovalPath}
		if got := s.RequestLines(); !reflect.DeepEqual(got, wantLines) {
			t.Errorf("requests = %q, want %q", got, wantLines)
		}
	})

	t.Run("private repository is unavailable without the read request", func(t *testing.T) {
		s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath: githubtest.OK(privateRepositoryBody),
		}))
		c := New(testToken, WithBaseURL(s.URL()))

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaForkPullRequestApproval)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := &ForkPullRequestApproval{ApprovalPolicy: Unavailable[setting.ForkPullRequestApprovalPolicy](UnavailablePrivateRepository)}
		if !reflect.DeepEqual(observed.ForkPullRequestApproval, want) {
			t.Errorf("ObserveRepository().ForkPullRequestApproval = %+v, want %+v", observed.ForkPullRequestApproval, want)
		}
		wantLines := []string{"GET " + repositoryPath}
		if got := s.RequestLines(); !reflect.DeepEqual(got, wantLines) {
			t.Errorf("requests = %q, want %q", got, wantLines)
		}
	})

	const bodyMarker = "BODY-MARKER"
	failures := []struct {
		name     string
		response githubtest.Response
		want     error
	}{
		{name: "422 after observing public is unprocessable", response: githubtest.Response{Status: http.StatusUnprocessableEntity, Body: bodyMarker}, want: ErrUnprocessable},
		{name: "403 is forbidden", response: githubtest.Response{Status: http.StatusForbidden, Body: bodyMarker}, want: ErrForbidden},
		{name: "404 is not found", response: githubtest.Response{Status: http.StatusNotFound, Body: bodyMarker}, want: ErrNotFound},
		{name: "301 is redirect", response: githubtest.Response{Status: http.StatusMovedPermanently}, want: ErrRedirect},
		{name: "missing approval_policy is unexpected response", response: githubtest.OK(`{}`), want: ErrUnexpectedResponse},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
				"GET " + repositoryPath:              githubtest.OK(repositoryBody),
				"GET " + forkPullRequestApprovalPath: tc.response,
			}))
			c := New(testToken, WithBaseURL(s.URL()))

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaForkPullRequestApproval)
			const wantFetch = "fetch fork pull request approval owner/repo"
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), wantFetch) {
				t.Fatalf("ObserveRepository() error = %v, want %v from %s", err, tc.want, wantFetch)
			}
			if observed != (ObservationResult{}) {
				t.Errorf("ObserveRepository() = %+v, want the zero value", observed)
			}
			for _, secret := range []string{testToken, bodyMarker} {
				if strings.Contains(err.Error(), secret) {
					t.Errorf("error = %q, must not contain %q", err, secret)
				}
			}
		})
	}
}
