package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/tmknom/reco/internal/github"
)

const (
	ghEnvValue     = "gh-token-value"
	githubEnvValue = "github-token-value"
)

const (
	repositoryBody          = `{"id": 1, "allow_auto_merge": true, "delete_branch_on_merge": false}`
	workflowPermissionsBody = `{"default_workflow_permissions": "read", "can_approve_pull_request_reviews": false}`
)

// receivedRequest は、GitHub の stub が受け取った要求のうちテストが確かめる値。
type receivedRequest struct {
	method        string
	path          string
	authorization string
}

// gitHubServer は、受け取った要求を記録する GitHub の stub。
type gitHubServer struct {
	mu       sync.Mutex
	requests []receivedRequest
}

// stubResponse は、GitHub の stub が 1 つの要求に返す応答。
type stubResponse struct {
	status int
	body   string
}

// startGitHubServer は、respond が n 番目 (1 から数える) の要求に決めた応答を返す GitHub の stub を立てる。
func startGitHubServer(t *testing.T, respond func(n int, r *http.Request) stubResponse) (*gitHubServer, []github.Option) {
	t.Helper()
	s := &gitHubServer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, receivedRequest{method: r.Method, path: r.URL.Path, authorization: r.Header.Get("Authorization")})
		n := len(s.requests)
		s.mu.Unlock()
		resp := respond(n, r)
		w.WriteHeader(resp.status)
		_, _ = w.Write([]byte(resp.body))
	}))
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return s, []github.Option{github.WithBaseURL(u)}
}

// newGitHubServer は、どの要求にも同じ応答を返す GitHub の stub を立てる。
func newGitHubServer(t *testing.T, status int, body string) (*gitHubServer, []github.Option) {
	t.Helper()
	return startGitHubServer(t, func(int, *http.Request) stubResponse {
		return stubResponse{status: status, body: body}
	})
}

// newGitHubServerSequence は、要求の順に responses を返す GitHub の stub を立てる。
// 同じ資源への要求ごとに異なる応答が要るテストのためのもの。
func newGitHubServerSequence(t *testing.T, responses ...stubResponse) (*gitHubServer, []github.Option) {
	t.Helper()
	return startGitHubServer(t, func(n int, r *http.Request) stubResponse {
		if n > len(responses) {
			t.Errorf("got request %d %s %s, want at most %d requests", n, r.Method, r.URL.Path, len(responses))
			return stubResponse{status: http.StatusInternalServerError}
		}
		return responses[n-1]
	})
}

// newGitHubServerByPath は、要求のパスごとに responses の応答を返す GitHub の stub を立てる。
// 一回の観測が複数の資源を読み取るテストのためのもの。
func newGitHubServerByPath(t *testing.T, responses map[string]stubResponse) (*gitHubServer, []github.Option) {
	t.Helper()
	return startGitHubServer(t, func(_ int, r *http.Request) stubResponse {
		resp, ok := responses[r.URL.Path]
		if !ok {
			t.Errorf("got request %s %s, want one of the stubbed paths", r.Method, r.URL.Path)
			return stubResponse{status: http.StatusInternalServerError}
		}
		return resp
	})
}

func (s *gitHubServer) receivedRequests() []receivedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]receivedRequest(nil), s.requests...)
}
