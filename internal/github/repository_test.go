package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tmknom/reco/internal/repository"
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
		want := Repository{ID: 1296269, General: General{AllowAutoMerge: true, DeleteBranchOnMerge: false}}
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

// TestFetchRepository は、送信する要求と、応答から観測値までのつなぎ目を確かめる。
//
//trace:checks FR-004
func TestFetchRepository(t *testing.T) {
	var gotMethod, gotPath string
	var gotHeader http.Header
	var gotContentLength int64
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotHeader, gotContentLength = r.Method, r.URL.Path, r.Header.Clone(), r.ContentLength
		_, _ = w.Write([]byte(repositoryBody))
	})

	repo, err := c.FetchRepository(t.Context(), mustReference(t, "owner/repo"))
	if err != nil {
		t.Fatalf("FetchRepository() error = %v", err)
	}
	want := Repository{ID: 1296269, General: General{AllowAutoMerge: true, DeleteBranchOnMerge: false}}
	if repo != want {
		t.Errorf("FetchRepository() = %+v, want %+v", repo, want)
	}

	if gotMethod != http.MethodGet || gotPath != "/repos/owner/repo" {
		t.Errorf("request = %s %s, want GET /repos/owner/repo", gotMethod, gotPath)
	}
	if gotContentLength != 0 || gotHeader.Get("Content-Type") != "" {
		t.Errorf("GET request has Content-Length %d and Content-Type %q, want no body", gotContentLength, gotHeader.Get("Content-Type"))
	}
	headers := map[string]string{
		"Authorization":        "Bearer " + testToken,
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2026-03-10",
		"User-Agent":           userAgent,
	}
	for name, value := range headers {
		if v := gotHeader.Get(name); v != value {
			t.Errorf("header %s = %q, want %q", name, v, value)
		}
	}
}

// TestFetchRepositoryFailure は、失敗応答が種類を保って呼び出し元に届き、
// エラー文言にトークン、応答本文、転送先を含めないことを確かめる。種類の判定規則は TestClassifyFailure が確かめる。
//
//trace:checks FR-003
//trace:checks FR-004
//trace:checks QR-001
func TestFetchRepositoryFailure(t *testing.T) {
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
			_, err := c.FetchRepository(t.Context(), mustReference(t, "owner/repo"))
			if !errors.Is(err, tc.want) {
				t.Fatalf("FetchRepository() error = %v, want %v", err, tc.want)
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
		_, err := c.FetchRepository(ctx, mustReference(t, "owner/repo"))
		if !errors.Is(err, ErrCommunicationFailure) || !errors.Is(err, context.Canceled) {
			t.Fatalf("FetchRepository() error = %v, want %v wrapping %v", err, ErrCommunicationFailure, context.Canceled)
		}
		if strings.Contains(err.Error(), testToken) {
			t.Errorf("error = %q, must not contain the token", err)
		}
	})
}
