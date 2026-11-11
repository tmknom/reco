// Package cmd は CLI 層。コマンドを組み立て、実行結果から終了コードを決める。
package cmd

import (
	"context"
	"io"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/logger"
)

const exitOK = 0

// ExitFailure は、処理の失敗を表す終了コード。
const ExitFailure = 1

// devVersion は、バージョンが埋め込まれていない実行ファイルが --version で表示する値。
// cobra は Version が空だと --version を登録しないため、空のままにしない。
const devVersion = "dev"

// Environment は、CLI の実行環境に由来する依存。
type Environment struct {
	Stdout io.Writer
	Stderr io.Writer
	// Version は、実行ファイルに埋め込まれたバージョン。埋め込まれていなければ空。
	Version string
	// WorkDir は、相対パスの基準にする起動時の作業ディレクトリ。
	WorkDir string
}

// Execute は args を実行し、終了コードを返す。
func Execute(ctx context.Context, args []string, env Environment) int {
	return execute(ctx, args, env)
}

func execute(ctx context.Context, args []string, env Environment, opts ...github.Option) int {
	level := new(slog.LevelVar)
	log := logger.New(env.Stderr, level)

	root := newRootCmd(env.Version, env.WorkDir, level, opts...) //nolint:contextcheck // サブコマンドは ExecuteContext に渡した ctx を c.Context() で受け取るため、組み立てに ctx を渡さない
	root.SetArgs(args)
	root.SetOut(env.Stdout)
	root.SetErr(env.Stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		// slog は error+4 のように ERROR より高いレベルも受理する。そのレベルでもエラー理由を省略しない
		level.Set(min(level.Level(), slog.LevelError))
		log.Error("command failed", "error", err)
		return ExitFailure
	}
	return exitOK
}

func newRootCmd(version, workDir string, level *slog.LevelVar, opts ...github.Option) *cobra.Command {
	if version == "" {
		version = devVersion
	}
	root := &cobra.Command{
		Use:           "reco",
		Short:         "Declare GitHub repository settings in YAML, review the diff, and apply it",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().TextVar(level, "log-level", level, "diagnostic log `LEVEL`: debug, info, warn, or error")
	root.AddCommand(newPlanCmd(workDir, opts...))
	root.AddCommand(newShowCmd(opts...))
	return root
}
