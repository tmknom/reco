package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/plan"
)

func newPlanCmd(env Environment, opts ...github.Option) *cobra.Command {
	var repo repoFlag
	config := newConfigFlag()
	c := &cobra.Command{
		Use:   "plan",
		Short: "Show the changes that would bring the current settings to the configuration",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			token, err := selectToken(env.Getenv)
			if err != nil {
				return err
			}
			dir, name := resolveConfigPath(env.WorkDir, config.path)
			return plan.Run(c.Context(), plan.Config{
				Token:         token,
				ConfigDir:     os.DirFS(dir),
				ConfigName:    name,
				ConfigPath:    config.path,
				Selection:     repo.selection(),
				Out:           c.OutOrStdout(),
				GitHubOptions: opts,
			})
		},
	}
	c.Flags().Var(config, "config", "path to the configuration file")
	c.Flags().Var(&repo, "repo", "declared repository to plan")
	return c
}
