// Package plan は、plan コマンドの処理の流れを持つ。
package plan

import (
	"context"
	"io"
	"io/fs"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/changeplandisplay"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

// Config は Run の入力。
type Config struct {
	Token string
	// ConfigDir は設定ファイルのあるディレクトリ、ConfigName はその中のファイル名。
	ConfigDir  fs.FS
	ConfigName string
	// ConfigPath は利用者が指定したままの設定ファイルのパス。表示とエラー文言に使う。
	ConfigPath string
	// Selection は利用者が明示したリポジトリ指定。明示しなければ nil。
	Selection     *repository.Reference
	Out           io.Writer
	GitHubOptions []github.Option
}

// Run は変更計画を生成し、Out に書く。GitHub の設定は更新しない。
func Run(ctx context.Context, cfg Config) error {
	client := github.New(cfg.Token, cfg.GitHubOptions...)
	file := changeplan.ConfigurationFile{Dir: cfg.ConfigDir, Name: cfg.ConfigName, Path: cfg.ConfigPath}
	p, err := changeplan.Generate(ctx, client, file, cfg.Selection)
	if err != nil {
		return err
	}
	_, err = io.WriteString(cfg.Out, changeplandisplay.ChangePlanText(cfg.ConfigPath, p))
	return err
}
