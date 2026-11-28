package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
)

// writeForkPullRequests は actions.fork_pull_requests を書く。
// 一回の観測では、承認ポリシーと実行条件のちょうど一方だけが公開範囲を理由に観測不能になる。
// 承認ポリシーが知らない値のときは、公開リポジトリだけが承認ポリシーを読むため、実行条件は公開を理由に観測不能になる。
// それ以外の組は公開範囲の文言で黙って示さないよう、不変条件の破れとして panic する。
func writeForkPullRequests(b *strings.Builder, approval github.ForkPullRequestApproval, workflows github.PrivateForkPullRequestWorkflows) {
	if policy, ok := approval.ApprovalPolicy.Value(); ok {
		requirePublicConditions(workflows)
		fmt.Fprintf(b, "      fork_pull_requests:\n        approval_policy: %s\n        # private_repositories: unavailable (the repository is public)\n", policy)
		return
	}
	reason := approval.ApprovalPolicy.UnavailableReason()
	if reason == github.UnavailableUnknownValue {
		requirePublicConditions(workflows)
		fmt.Fprintf(b, "      # fork_pull_requests:\n      #   approval_policy: %s\n      #   private_repositories: unavailable (the repository is public)\n", unknownValueText)
		return
	}
	if reason != github.UnavailablePrivateRepository {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for approval_policy", int(reason)))
	}
	conditions, ok := workflows.Conditions.Value()
	if !ok {
		panic("show: private_repositories is unavailable on a private repository")
	}
	fmt.Fprintf(b, "      fork_pull_requests:\n        # approval_policy: unavailable (the repository is private)\n        private_repositories:\n"+
		"          run_workflows_from_fork_pull_requests: %t\n          send_write_tokens_to_workflows: %t\n"+
		"          send_secrets_and_variables: %t\n          require_approval_for_fork_pr_workflows: %t\n",
		conditions.RunWorkflowsFromForkPullRequests, conditions.SendWriteTokensToWorkflows,
		conditions.SendSecretsAndVariables, conditions.RequireApprovalForForkPullRequestWorkflows)
}

// requirePublicConditions は、実行条件が公開を理由に観測不能であることを確かめ、そうでなければ不変条件の破れとして panic する。
func requirePublicConditions(workflows github.PrivateForkPullRequestWorkflows) {
	if reason := workflows.Conditions.UnavailableReason(); reason != github.UnavailablePublicRepository {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for private_repositories", int(reason)))
	}
}
