package github

import (
	"net/url"
	"testing"
)

// TestEnvironmentURL は、名前を1つのパス要素として送ることを確かめる。
//
//trace:checks FR-004 FR-011 QR-002
func TestEnvironmentURL(t *testing.T) {
	cases := []struct {
		name            string
		environmentName string
		wantEscaped     string
		wantPath        string
	}{
		{name: "plain name is one path element", environmentName: "production", wantEscaped: "/repos/owner/repo/environments/production", wantPath: "/repos/owner/repo/environments/production"},
		{name: "slash is escaped as %2F", environmentName: "a/b", wantEscaped: "/repos/owner/repo/environments/a%2Fb", wantPath: "/repos/owner/repo/environments/a/b"},
		{name: "space is escaped as %20", environmentName: "a b", wantEscaped: "/repos/owner/repo/environments/a%20b", wantPath: "/repos/owner/repo/environments/a b"},
		{name: "percent is escaped as %25", environmentName: "100%", wantEscaped: "/repos/owner/repo/environments/100%25", wantPath: "/repos/owner/repo/environments/100%"},
		{name: "literal %2F is not decoded", environmentName: "a%2Fb", wantEscaped: "/repos/owner/repo/environments/a%252Fb", wantPath: "/repos/owner/repo/environments/a%2Fb"},
		{name: "question mark is escaped as %3F", environmentName: "a?b", wantEscaped: "/repos/owner/repo/environments/a%3Fb", wantPath: "/repos/owner/repo/environments/a?b"},
		{name: "hash is escaped as %23", environmentName: "a#b", wantEscaped: "/repos/owner/repo/environments/a%23b", wantPath: "/repos/owner/repo/environments/a#b"},
		{name: "dots inside a name are kept", environmentName: "a..b", wantEscaped: "/repos/owner/repo/environments/a..b", wantPath: "/repos/owner/repo/environments/a..b"},
	}
	c := New(testToken)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, ok := c.environmentURL(mustReference(t, "owner/repo"), tc.environmentName)
			if !ok {
				t.Fatalf("environmentURL(%q) ok = false, want true", tc.environmentName)
			}
			if got := u.EscapedPath(); got != tc.wantEscaped {
				t.Errorf("environmentURL(%q) escaped path = %q, want %q", tc.environmentName, got, tc.wantEscaped)
			}
			if u.Path != tc.wantPath {
				t.Errorf("environmentURL(%q) path = %q, want %q", tc.environmentName, u.Path, tc.wantPath)
			}
			if u.Host != "api.github.com" || u.Scheme != "https" {
				t.Errorf("environmentURL(%q) = %q, want the client origin", tc.environmentName, u)
			}
		})
	}

	for _, name := range []string{".", ".."} {
		t.Run("dot segment "+name+" is not addressable", func(t *testing.T) {
			if u, ok := c.environmentURL(mustReference(t, "owner/repo"), name); ok {
				t.Errorf("environmentURL(%q) = %q, true, want false", name, u)
			}
		})
	}
}

// TestDeploymentBranchPoliciesURL は、Environment の URL に接尾を足し、組めない名前を伝えることだけを確かめる。名前のエスケープは TestEnvironmentURL が確かめる。
//
//trace:checks FR-004
func TestDeploymentBranchPoliciesURL(t *testing.T) {
	c := New(testToken)
	u, ok := c.deploymentBranchPoliciesURL(mustReference(t, "owner/repo"), "a/b")
	if !ok {
		t.Fatal("deploymentBranchPoliciesURL(\"a/b\") ok = false, want true")
	}
	if got, want := u.EscapedPath(), "/repos/owner/repo/environments/a%2Fb/deployment-branch-policies"; got != want {
		t.Errorf("escaped path = %q, want %q", got, want)
	}
	if got, want := u.Path, "/repos/owner/repo/environments/a/b/deployment-branch-policies"; got != want {
		t.Errorf("path = %q, want %q", got, want)
	}

	if u, ok := c.deploymentBranchPoliciesURL(mustReference(t, "owner/repo"), ".."); ok {
		t.Errorf("deploymentBranchPoliciesURL(\"..\") = %q, true, want false", u)
	}
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
