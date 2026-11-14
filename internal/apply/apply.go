// Package apply は、apply コマンドの処理の流れと、承認後の変更の実行および結果の表示を持つ。
package apply

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/tmknom/reco/internal/changeplan"
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
	Selection *repository.Reference
	Out       io.Writer
	// Approve は、変更計画を提示したあとに利用者の対話承認を求める。承認されたときだけ nil を返す。
	Approve func(ctx context.Context) error
	// After は、時間の経過を知らせる。更新が反映されるのを待つために使う。
	After         func(d time.Duration) <-chan time.Time
	GitHubOptions []github.Option
}

// Run は変更計画を生成し、承認されたときだけ GitHub の設定を更新して、結果を Out に書く。
func Run(ctx context.Context, cfg Config) error {
	client := github.New(cfg.Token, cfg.GitHubOptions...)
	file := changeplan.ConfigurationFile{Dir: cfg.ConfigDir, Name: cfg.ConfigName, Path: cfg.ConfigPath}
	p, err := changeplan.Generate(ctx, client, file, cfg.Selection)
	if err != nil {
		return err
	}
	return applyPlan(ctx, cfg, client, p)
}

func applyPlan(ctx context.Context, cfg Config, client *github.Client, p changeplan.Plan) error {
	changes, err := executableChanges(p)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return writeNoChanges(cfg.Out)
	}
	if err := writeChangePlan(cfg.Out, cfg.ConfigPath, p.Target, changes); err != nil {
		return err
	}
	if err := cfg.Approve(ctx); err != nil {
		return fmt.Errorf("apply cancelled: %w; no changes made", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("apply cancelled: %w; no changes made", err)
	}
	if _, err := io.WriteString(cfg.Out, "Applying changes...\n"); err != nil {
		return err
	}
	records := executeChanges(ctx, client, p, changes, cfg.After)
	writeErr := writeChangeReport(cfg.Out, p.Target, records)
	return errors.Join(changesError(records, p.Target), writeErr)
}

// executableChanges は、変更計画の論理的な設定変更を実行順で返す。
// 実行できる更新操作を持たないものがあれば、承認したあとで実行できないと分かるより前に止めるため、
// 承認を求める前の error にする。
func executableChanges(p changeplan.Plan) ([]changeplan.LogicalChange, error) {
	changes, err := p.LogicalChanges()
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		if _, ok := updateOperationOf(change.Key); !ok {
			return nil, fmt.Errorf("%s: %s cannot be applied: no update operation", p.Target, change.Key)
		}
	}
	return changes, nil
}
