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
	observed, err := client.ObserveRepository(ctx, cfg.Target, setting.AreaActionsPermissions, setting.AreaWorkflowPermissions, setting.AreaActionsAccess, setting.AreaForkPullRequestApproval, setting.AreaPrivateForkPullRequestWorkflows, setting.AreaRetention, setting.AreaOIDCSubject, setting.AreaRepositoryVariables, setting.AreaEnvironments, setting.AreaEnvironmentVariables, setting.AreaDependabot, setting.AreaSecretScanning, setting.AreaPrivateVulnerabilityReporting, setting.AreaRulesets)
	if err != nil {
		return err
	}
	return writeObservationResult(cfg.Out, cfg.Target, observed)
}

// writeObservationResult は観測結果を、表示仕様の show の形式で書く。
// observed はすべての設定の領域を読み取った観測の結果に限る。読み取っていなければ nil 参照で止まり、値を補って表示しない。
func writeObservationResult(w io.Writer, target repository.Reference, observed github.ObservationResult) error {
	var b strings.Builder
	fmt.Fprintf(&b, "repositories:\n  - name: %s\n", target)
	writeGeneral(&b, observed.General)
	b.WriteString("    actions:\n")
	writeActionsPermissions(&b, *observed.ActionsPermissions)
	writeWorkflowPermissions(&b, *observed.WorkflowPermissions)
	writeActionsAccess(&b, *observed.ActionsAccess)
	writeForkPullRequestApproval(&b, *observed.ForkPullRequestApproval)
	writePrivateForkPullRequestWorkflows(&b, *observed.PrivateForkPullRequestWorkflows)
	writeRetention(&b, *observed.Retention)
	writeOIDCSubject(&b, *observed.OIDCSubject)
	writeRepositoryVariables(&b, *observed.RepositoryVariables)
	writeEnvironments(&b, *observed.Environments)
	writeSecurity(&b, *observed.Dependabot, *observed.SecretScanning, *observed.PrivateVulnerabilityReporting)
	writeRulesets(&b, *observed.Rulesets)
	// 全文を組み立ててから一度だけ書き、書き込みの失敗の経路を一つにする
	_, err := io.WriteString(w, b.String())
	return err
}

