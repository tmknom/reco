package github

import (
	"net/url"
	"strconv"

	"github.com/tmknom/reco/internal/repository"
)

// repositoryURL は送信先のオリジンに /repos/{owner}/{repo} を置く。
// url.URL.JoinPath は path が空のオリジンに相対パスを返すため使わない。
func (c *Client) repositoryURL(reference repository.Reference) *url.URL {
	u := *c.baseURL
	u.Path = "/repos/" + reference.Owner() + "/" + reference.Repo()
	return &u
}

func (c *Client) workflowPermissionsURL(reference repository.Reference) *url.URL {
	u := c.repositoryURL(reference)
	u.Path += "/actions/permissions/workflow"
	return u
}

func (c *Client) actionsPermissionsURL(reference repository.Reference) *url.URL {
	u := c.repositoryURL(reference)
	u.Path += "/actions/permissions"
	return u
}

func (c *Client) selectedActionsURL(reference repository.Reference) *url.URL {
	u := c.actionsPermissionsURL(reference)
	u.Path += "/selected-actions"
	return u
}

func (c *Client) environmentsURL(reference repository.Reference) *url.URL {
	u := c.repositoryURL(reference)
	u.Path += "/environments"
	return u
}

// environmentURL は、name の Environment の URL を返す。
// name は1つのパス要素として送るため、Path と RawPath の両方を設定し、`/` を `%2F` のまま送る。Path だけでは `/` が区切りとして送られる。
// `.` と `..` は、url.PathEscape がそのまま残し、パスの要素としては URL の解決で dot-segment として取り除かれ得る (RFC 3986 5.2.4) ため、偽を返す。
func (c *Client) environmentURL(reference repository.Reference, name string) (*url.URL, bool) {
	if name == "." || name == ".." {
		return nil, false
	}
	u := c.environmentsURL(reference)
	u.RawPath = u.Path + "/" + url.PathEscape(name)
	u.Path += "/" + name
	return u, true
}

// deploymentBranchPoliciesURL は、name の Environment の個別パターンの一覧の URL を返す。name が environmentURL で組めなければ偽を返す。
func (c *Client) deploymentBranchPoliciesURL(reference repository.Reference, name string) (*url.URL, bool) {
	u, ok := c.environmentURL(reference, name)
	if !ok {
		return nil, false
	}
	u.RawPath += "/deployment-branch-policies"
	u.Path += "/deployment-branch-policies"
	return u, true
}

// deploymentBranchPolicyURL は、name の Environment の ID が id の個別パターンの URL を返す。name が environmentURL で組めなければ偽を返す。
func (c *Client) deploymentBranchPolicyURL(reference repository.Reference, name string, id DeploymentPatternID) (*url.URL, bool) {
	u, ok := c.deploymentBranchPoliciesURL(reference, name)
	if !ok {
		return nil, false
	}
	suffix := "/" + strconv.FormatInt(int64(id), 10)
	u.RawPath += suffix
	u.Path += suffix
	return u, true
}

func (c *Client) actionsAccessURL(reference repository.Reference) *url.URL {
	u := c.actionsPermissionsURL(reference)
	u.Path += "/access"
	return u
}
