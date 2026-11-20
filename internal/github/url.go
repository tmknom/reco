package github

import (
	"net/url"

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

func (c *Client) actionsAccessURL(reference repository.Reference) *url.URL {
	u := c.actionsPermissionsURL(reference)
	u.Path += "/access"
	return u
}
