package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
)

// writeForkPullRequests は actions.fork_pull_requests を書く。
// 一回の観測では、承認ポリシーと実行条件のちょうど一方だけが公開範囲を理由に観測不能になる。
// それ以外の組は公開範囲の文言で黙って示さないよう、不変条件の破れとして panic する。
func writeForkPullRequests(b *strings.Builder, approval github.ForkPullRequestApproval, workflows github.PrivateForkPullRequestWorkflows) {
	if policy, ok := approval.ApprovalPolicy.Value(); ok {
		if reason := workflows.Conditions.UnavailableReason(); reason != github.UnavailablePublicRepository {
			panic(fmt.Sprintf("show: undefined unavailable reason %d for private_repositories", int(reason)))
		}
		fmt.Fprintf(b, "      fork_pull_requests:\n        approval_policy: %s\n        # private_repositories: unavailable (the repository is public)\n", policy)
		return
	}
	if reason := approval.ApprovalPolicy.UnavailableReason(); reason != github.UnavailablePrivateRepository {
		panic(fmt.Sprintf("show: undefined unavailable reason %d for approval_policy", int(reason)))
	}
	conditions, ok := workflows.Conditions.Value()
	if !ok {
		panic("show: private_repositories is unavailable on a private repository")
	}
	fmt.Fprintf(b, "      # reco does not accept private_repositories in the configuration file yet\n"+
		"      # fork_pull_requests:\n      #   approval_policy: unavailable (the repository is private)\n      #   private_repositories:\n"+
		"      #     run_workflows_from_fork_pull_requests: %t\n      #     send_write_tokens_to_workflows: %t\n"+
		"      #     send_secrets_and_variables: %t\n      #     require_approval_for_fork_pr_workflows: %t\n",
		conditions.RunWorkflowsFromForkPullRequests, conditions.SendWriteTokensToWorkflows,
		conditions.SendSecretsAndVariables, conditions.RequireApprovalForForkPullRequestWorkflows)
}
