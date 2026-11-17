// Package plan は、plan コマンドの処理の流れを持つ。
package plan

import (
	"context"
	"io"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/changeplandisplay"
	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

// Config は Run の入力。
type Config struct {
	Token string
	// Configuration は設定宣言を読み込む設定ファイル。
	Configuration configuration.File
	// Selection は利用者が明示したリポジトリ指定。明示しなければ nil。
	Selection     *repository.Reference
	Out           io.Writer
	GitHubOptions []github.Option
}

// Run は変更計画を生成し、Out に書く。GitHub の設定は更新しない。
func Run(ctx context.Context, cfg Config) error {
	client := github.New(cfg.Token, cfg.GitHubOptions...)
	p, err := changeplan.Generate(ctx, client, cfg.Configuration, cfg.Selection)
	if err != nil {
		return err
	}
	_, err = io.WriteString(cfg.Out, changeplandisplay.Text(cfg.Configuration.Path, p))
	return err
}
