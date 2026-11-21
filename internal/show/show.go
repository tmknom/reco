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
	observed, err := client.ObserveRepository(ctx, cfg.Target, setting.AreaActionsPermissions, setting.AreaWorkflowPermissions, setting.AreaActionsAccess, setting.AreaEnvironments, setting.AreaDependabot)
	if err != nil {
		return err
	}
	return writeObservationResult(cfg.Out, cfg.Target, observed)
}

// writeObservationResult は観測結果を、CLI 仕様の show の形式で書く。
// observed はすべての設定の領域を読み取った観測の結果に限る。読み取っていなければ nil 参照で止まり、値を補って表示しない。
func writeObservationResult(w io.Writer, target repository.Reference, observed github.ObservationResult) error {
	var b strings.Builder
	fmt.Fprintf(&b, "repositories:\n  - name: %s\n    general:\n      delete_branch_on_merge: %t\n      allow_auto_merge: %t\n",
		target, observed.General.DeleteBranchOnMerge, observed.General.AllowAutoMerge)
	b.WriteString("    actions:\n")
	writeActionsPermissions(&b, *observed.ActionsPermissions)
	fmt.Fprintf(&b, "      workflow:\n        default_workflow_permissions: %s\n        can_approve_pull_request_reviews: %t\n",
		observed.WorkflowPermissions.DefaultWorkflowPermissions, observed.WorkflowPermissions.CanApprovePullRequestReviews)
	writeActionsAccess(&b, *observed.ActionsAccess)
	writeEnvironments(&b, *observed.Environments)
	writeSecurity(&b, *observed.Dependabot)
	// 全文を組み立ててから一度だけ書き、書き込みの失敗の経路を一つにする
	_, err := io.WriteString(w, b.String())
	return err
}

// writeActionsPermissions は actions.permissions を書く。
func writeActionsPermissions(b *strings.Builder, p github.ActionsPermissions) {
	fmt.Fprintf(b, "      permissions:\n        enabled: %t\n", p.Enabled)
	if policy, ok := p.AllowedActions.Value(); ok {
		fmt.Fprintf(b, "        allowed_actions: %s\n", policy)
	}
	if criteria, ok := p.SelectedActions.Value(); ok {
		fmt.Fprintf(b, "        selected_actions:\n          github_owned_allowed: %t\n          verified_allowed: %t\n",
			criteria.GitHubOwnedAllowed, criteria.VerifiedAllowed)
		writePatternsAllowed(b, criteria.PatternsAllowed)
	}
	if required, ok := p.SHAPinningRequired.Value(); ok {
		fmt.Fprintf(b, "        sha_pinning_required: %t\n", required)
	}
}

// writePatternsAllowed は patterns_allowed を、空なら [] で、そうでなければ一行に一つずつ書く。
func writePatternsAllowed(b *strings.Builder, patterns []string) {
	if len(patterns) == 0 {
		b.WriteString("          patterns_allowed: []\n")
		return
	}
	b.WriteString("          patterns_allowed:\n")
	for _, pattern := range patterns {
		fmt.Fprintf(b, "            - %s\n", quote(pattern))
	}
}

// writeActionsAccess は actions.access を書く。
// 公開以外の理由はこの設定に当たらず、公開の文言で黙って示さないよう不変条件の破れとして panic する。
func writeActionsAccess(b *strings.Builder, a github.ActionsAccess) {
	if level, ok := a.AccessLevel.Value(); ok {
		fmt.Fprintf(b, "      access:\n        access_level: %s\n", level)
		return
	}
	if reason := a.AccessLevel.UnavailableReason(); reason != github.UnavailablePublicRepository {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for access_level", int(reason)))
	}
	b.WriteString("      # access:\n      #   access_level: unavailable (the repository is public)\n")
}
