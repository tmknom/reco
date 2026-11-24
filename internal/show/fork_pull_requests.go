package show

import (
	"fmt"
	"strings"

	"github.com/tmknom/reco/internal/github"
)

// writeForkPullRequests は actions.fork_pull_requests を書く。
// 設定宣言がまだ受け付けないため、すべての行を YAML のコメントにし、その前の行で書けないことを示す。
// 公開範囲以外の理由はこれらの設定に当たらず、公開範囲の文言で黙って示さないよう不変条件の破れとして panic する。
func writeForkPullRequests(b *strings.Builder, approval github.ForkPullRequestApproval, workflows github.PrivateForkPullRequestWorkflows) {
	b.WriteString("      # reco does not accept the fork pull request settings in the configuration file yet\n      # fork_pull_requests:\n")
	if policy, ok := approval.ApprovalPolicy.Value(); ok {
		fmt.Fprintf(b, "      #   approval_policy: %s\n", policy)
	} else {
		if reason := approval.ApprovalPolicy.UnavailableReason(); reason != github.UnavailablePrivateRepository {
			panic(fmt.Sprintf("show: undefined unavailable reason %d for approval_policy", int(reason)))
		}
		b.WriteString("      #   approval_policy: unavailable (the repository is private)\n")
	}
	conditions, ok := workflows.Conditions.Value()
	if !ok {
		if reason := workflows.Conditions.UnavailableReason(); reason != github.UnavailablePublicRepository {
			panic(fmt.Sprintf("show: undefined unavailable reason %d for private_repositories", int(reason)))
		}
		b.WriteString("      #   private_repositories: unavailable (the repository is public)\n")
		return
	}
	fmt.Fprintf(b, "      #   private_repositories:\n"+
		"      #     run_workflows_from_fork_pull_requests: %t\n      #     send_write_tokens_to_workflows: %t\n"+
		"      #     send_secrets_and_variables: %t\n      #     require_approval_for_fork_pr_workflows: %t\n",
		conditions.RunWorkflowsFromForkPullRequests, conditions.SendWriteTokensToWorkflows,
		conditions.SendSecretsAndVariables, conditions.RequireApprovalForForkPullRequestWorkflows)
}