// writeActionsPermissions は actions.permissions を書く。
func writeActionsPermissions(b *strings.Builder, p github.ActionsPermissions) {
	fmt.Fprintf(b, "      permissions:\n        enabled: %t\n", p.Enabled)
	if policy, ok := p.AllowedActions.Value(); ok {
		fmt.Fprintf(b, "        allowed_actions: %s\n", policy)
	} else {
		writeUnavailableAllowedActions(b, p.AllowedActions.UnavailableReason())
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

// writeUnavailableAllowedActions は、観測不能の Actions 許可ポリシーを、理由 reason に応じて書く。
// Actions が無効なら、その状態が enabled の行から分かるため、行を示さない。知らない値なら、行をコメントにする。
// selected_actions は、どちらの理由でも確認できない理由が上の行から分かるため、示さない。
// ほかの理由はこの設定に当たらず、行を黙って省かないよう不変条件の破れとして panic する。
func writeUnavailableAllowedActions(b *strings.Builder, reason github.UnavailableReason) {
	switch reason {
	case github.UnavailableActionsDisabled:
	case github.UnavailableUnknownValue:
		fmt.Fprintf(b, "        # allowed_actions: %s\n", unknownValueText)
	default:
		panic(fmt.Sprintf("show: undefined unavailable reason %d for allowed_actions", int(reason)))
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

// writeWorkflowPermissions は actions.workflow_permissions を書く。
func writeWorkflowPermissions(b *strings.Builder, w github.WorkflowPermissions) {
	b.WriteString("      workflow_permissions:\n")
	if level, ok := w.DefaultWorkflowPermissions.Value(); ok {
		fmt.Fprintf(b, "        default_workflow_permissions: %s\n", level)
	} else {
		requireUnknownValue("default_workflow_permissions", w.DefaultWorkflowPermissions.UnavailableReason())
		fmt.Fprintf(b, "        # default_workflow_permissions: %s\n", unknownValueText)
	}
	fmt.Fprintf(b, "        can_approve_pull_request_reviews: %t\n", w.CanApprovePullRequestReviews)
}

// writeActionsAccess は actions.access を書く。
// 公開と知らない値以外の理由はこの設定に当たらず、それらの文言で黙って示さないよう不変条件の破れとして panic する。
func writeActionsAccess(b *strings.Builder, a github.ActionsAccess) {
	if level, ok := a.AccessLevel.Value(); ok {
		fmt.Fprintf(b, "      access:\n        access_level: %s\n", level)
		return
	}
	switch reason := a.AccessLevel.UnavailableReason(); reason {
	case github.UnavailablePublicRepository:
		b.WriteString("      # access:\n      #   access_level: unavailable (the repository is public)\n")
	case github.UnavailableUnknownValue:
		fmt.Fprintf(b, "      # access:\n      #   access_level: %s\n", unknownValueText)
	default:
		panic(fmt.Sprintf("show: undefined unavailable reason %d for access_level", int(reason)))
	}
}

// writeForkPullRequestApproval は actions.fork_pr_contributor_approval を書く。
// 観測不能なら、設定のない空のマッピングを残さないよう、まとまりごとコメントにする。
// 非公開と知らない値以外の理由はこの設定に当たらず、それらの文言で黙って示さないよう不変条件の破れとして panic する。
func writeForkPullRequestApproval(b *strings.Builder, a github.ForkPullRequestApproval) {
	if policy, ok := a.ApprovalPolicy.Value(); ok {
		fmt.Fprintf(b, "      fork_pr_contributor_approval:\n        approval_policy: %s\n", policy)
		return
	}
	switch reason := a.ApprovalPolicy.UnavailableReason(); reason {
	case github.UnavailablePrivateRepository:
		b.WriteString("      # fork_pr_contributor_approval:\n      #   approval_policy: unavailable (the repository is private)\n")
	case github.UnavailableUnknownValue:
		fmt.Fprintf(b, "      # fork_pr_contributor_approval:\n      #   approval_policy: %s\n", unknownValueText)
	default:
		panic(fmt.Sprintf("show: undefined unavailable reason %d for approval_policy", int(reason)))
	}
}

// writePrivateForkPullRequestWorkflows は actions.fork_pr_workflows_private_repos を書く。
// 公開以外の理由はこの設定に当たらず、その文言で黙って示さないよう不変条件の破れとして panic する。
func writePrivateForkPullRequestWorkflows(b *strings.Builder, w github.PrivateForkPullRequestWorkflows) {
	if conditions, ok := w.Conditions.Value(); ok {
		fmt.Fprintf(b, "      fork_pr_workflows_private_repos:\n"+
			"        run_workflows_from_fork_pull_requests: %t\n        send_write_tokens_to_workflows: %t\n"+
			"        send_secrets_and_variables: %t\n        require_approval_for_fork_pr_workflows: %t\n",
			conditions.RunWorkflowsFromForkPullRequests, conditions.SendWriteTokensToWorkflows,
			conditions.SendSecretsAndVariables, conditions.RequireApprovalForForkPullRequestWorkflows)
		return
	}
	if reason := w.Conditions.UnavailableReason(); reason != github.UnavailablePublicRepository {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for fork_pr_workflows_private_repos", int(reason)))
	}
	b.WriteString("      # fork_pr_workflows_private_repos: unavailable (the repository is public)\n")
}

// writeRetention は actions.artifact_and_log_retention を書く。
// 上限は設定宣言に書けない値のため、値の位置ではなく行末のコメントに書く。
func writeRetention(b *strings.Builder, r github.Retention) {
	fmt.Fprintf(b, "      artifact_and_log_retention:\n        days: %d # maximum allowed: %d\n", r.Days, r.MaximumAllowedDays)
}
