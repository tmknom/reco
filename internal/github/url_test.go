package github

import (
	"net/url"
	"testing"
)

// TestDeploymentBranchPoliciesURL は、名前を1つのパス要素として送ることを確かめる。
//
//trace:checks FR-004
//trace:checks QR-002
func TestDeploymentBranchPoliciesURL(t *testing.T) {
	cases := []struct {
		name            string
		environmentName string
		wantEscaped     string
		wantPath        string
	}{
		{name: "plain name is one path element", environmentName: "production", wantEscaped: "/repos/owner/repo/environments/production/deployment-branch-policies", wantPath: "/repos/owner/repo/environments/production/deployment-branch-policies"},
		{name: "slash is escaped as %2F", environmentName: "a/b", wantEscaped: "/repos/owner/repo/environments/a%2Fb/deployment-branch-policies", wantPath: "/repos/owner/repo/environments/a/b/deployment-branch-policies"},
		{name: "dots inside a name are kept", environmentName: "a..b", wantEscaped: "/repos/owner/repo/environments/a..b/deployment-branch-policies", wantPath: "/repos/owner/repo/environments/a..b/deployment-branch-policies"},
	}
	c := New(testToken)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, ok := c.deploymentBranchPoliciesURL(mustReference(t, "owner/repo"), tc.environmentName)
			if !ok {
				t.Fatalf("deploymentBranchPoliciesURL(%q) ok = false, want true", tc.environmentName)
			}
			if got := u.EscapedPath(); got != tc.wantEscaped {
				t.Errorf("deploymentBranchPoliciesURL(%q) escaped path = %q, want %q", tc.environmentName, got, tc.wantEscaped)
			}
			if u.Path != tc.wantPath {
				t.Errorf("deploymentBranchPoliciesURL(%q) path = %q, want %q", tc.environmentName, u.Path, tc.wantPath)
			}
			if u.Host != "api.github.com" || u.Scheme != "https" {
				t.Errorf("deploymentBranchPoliciesURL(%q) = %q, want the client origin", tc.environmentName, u)
			}
		})
	}

	for _, name := range []string{".", ".."} {
		t.Run("dot segment "+name+" is not addressable", func(t *testing.T) {
			if u, ok := c.deploymentBranchPoliciesURL(mustReference(t, "owner/repo"), name); ok {
				t.Errorf("deploymentBranchPoliciesURL(%q) = %q, true, want false", name, u)
			}
		})
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
