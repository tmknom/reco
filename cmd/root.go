// Package cmd は CLI 層。コマンドを組み立て、実行結果から終了コードを決める。
package cmd

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/tmknom/reco/internal/logger"
)

const (
	exitOK      = 0
	exitFailure = 1
)

// devVersion は、バージョンが埋め込まれていない実行ファイルが --version で表示する値。
// cobra は Version が空だと --version を登録しないため、空のままにしない。
const devVersion = "dev"

// Environment は、CLI の実行環境に由来する依存。
type Environment struct {
	Stdout io.Writer
	Stderr io.Writer
	// Version は、実行ファイルに埋め込まれたバージョン。埋め込まれていなければ空。
	Version string
}

// Execute は args を実行し、終了コードを返す。
func Execute(ctx context.Context, args []string, env Environment) int {
	level := new(slog.LevelVar)
	log := logger.New(env.Stderr, level)

	root := newRootCmd(env.Version, level)
	root.SetArgs(args)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		log.Error("command failed", "error", err)
		if usage, ok := errors.AsType[*usageError](err); ok {
			// Usage() は SetOut の出力先 (stdout) に書くため、使い方の文字列を stderr へ書く
			usage.cmd.PrintErr(usage.cmd.UsageString())
		}
		return exitFailure
	}
	return exitOK
}

func newRootCmd(version string, level *slog.LevelVar) *cobra.Command {
	if version == "" {
		version = devVersion
	}
	root := &cobra.Command{
		Use:           "reco",
		Short:         "Declare GitHub repository settings in YAML, review the diff, and apply it",
		Version:       version,
		Args:          noArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}
	// CLI 仕様のコマンド一覧が外部契約のため、cobra が自動で加える completion を外す
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(flagUsageError)
	root.PersistentFlags().Var(newLogLevel(level), "log-level", "level of diagnostic logs ("+logLevelChoices()+")")
	return root
}
