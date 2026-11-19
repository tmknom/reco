// Package apply は、apply コマンドの処理の流れと、承認後の変更の実行および結果の表示を持つ。
package apply

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

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
	p, err := changeplan.Generate(ctx, client, cfg.Configuration, cfg.Selection)
	if err != nil {
		return err
	}
	return applyPlan(ctx, cfg, client, p)
}

// applyPlan は、選択したすべての対象リポジトリの論理的な設定変更を確定してから、変更計画の全体に承認を一度だけ求め、宣言順に実行する。
// 適用不能な変更予定が一つでもあれば、利用者がすべての理由を読めるよう変更計画を表示し、承認を求めずに止める。
// executableChanges の拒否は最初の一件で返るため、それより先に変更計画の全体から適用不能を挙げる。
func applyPlan(ctx context.Context, cfg Config, client *github.Client, p changeplan.Plan) error {
	text := changeplandisplay.Text(cfg.Configuration.Path, p)
	if notApplicable := p.NotApplicableError(); notApplicable != nil {
		if _, err := io.WriteString(cfg.Out, text); err != nil {
			return err
		}
		return fmt.Errorf("%w; no changes made", notApplicable)
	}
	repositories, err := executableChanges(p)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(p.Repositories, changeplan.RepositoryPlan.HasPlannedChanges) {
		_, err := io.WriteString(cfg.Out, text)
		return err
	}
	if _, err := io.WriteString(cfg.Out, text+"\n"); err != nil {
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
	records := executeRepositories(ctx, client, repositories, cfg.After)
	writeErr := writeChangeReport(cfg.Out, records)
	return errors.Join(repositoriesError(records), writeErr)
}

// repositoryChanges は、承認の前に確定した、一つの対象リポジトリのリポジトリの計画と、その論理的な設定変更の実行順の並び。
// 変更不要の対象リポジトリは changes が空になる。
type repositoryChanges struct {
	plan    changeplan.RepositoryPlan
	changes []changeplan.LogicalChange
}

// executableChanges は、選択したすべての対象リポジトリについて、論理的な設定変更を実行順で返す。
// 戻り値は変更計画のリポジトリの計画と同じ長さ、同じ並びで、変更不要の対象リポジトリも changes が空の要素として含む。
// 論理的な設定変更にできない変更予定の拒否は、論理的な設定変更を導く changeplan が持つ。
// INV-002 のため、すべての対象リポジトリの更新操作の更新エンドポイントを承認の前に引き、更新要求を持たない設定の更新操作があれば、
// 先の対象リポジトリを更新したあとではなく、更新要求を一つも送る前に止める。戻り値を使わないのは、この呼び出しが引けることの検査だけを目的とするため。
func executableChanges(p changeplan.Plan) ([]repositoryChanges, error) {
	repositories := make([]repositoryChanges, len(p.Repositories))
	for i, r := range p.Repositories {
		changes, err := r.LogicalChanges()
		if err != nil {
			return nil, fmt.Errorf("%w; no changes made", err)
		}
		for _, change := range changes {
			for _, operation := range change.Operations {
				github.UpdateEndpointOf(operation.Key)
			}
		}
		repositories[i] = repositoryChanges{plan: r, changes: changes}
	}
	return repositories, nil
}
