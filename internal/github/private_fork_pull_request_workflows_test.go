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
func TestPrivateForkPullRequestWorkflowsUnavailability(t *testing.T) {
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
			reason, unavailable := privateForkPullRequestWorkflowsUnavailability(tc.private)
			if reason != tc.wantReason || unavailable != tc.wantUnavailable {
				t.Errorf("privateForkPullRequestWorkflowsUnavailability(%t) = (%v, %t), want (%v, %t)", tc.private, reason, unavailable, tc.wantReason, tc.wantUnavailable)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestDecodePrivateForkPullRequestWorkflows(t *testing.T) {
	// 4項目のどの2つを取り違えても、どちらかのケースで値が入れ替わるよう、真の項目の組を2通りに選ぶ
	successes := []struct {
		name string
		body string
		want setting.PrivateForkPullRequestWorkflowConditions
	}{
		{
			name: "first and last conditions are read in place",
			body: `{"run_workflows_from_fork_pull_requests": true, "send_write_tokens_to_workflows": false, "send_secrets_and_variables": false, "require_approval_for_fork_pr_workflows": true}`,
			want: setting.PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true, RequireApprovalForForkPullRequestWorkflows: true},
		},
		{
			name: "first two conditions are read in place",
			body: `{"run_workflows_from_fork_pull_requests": true, "send_write_tokens_to_workflows": true, "send_secrets_and_variables": false, "require_approval_for_fork_pr_workflows": false}`,
			want: setting.PrivateForkPullRequestWorkflowConditions{RunWorkflowsFromForkPullRequests: true, SendWriteTokensToWorkflows: true},
		},
	}
	for _, tc := range successes {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodePrivateForkPullRequestWorkflows([]byte(tc.body))
			if err != nil || got != tc.want {
				t.Errorf("decodePrivateForkPullRequestWorkflows() = (%+v, %v), want (%+v, nil)", got, err, tc.want)
			}
		})
	}

	failures := []struct {
		name        string
		body        string
		errContains string
	}{
		{
			name:        "missing run_workflows_from_fork_pull_requests is unexpected response",
			body:        `{"send_write_tokens_to_workflows": false, "send_secrets_and_variables": false, "require_approval_for_fork_pr_workflows": false}`,
			errContains: "missing fields: run_workflows_from_fork_pull_requests",
		},
		{
			name:        "missing send_write_tokens_to_workflows is unexpected response",
			body:        `{"run_workflows_from_fork_pull_requests": false, "send_secrets_and_variables": false, "require_approval_for_fork_pr_workflows": false}`,
			errContains: "missing fields: send_write_tokens_to_workflows",
		},
		{
			name:        "null send_secrets_and_variables is unexpected response",
			body:        `{"run_workflows_from_fork_pull_requests": false, "send_write_tokens_to_workflows": false, "send_secrets_and_variables": null, "require_approval_for_fork_pr_workflows": false}`,
			errContains: "missing fields: send_secrets_and_variables",
		},
		{
			name:        "missing require_approval_for_fork_pr_workflows is unexpected response",
			body:        `{"run_workflows_from_fork_pull_requests": false, "send_write_tokens_to_workflows": false, "send_secrets_and_variables": false}`,
			errContains: "missing fields: require_approval_for_fork_pr_workflows",
		},
		{
			name: "every missing condition is named",
			body: `{}`,
			errContains: "missing fields: run_workflows_from_fork_pull_requests, send_write_tokens_to_workflows, " +
				"send_secrets_and_variables, require_approval_for_fork_pr_workflows",
		},
		{
			name: "non-boolean condition is unexpected response",
			body: `{"run_workflows_from_fork_pull_requests": "true", "send_write_tokens_to_workflows": false, "send_secrets_and_variables": false, "require_approval_for_fork_pr_workflows": false}`,
		},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodePrivateForkPullRequestWorkflows([]byte(tc.body))
			if !errors.Is(err, ErrUnexpectedResponse) {
				t.Fatalf("decodePrivateForkPullRequestWorkflows() error = %v, want %v", err, ErrUnexpectedResponse)
			}
			if !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("decodePrivateForkPullRequestWorkflows() error = %q, want it to contain %q", err, tc.errContains)
			}
		})
	}
}

