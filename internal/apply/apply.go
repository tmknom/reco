// Package apply は、apply コマンドの処理の流れと、承認後の変更の実行および結果の表示を持つ。
package apply

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/changeplandisplay"
	"github.com/tmknom/reco/internal/configuration"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
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

// repositoryChanges は、承認の前に確定した、一つの対象リポジトリのリポジトリの計画と、その論理的な設定変更の実行順の並び。
// 変更不要の対象リポジトリは changes が空になる。
type repositoryChanges struct {
	plan    changeplan.RepositoryPlan
	changes []changeplan.LogicalChange
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
// apply がまだ実行しない変更予定が一つでもあれば、同じく変更計画を表示し、承認を求めずに止める。
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
	if unsupported := unsupportedChangesError(repositories); unsupported != nil {
		if _, err := io.WriteString(cfg.Out, text); err != nil {
			return err
		}
		return fmt.Errorf("%w; no changes made", unsupported)
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

// executableChanges は、選択したすべての対象リポジトリについて、リポジトリの計画とその論理的な設定変更の実行順の並びを組にして返す。
// 戻り値は変更計画のリポジトリの計画と同じ長さ、同じ並びで、変更不要の対象リポジトリも changes が空の要素として含む。
// 論理的な設定変更の導出と、承認の前の更新の宛先の確認は changeplan が持つ。
func executableChanges(p changeplan.Plan) ([]repositoryChanges, error) {
	all, err := p.LogicalChanges()
	if err != nil {
		return nil, fmt.Errorf("%w; no changes made", err)
	}
	repositories := make([]repositoryChanges, len(p.Repositories))
	for i, r := range p.Repositories {
		repositories[i] = repositoryChanges{plan: r, changes: all[i]}
	}
	return repositories, nil
}

// unsupportedChangesError は、apply がまだ実行しない、非公開リポジトリの Fork Pull Request ワークフローの実行条件の更新操作を、
// 利用者がすべてを読めるよう変更計画の並びで挙げた error を返す。なければ nil を返す。
// 実行条件の論理的な設定変更は更新操作を一つだけ持つため、更新操作ごとに挙げれば論理的な設定変更ごとに一つになる。
// 適用不能の拒否と同じく、対象リポジトリと管理の単位の組を "; " でつなぐ。
func unsupportedChangesError(repositories []repositoryChanges) error {
	var found []string
	for _, r := range repositories {
		for _, change := range r.changes {
			for _, operation := range change.Operations {
				if operation.Unit.Key == setting.PrivateForkPullRequestWorkflows {
					found = append(found, fmt.Sprintf("%s: %s is a change that apply does not support yet", r.plan.Target, operation.Unit.String()))
				}
			}
		}
	}
	if len(found) == 0 {
		return nil
	}
	return errors.New("plan cannot be applied: " + strings.Join(found, "; "))
}
