package show

import (
	"strings"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

//trace:checks FR-004 QR-002
func TestWriteForkPullRequests(t *testing.T) {
	cases := []struct {
		name      string
		approval  github.ForkPullRequestApproval
		workflows github.PrivateForkPullRequestWorkflows
		want      string
	}{
		{
			name:      "public repository shows the approval policy as a declarable value and the unavailable conditions as a comment",
			approval:  github.ForkPullRequestApproval{ApprovalPolicy: github.Observed(setting.ForkPullRequestApprovalAllExternalContributors)},
			workflows: github.PrivateForkPullRequestWorkflows{Conditions: github.Unavailable[setting.PrivateForkPullRequestWorkflowConditions](github.UnavailablePublicRepository)},
			want: "      fork_pull_requests:\n" +
				"        approval_policy: all_external_contributors\n" +
				"        # private_repositories: unavailable (the repository is public)\n",
		},
		{
			name:     "private repository keeps every line as a comment under the notice about private_repositories",
			approval: github.ForkPullRequestApproval{ApprovalPolicy: github.Unavailable[setting.ForkPullRequestApprovalPolicy](github.UnavailablePrivateRepository)},
			workflows: github.PrivateForkPullRequestWorkflows{Conditions: github.Observed(setting.PrivateForkPullRequestWorkflowConditions{
				RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true,
			})},
			want: "      # reco does not accept private_repositories in the configuration file yet\n" +
				"      # fork_pull_requests:\n" +
				"      #   approval_policy: unavailable (the repository is private)\n" +
				"      #   private_repositories:\n" +
				"      #     run_workflows_from_fork_pull_requests: true\n" +
				"      #     send_write_tokens_to_workflows: false\n" +
				"      #     send_secrets_and_variables: true\n" +
				"      #     require_approval_for_fork_pr_workflows: false\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			writeForkPullRequests(&b, tc.approval, tc.workflows)
			if b.String() != tc.want {
				t.Errorf("writeForkPullRequests() wrote %q, want %q", b.String(), tc.want)
			}
		})
	}

	panics := []struct {
		name      string
		approval  github.ForkPullRequestApproval
		workflows github.PrivateForkPullRequestWorkflows
	}{
		{
			name:      "approval policy unavailable for a reason other than private panics",
			approval:  github.ForkPullRequestApproval{ApprovalPolicy: github.Unavailable[setting.ForkPullRequestApprovalPolicy](github.UnavailablePublicRepository)},
			workflows: github.PrivateForkPullRequestWorkflows{Conditions: github.Observed(setting.PrivateForkPullRequestWorkflowConditions{})},
		},
		{
			name:      "conditions unavailable for a reason other than public panics",
			approval:  github.ForkPullRequestApproval{ApprovalPolicy: github.Observed(setting.ForkPullRequestApprovalFirstTimeContributors)},
			workflows: github.PrivateForkPullRequestWorkflows{Conditions: github.Unavailable[setting.PrivateForkPullRequestWorkflowConditions](github.UnavailablePrivateRepository)},
		},
		{
			name:      "unavailable conditions on a private repository panic",
			approval:  github.ForkPullRequestApproval{ApprovalPolicy: github.Unavailable[setting.ForkPullRequestApprovalPolicy](github.UnavailablePrivateRepository)},
			workflows: github.PrivateForkPullRequestWorkflows{Conditions: github.Unavailable[setting.PrivateForkPullRequestWorkflowConditions](github.UnavailablePublicRepository)},
		},
		{
			name:      "observed conditions on a public repository panic",
			approval:  github.ForkPullRequestApproval{ApprovalPolicy: github.Observed(setting.ForkPullRequestApprovalFirstTimeContributors)},
			workflows: github.PrivateForkPullRequestWorkflows{Conditions: github.Observed(setting.PrivateForkPullRequestWorkflowConditions{})},
		},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("writeForkPullRequests() did not panic")
				}
			}()
			writeForkPullRequests(&strings.Builder{}, tc.approval, tc.workflows)
		})
	}
}
