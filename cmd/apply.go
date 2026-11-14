package cmd

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/tmknom/reco/internal/apply"
	"github.com/tmknom/reco/internal/github"
)

func newApplyCmd(env Environment, opts ...github.Option) *cobra.Command {
	var repo repoFlag
	config := newConfigFlag()
	c := &cobra.Command{
		Use:   "apply",
		Short: "Apply the changes that bring the current settings to the configuration after approval",
		Args:  cobra.NoArgs,
		RunE: func(c *cobra.Command, _ []string) error {
			token, err := selectToken(env.Getenv)
			if err != nil {
				return err
			}
			dir, name := resolveConfigPath(env.WorkDir, config.path)
			return apply.Run(c.Context(), apply.Config{
				Token:      token,
				ConfigDir:  os.DirFS(dir),
				ConfigName: name,
				ConfigPath: config.path,
				Selection:  repo.selection(),
				Out:        c.OutOrStdout(),
				Approve: func(ctx context.Context) error {
					return requestApproval(ctx, env.Stdin, env.StdinIsTerminal, c.ErrOrStderr())
				},
				After:         time.After,
				GitHubOptions: opts,
			})
		},
	}
	c.Flags().Var(config, "config", "path to the configuration file")
	c.Flags().Var(&repo, "repo", "declared repository to apply")
	return c
}
