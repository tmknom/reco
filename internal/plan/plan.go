// Package plan は、plan コマンドの処理の流れを持つ。
package plan

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

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
// 適用不能な変更予定があれば、利用者がすべての理由を読めるよう変更計画を最後まで書いてから error を返す。
func Run(ctx context.Context, cfg Config) error {
	client := github.New(cfg.Token, cfg.GitHubOptions...)
	p, err := changeplan.Generate(ctx, client, cfg.Configuration, cfg.Selection)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(cfg.Out, changeplandisplay.Text(cfg.Configuration.Path, p)); err != nil {
		return err
	}
	return notApplicableError(p)
}

// notApplicableError は、変更計画の適用不能な変更予定を、変更計画の並びで対象と設定を挙げた error を返す。なければ nil を返す。
func notApplicableError(p changeplan.Plan) error {
	var found []string
	for _, r := range p.Repositories {
		for _, s := range r.NotApplicableChanges() {
			found = append(found, fmt.Sprintf("%s: %s is not applicable", r.Target, s.Key))
		}
	}
	if len(found) == 0 {
		return nil
	}
	return errors.New("plan cannot be applied: " + strings.Join(found, "; "))
}
