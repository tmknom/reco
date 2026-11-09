package cmd

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/tmknom/reco/internal/github"
)

const (
	ghEnvValue     = "gh-token-value"
	githubEnvValue = "github-token-value"
)

type gitHubServer struct {
	mu             sync.Mutex
	authorizations []string
}

func newGitHubServer(t *testing.T, status int, body string) (*gitHubServer, []github.Option) {
	t.Helper()
	s := &gitHubServer{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.authorizations = append(s.authorizations, r.Header.Get("Authorization"))
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

func (s *gitHubServer) receivedAuthorizations() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.authorizations...)
}

const repositoryBody = `{"id": 1, "allow_auto_merge": true, "delete_branch_on_merge": false}`

//trace:checks FR-003
//trace:checks FR-004
//trace:checks QR-001
func TestShow_Success(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "repository settings are shown", args: []string{"show", "--repo", "owner/repo"}},
		{name: "--log-level after the command is accepted", args: []string{"show", "--log-level", "debug", "--repo", "owner/repo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", ghEnvValue)
			t.Setenv("GITHUB_TOKEN", githubEnvValue)
			server, opts := newGitHubServer(t, http.StatusOK, repositoryBody)

			code, stdout, stderr := runExecute(t, "", opts, tc.args...)
			if code != 0 {
				t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
			}
			if !strings.Contains(stdout, "allow_auto_merge: true") {
				t.Errorf("got stdout %q, want it to contain the observed allow_auto_merge", stdout)
			}
			if stderr != "" {
				t.Errorf("got stderr %q, want empty", stderr)
			}
			if got := server.receivedAuthorizations(); len(got) != 1 || !strings.Contains(got[0], ghEnvValue) {
				t.Errorf("got Authorization %q, want one request with GH_TOKEN", got)
			}
			for _, token := range []string{ghEnvValue, githubEnvValue} {
				if strings.Contains(stdout+stderr, token) {
					t.Errorf("got output containing token %q", token)
				}
			}
		})
	}
}

func TestShow_UsageError(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "missing --repo", args: []string{"show"}},
		{name: "empty --repo", args: []string{"show", "--repo", ""}},
		{name: "--config is not defined for show", args: []string{"show", "--repo", "owner/repo", "--config", "reco.yaml"}},
		{name: "extra positional argument", args: []string{"show", "--repo", "owner/repo", "extra"}},
		{name: "invalid --log-level after the command", args: []string{"show", "--log-level", "INFO", "--repo", "owner/repo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", ghEnvValue)
			t.Setenv("GITHUB_TOKEN", "")
			server, opts := newGitHubServer(t, http.StatusOK, repositoryBody)

			code, stdout, stderr := runExecute(t, "", opts, tc.args...)
			if code != 1 {
				t.Errorf("got exit code %d, want 1", code)
			}
			if stdout != "" {
				t.Errorf("got stdout %q, want empty", stdout)
			}
			records := errorRecords(t, stderr)
			if len(records) != 1 || records[0]["msg"] != "command failed" {
				t.Errorf("got ERROR records %v, want one command failed record", records)
			}
			if !strings.Contains(stderr, "reco show") {
				t.Errorf("got stderr %q, want show usage", stderr)
			}
			if got := server.receivedAuthorizations(); len(got) != 0 {
				t.Errorf("got %d requests, want none", len(got))
			}
		})
	}
}

//trace:checks FR-003
func TestShow_TokenMissing(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	server, opts := newGitHubServer(t, http.StatusOK, repositoryBody)

	code, stdout, stderr := runExecute(t, "", opts, "show", "--repo", "owner/repo")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("got stdout %q, want empty", stdout)
	}
	records := errorRecords(t, stderr)
	if len(records) != 1 || records[0]["msg"] != "command failed" {
		t.Errorf("got ERROR records %v, want one command failed record", records)
	}
	if got := server.receivedAuthorizations(); len(got) != 0 {
		t.Errorf("got %d requests, want none", len(got))
	}
}

// 観測操作の失敗の種類ごとの文言は internal/github が決めて検証しているため、ここでは失敗が結果として表示されず、
// 対象と理由が stderr に届くことを 1 種類で確かめる。
//
//trace:checks FR-003
//trace:checks FR-004
//trace:checks QR-001
func TestShow_ObservationFailure(t *testing.T) {
	t.Setenv("GH_TOKEN", ghEnvValue)
	t.Setenv("GITHUB_TOKEN", githubEnvValue)
	server, opts := newGitHubServer(t, http.StatusUnauthorized, `{"message": "Bad credentials"}`)

	code, stdout, stderr := runExecute(t, "", opts, "show", "--repo", "owner/repo")
	if code != 1 {
		t.Errorf("got exit code %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("got stdout %q, want empty", stdout)
	}
	records := errorRecords(t, stderr)
	if len(records) != 1 || records[0]["msg"] != "command failed" {
		t.Fatalf("got ERROR records %v, want one command failed record", records)
	}
	if msg, _ := records[0]["error"].(string); !strings.Contains(msg, "owner/repo") || !strings.Contains(msg, github.ErrUnauthorized.Error()) {
		t.Errorf("got error %q, want it to contain the target and the authentication failure", msg)
	}
	if got := server.receivedAuthorizations(); len(got) != 1 || !strings.Contains(got[0], ghEnvValue) {
		t.Errorf("got Authorization %q, want one request with GH_TOKEN and no retry with GITHUB_TOKEN", got)
	}
	for _, token := range []string{ghEnvValue, githubEnvValue} {
		if strings.Contains(stdout+stderr, token) {
			t.Errorf("got output containing token %q", token)
		}
	}
}
