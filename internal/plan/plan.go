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
	return presentPlan(cfg, p)
}

// presentPlan は、変更計画 p を Out に書く。
// 適用不能な変更予定があれば、利用者がすべての理由を読めるよう変更計画を最後まで書いてから error を返す。
// なければ、apply が承認の前に行うのと同じく論理的な設定変更を導き、plan が通った変更計画は apply でも承認の要求まで進むようにする。
func presentPlan(cfg Config, p changeplan.Plan) error {
	if _, err := io.WriteString(cfg.Out, changeplandisplay.Text(cfg.Configuration.Path, p)); err != nil {
		return err
	}
	if err := p.NotApplicableError(); err != nil {
		return err
	}
	_, err := p.LogicalChanges()
	return err
}