//trace:checks FR-004 QR-002
func TestObserveRepositoryPrivateForkPullRequestWorkflows(t *testing.T) {
	t.Run("private repository reads the conditions without reading the approval policy", func(t *testing.T) {
		body := githubtest.PrivateForkPullRequestWorkflowsBody{RunWorkflowsFromForkPullRequests: true, RequireApprovalForForkPullRequestWorkflows: true}.JSON()
		s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath:                      githubtest.OK(privateRepositoryBody),
			"GET " + privateForkPullRequestWorkflowsPath: githubtest.OK(body),
		}))
		c := New(testToken, WithBaseURL(s.URL()))

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaPrivateForkPullRequestWorkflows)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := &PrivateForkPullRequestWorkflows{Conditions: Observed(setting.PrivateForkPullRequestWorkflowConditions{
			RunWorkflowsFromForkPullRequests: true, RequireApprovalForForkPullRequestWorkflows: true,
		})}
		if !reflect.DeepEqual(observed.PrivateForkPullRequestWorkflows, want) {
			t.Errorf("ObserveRepository().PrivateForkPullRequestWorkflows = %+v, want %+v", observed.PrivateForkPullRequestWorkflows, want)
		}
		wantLines := []string{"GET " + repositoryPath, "GET " + privateForkPullRequestWorkflowsPath}
		if got := s.RequestLines(); !reflect.DeepEqual(got, wantLines) {
			t.Errorf("requests = %q, want %q", got, wantLines)
		}
	})

	t.Run("public repository is unavailable without the read request", func(t *testing.T) {
		s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
			"GET " + repositoryPath: githubtest.OK(repositoryBody),
		}))
		c := New(testToken, WithBaseURL(s.URL()))

		observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaPrivateForkPullRequestWorkflows)
		if err != nil {
			t.Fatalf("ObserveRepository() error = %v", err)
		}
		want := &PrivateForkPullRequestWorkflows{Conditions: Unavailable[setting.PrivateForkPullRequestWorkflowConditions](UnavailablePublicRepository)}
		if !reflect.DeepEqual(observed.PrivateForkPullRequestWorkflows, want) {
			t.Errorf("ObserveRepository().PrivateForkPullRequestWorkflows = %+v, want %+v", observed.PrivateForkPullRequestWorkflows, want)
		}
		wantLines := []string{"GET " + repositoryPath}
		if got := s.RequestLines(); !reflect.DeepEqual(got, wantLines) {
			t.Errorf("requests = %q, want %q", got, wantLines)
		}
	})

	// 失敗の種類の判定規則は TestClassifyFailure が確かめるため、403 と 404 は承認ポリシーの取得で確かめ、ここでは公開範囲に関わる 422 などに絞る
	failures := []struct {
		name     string
		response githubtest.Response
		want     error
	}{
		{name: "422 after observing private is unprocessable", response: githubtest.Response{Status: http.StatusUnprocessableEntity}, want: ErrUnprocessable},
		{name: "301 is redirect", response: githubtest.Response{Status: http.StatusMovedPermanently}, want: ErrRedirect},
		{name: "missing condition is unexpected response", response: githubtest.OK(`{}`), want: ErrUnexpectedResponse},
	}
	for _, tc := range failures {
		t.Run(tc.name, func(t *testing.T) {
			s := githubtest.NewServer(t, githubtest.Routes(t, map[string]githubtest.Response{
				"GET " + repositoryPath:                      githubtest.OK(privateRepositoryBody),
				"GET " + privateForkPullRequestWorkflowsPath: tc.response,
			}))
			c := New(testToken, WithBaseURL(s.URL()))

			observed, err := c.ObserveRepository(t.Context(), mustReference(t, "owner/repo"), setting.AreaPrivateForkPullRequestWorkflows)
			const wantFetch = "fetch private fork pull request workflows owner/repo"
			if !errors.Is(err, tc.want) || !strings.Contains(err.Error(), wantFetch) {
				t.Fatalf("ObserveRepository() error = %v, want %v from %s", err, tc.want, wantFetch)
			}
			if observed != (ObservationResult{}) {
				t.Errorf("ObserveRepository() = %+v, want the zero value", observed)
			}
		})
	}
}
