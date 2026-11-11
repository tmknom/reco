package cmd

import (
	"github.com/spf13/cobra"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/show"
)

func newShowCmd(opts ...github.Option) *cobra.Command {
	var repo repoFlag
	c := &cobra.Command{
		Use:     "show",
		Short:   "Show the current settings of a repository",
		Args:    noArgs,
		PreRunE: requiredFlags,
		RunE: func(c *cobra.Command, _ []string) error {
			token, err := selectToken()
			if err != nil {
				return err
			}
			return show.Run(c.Context(), show.Config{
				Token:         token,
				Target:        repo.reference,
				Out:           c.OutOrStdout(),
				GitHubOptions: opts,
			})
		},
	}
	c.Flags().Var(&repo, "repo", "repository to show")
	if err := c.MarkFlagRequired("repo"); err != nil {
		panic(err)
	}
	return c
}
