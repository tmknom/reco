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

const repositoryBody = `{"id": 1, "allow_auto_merge": true, "delete_branch_on_merge": false}`

// receivedRequest は、GitHub の stub が受け取った要求のうちテストが確かめる値。
type receivedRequest struct {
	method        string
	path          string
	authorization string
}

// gitHubServer は、応答を固定し、受け取った要求を記録する GitHub の stub。
type gitHubServer struct {
	mu       sync.Mutex
	requests []receivedRequest
}

func newGitHubServer(t *testing.T, status int, body string) (*gitHubServer, []github.Option) {
	t.Helper()
	s := &gitHubServer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, receivedRequest{method: r.Method, path: r.URL.Path, authorization: r.Header.Get("Authorization")})
		s.mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	return s, []github.Option{github.WithBaseURL(u)}
}

func (s *gitHubServer) receivedRequests() []receivedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]receivedRequest(nil), s.requests...)
}
