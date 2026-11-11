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
	change, ok, err := logicalChange(p)
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
	report := executeChange(ctx, client, p, change)
	writeErr := writeChangeReport(cfg.Out, p.Target, change, report)
	return errors.Join(report.err(p.Target, change), writeErr)
}

// logicalChange は、変更計画から適用する論理的な設定変更を一つ取り出す。変更が不要なら ok は偽になる。
func logicalChange(p changeplan.Plan) (change changeplan.SettingPlan, ok bool, err error) {
	var changes []changeplan.SettingPlan
	for _, s := range p.Settings {
		if s.Result == changeplan.Match {
			continue
		}
		if s.Applicability != changeplan.Applicable {
			return changeplan.SettingPlan{}, false, fmt.Errorf("%s: %s cannot be applied", p.Target, s.Setting)
		}
		changes = append(changes, s)
	}
	if len(changes) == 0 {
		return changeplan.SettingPlan{}, false, nil
	}
	// 変更の実行は、承認時の現在値が観測できた allow_auto_merge の 1 件だけを更新する。
	// changeplan.Generate が作る計画はこの形に収まるが、承認したあとで実行できないと分かるより前に止めるため、
	// 不変条件の破れとして panic せず、承認を求める前の error にする。
	if len(changes) > 1 || changes[0].Setting != changeplan.SettingAllowAutoMerge || changes[0].Result != changeplan.Mismatch {
		return changeplan.SettingPlan{}, false, fmt.Errorf("%s: only one known change of %s can be applied", p.Target, changeplan.SettingAllowAutoMerge)
	}
	return changes[0], true, nil
}

// writeChangePlan は、承認を求める変更計画を CLI 仕様の apply の形式で書く。
func writeChangePlan(w io.Writer, configPath string, target repository.Reference, change changeplan.SettingPlan) error {
	_, err := fmt.Fprintf(w, "Configuration: %s\nRepository: %s\n  %s: %t -> %t\n\n",
		configPath, target, change.Setting, change.Current, change.Desired)
	return err
}

func writeNoChanges(w io.Writer) error {
	_, err := io.WriteString(w, "No changes.\nRepositories checked: 1\n")
	return err
}
