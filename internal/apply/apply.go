// Package apply は、apply コマンドの処理の流れと、承認後の変更の実行および結果の表示を持つ。
package apply

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
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
	Approve       func(ctx context.Context) error
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
	change, ok, err := singleLogicalChange(p)
	if err != nil {
		return err
	}
	if !ok {
		return writeNoChanges(cfg.Out)
	}
	if err := writeChangePlan(cfg.Out, cfg.ConfigPath, p.Target, change); err != nil {
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
	record := executeChange(ctx, client, p, change)
	writeErr := writeChangeReport(cfg.Out, p.Target, change, record)
	return errors.Join(changeError(record, p.Target, change), writeErr)
}

// singleLogicalChange は、変更計画から実行する論理的な設定変更を一つ取り出す。変更が不要なら ok は偽になる。
// 変更の実行は allow_auto_merge の論理的な設定変更 1 件だけを更新する。
// それ以外の論理的な設定変更を持つ変更計画は、承認したあとで実行できないと分かるより前に止めるため、
// 承認を求める前の error にする。
func singleLogicalChange(p changeplan.Plan) (change changeplan.LogicalChange, ok bool, err error) {
	changes, err := p.LogicalChanges()
	if err != nil {
		return changeplan.LogicalChange{}, false, err
	}
	if len(changes) == 0 {
		return changeplan.LogicalChange{}, false, nil
	}
	if len(changes) > 1 || changes[0].Key != setting.AllowAutoMerge {
		return changeplan.LogicalChange{}, false, fmt.Errorf("%s: only one known change of %s can be applied", p.Target, setting.AllowAutoMerge)
	}
	return changes[0], true, nil
}
