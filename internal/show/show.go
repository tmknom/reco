// Package show は、show コマンドの処理の流れと観測結果の表示を持つ。
package show

import (
	"context"
	"fmt"
	"io"
	"strings"

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
	observed, err := client.ObserveRepository(ctx, cfg.Target, setting.AreaActionsPermissions, setting.AreaWorkflowPermissions, setting.AreaActionsAccess)
	if err != nil {
		return err
	}
	return writeObservationResult(cfg.Out, cfg.Target, observed)
}

// writeObservationResult は観測結果を CLI 仕様の show の形式で書く。
// Repository 行には GitHub が返す名前ではなく、利用者が指定したリポジトリ指定を出す。
// 観測操作の失敗の文言と同じ名前で対象を示すためで、リダイレクトに追従しないため両者の差は大文字小文字に限られる。
// observed はすべての設定の領域を読み取った観測の結果に限る。読み取っていなければ nil 参照で止まり、値を補って表示しない。
func writeObservationResult(w io.Writer, target repository.Reference, observed github.ObservationResult) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Repository: %s\n\nGeneral:\n  delete_branch_on_merge: %t\n  allow_auto_merge: %t\n",
		target, observed.General.DeleteBranchOnMerge, observed.General.AllowAutoMerge)
	writeActionsPermissions(&b, *observed.ActionsPermissions)
	fmt.Fprintf(&b, "Workflow permissions:\n  default_workflow_permissions: %s\n  can_approve_pull_request_reviews: %t\n",
		observed.WorkflowPermissions.DefaultWorkflowPermissions, observed.WorkflowPermissions.CanApprovePullRequestReviews)
	writeActionsAccess(&b, *observed.ActionsAccess)
	// 全文を組み立ててから一度だけ書き、書き込みの失敗の経路を一つにする
	_, err := io.WriteString(w, b.String())
	return err
}

// writeActionsPermissions は Actions permissions の節を書く。
func writeActionsPermissions(b *strings.Builder, p github.ActionsPermissions) {
	fmt.Fprintf(b, "Actions permissions:\n  enabled: %t\n", p.Enabled)
	if policy, ok := p.AllowedActions.Value(); ok {
		fmt.Fprintf(b, "  allowed_actions: %s\n", policy)
	}
	if criteria, ok := p.SelectedActions.Value(); ok {
		fmt.Fprintf(b, "  selected_actions:\n    github_owned_allowed: %t\n    verified_allowed: %t\n    patterns_allowed: %s\n",
			criteria.GitHubOwnedAllowed, criteria.VerifiedAllowed, criteria.PatternsAllowedString())
	}
	if required, ok := p.SHAPinningRequired.Value(); ok {
		fmt.Fprintf(b, "  sha_pinning_required: %t\n", required)
	}
}

// writeActionsAccess は Actions access の節を書く。
// 公開以外の理由はこの設定に当たらず、公開の文言で黙って示さないよう不変条件の破れとして panic する。
func writeActionsAccess(b *strings.Builder, a github.ActionsAccess) {
	b.WriteString("Actions access:\n")
	if level, ok := a.AccessLevel.Value(); ok {
		fmt.Fprintf(b, "  access_level: %s\n", level)
		return
	}
	if reason := a.AccessLevel.UnavailableReason(); reason != github.UnavailablePublicRepository {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for access_level", int(reason)))
	}
	b.WriteString("  access_level: unavailable (the repository is public)\n")
}
