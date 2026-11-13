// Package show は、show コマンドの処理の流れと観測結果の表示を持つ。
package show

import (
	"context"
	"fmt"
	"io"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// Config は Run の入力。
type Config struct {
	Token         string
	Target        repository.Reference
	Out           io.Writer
	GitHubOptions []github.Option
}

// Run は対象リポジトリを観測し、観測結果を Out に書く。
func Run(ctx context.Context, cfg Config) error {
	client := github.New(cfg.Token, cfg.GitHubOptions...)
	// 失敗の文言は対象名を含むため、ここでは包み直さない
	observed, err := client.ObserveRepository(ctx, cfg.Target, setting.AreaWorkflowPermissions)
	if err != nil {
		return err
	}
	return writeObservationResult(cfg.Out, cfg.Target, observed)
}

// writeObservationResult は観測結果を CLI 仕様の show の形式で書く。
// Repository 行には GitHub が返す名前ではなく、利用者が指定したリポジトリ指定を出す。
// 観測操作の失敗の文言と同じ名前で対象を示すためで、リダイレクトに追従しないため両者の差は大文字小文字に限られる。
// observed は Workflow permissions を読み取った観測の結果に限る。読み取っていなければ nil 参照で止まり、値を補って表示しない。
func writeObservationResult(w io.Writer, target repository.Reference, observed github.ObservationResult) error {
	_, err := fmt.Fprintf(w,
		"Repository: %s\n\nGeneral:\n  delete_branch_on_merge: %t\n  allow_auto_merge: %t\nWorkflow permissions:\n  default_workflow_permissions: %s\n  can_approve_pull_request_reviews: %t\n",
		target,
		observed.General.DeleteBranchOnMerge,
		observed.General.AllowAutoMerge,
		observed.WorkflowPermissions.DefaultWorkflowPermissions,
		observed.WorkflowPermissions.CanApprovePullRequestReviews,
	)
	return err
}
