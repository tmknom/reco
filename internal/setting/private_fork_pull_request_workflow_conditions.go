package setting

// PrivateForkPullRequestWorkflowConditions は、非公開リポジトリの Fork Pull Request ワークフローの実行条件である。4項目の組を一つの値として扱う。
type PrivateForkPullRequestWorkflowConditions struct {
	RunWorkflowsFromForkPullRequests           bool
	SendWriteTokensToWorkflows                 bool
	SendSecretsAndVariables                    bool
	RequireApprovalForForkPullRequestWorkflows bool
}
