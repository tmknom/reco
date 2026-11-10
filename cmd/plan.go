package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/reco"
)

func newPlanCmd(workDir string, opts ...github.Option) *cobra.Command {
	var repo repoFlag
	config := newConfigFlag()
	c := &cobra.Command{
		Use:   "plan",
		Short: "Show the changes that would bring the current settings to the configuration",
		Args:  noArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			token, err := selectToken()
			if err != nil {
				return err
			}
			dir, name := resolveConfigPath(workDir, config.path)
			return reco.Plan(c.Context(), reco.PlanConfig{
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
