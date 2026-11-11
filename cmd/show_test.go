package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
)

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

			code, stdout, stderr := runExecute(t, Environment{}, opts, tc.args...)
			if code != 0 {
				t.Errorf("got exit code %d, want 0 (stderr %q)", code, stderr)
			}
			if !strings.Contains(stdout, "allow_auto_merge: true") {
				t.Errorf("got stdout %q, want it to contain the observed allow_auto_merge", stdout)
			}
			if stderr != "" {
				t.Errorf("got stderr %q, want empty", stderr)
			}
			if got := server.receivedRequests(); len(got) != 1 || !strings.Contains(got[0].authorization, ghEnvValue) {
				t.Errorf("got requests %+v, want one request with GH_TOKEN", got)
			}
			for _, token := range []string{ghEnvValue, githubEnvValue} {
				if strings.Contains(stdout+stderr, token) {
					t.Errorf("got output containing token %q", token)
				}
			}
		})
	}
}

func TestShow_InvalidInput(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "missing --repo", args: []string{"show"}},
		{name: "empty --repo", args: []string{"show", "--repo", ""}},
		{name: "--config is not defined for show", args: []string{"show", "--repo", "owner/repo", "--config", "reco.yaml"}},
		{name: "extra positional argument", args: []string{"show", "--repo", "owner/repo", "extra"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GH_TOKEN", ghEnvValue)
			t.Setenv("GITHUB_TOKEN", "")
			server, opts := newGitHubServer(t, http.StatusOK, repositoryBody)

			code, stdout, stderr := runExecute(t, Environment{}, opts, tc.args...)
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
			if got := server.receivedRequests(); len(got) != 0 {
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

	code, stdout, stderr := runExecute(t, Environment{}, opts, "show", "--repo", "owner/repo")
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
	if got := server.receivedRequests(); len(got) != 0 {
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

	code, stdout, stderr := runExecute(t, Environment{}, opts, "show", "--repo", "owner/repo")
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
	if got := server.receivedRequests(); len(got) != 1 || !strings.Contains(got[0].authorization, ghEnvValue) {
		t.Errorf("got requests %+v, want one request with GH_TOKEN and no retry with GITHUB_TOKEN", got)
	}
	for _, token := range []string{ghEnvValue, githubEnvValue} {
		if strings.Contains(stdout+stderr, token) {
			t.Errorf("got output containing token %q", token)
		}
	}
}
