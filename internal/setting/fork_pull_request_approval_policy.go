package setting

// ForkPullRequestApprovalPolicy は、Fork Pull Request ワークフローの承認ポリシーが取り得る値。値は GitHub REST API の文字列で、表示にもそのまま使う。
type ForkPullRequestApprovalPolicy string

const (
	ForkPullRequestApprovalFirstTimeContributorsNewToGitHub ForkPullRequestApprovalPolicy = "first_time_contributors_new_to_github"
	ForkPullRequestApprovalFirstTimeContributors            ForkPullRequestApprovalPolicy = "first_time_contributors"
	ForkPullRequestApprovalAllExternalContributors          ForkPullRequestApprovalPolicy = "all_external_contributors"
)

// ParseForkPullRequestApprovalPolicy は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseForkPullRequestApprovalPolicy(s string) (ForkPullRequestApprovalPolicy, bool) {
	switch policy := ForkPullRequestApprovalPolicy(s); policy {
	case ForkPullRequestApprovalFirstTimeContributorsNewToGitHub, ForkPullRequestApprovalFirstTimeContributors, ForkPullRequestApprovalAllExternalContributors:
		return policy, true
	default:
		return "", false
	}
}
