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
	// After は、時間の経過を知らせる。更新要求の間隔と、更新が反映されるのを待つために使う。
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
	r, err := soleRepositoryPlan(p)
	if err != nil {
		return err
	}
	changes, err := executableChanges(r)
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return writeNoChanges(cfg.Out)
	}
	if err := writeChangePlan(cfg.Out, cfg.ConfigPath, r); err != nil {
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
	records := executeChanges(ctx, client, r, changes, cfg.After, &updateInterval{})
	writeErr := writeChangeReport(cfg.Out, r.Target, records)
	return errors.Join(changesError(records, r.Target), writeErr)
}

// ErrMultipleRepositories は、対象リポジトリが2件以上の変更計画を apply が実行できないことを表す。
var ErrMultipleRepositories = errors.New("apply can update only one repository at a time")

// soleRepositoryPlan は、変更計画のただ一つのリポジトリの計画を返す。
// 複数の対象リポジトリの実行と結果の表示を持たないため、2件以上は先頭だけを黙って実行せず error にする。
// リポジトリの計画を持たない変更計画は changeplan が生成しないため、不変条件の破れとして panic する。
func soleRepositoryPlan(p changeplan.Plan) (changeplan.RepositoryPlan, error) {
	switch n := len(p.Repositories); {
	case n == 0:
		panic("apply: change plan has no repository plan")
	case n > 1:
		return changeplan.RepositoryPlan{}, fmt.Errorf("%d repositories are selected: %w; no changes made", n, ErrMultipleRepositories)
	}
	return p.Repositories[0], nil
}

// executableChanges は、リポジトリの計画の論理的な設定変更を実行順で返す。
// 論理的な設定変更にできない変更予定の拒否は、論理的な設定変更を導く changeplan が持つ。
// 更新操作は承認の前にすべて引き、欠けていれば、先の変更を更新したあとではなく、更新要求を一つも送る前に止める。
func executableChanges(r changeplan.RepositoryPlan) ([]changeplan.LogicalChange, error) {
	changes, err := r.LogicalChanges()
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		for _, operation := range change.Operations {
			updateSenderOf(operation.Key)
		}
	}
	return changes, nil
}
