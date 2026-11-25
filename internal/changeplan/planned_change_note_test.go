package changeplan

import (
	"slices"
	"testing"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// TestSettingPlan_Notes は、変更予定の注記が、実行条件の不一致のうち、書き込みトークンか Secrets と Variables の送信を
// 効いていない状態から効いている状態に変える項目にだけ、項目の並びで付くことを確かめる。
//
//trace:checks FR-007
func TestSettingPlan_Notes(t *testing.T) {
	type conditions = setting.PrivateForkPullRequestWorkflowConditions
	plan := func(observed, desired conditions, result ReconciliationResult) SettingPlan {
		return SettingPlan{
			Unit:     setting.Unit{Key: setting.PrivateForkPullRequestWorkflows},
			Observed: github.Observed(setting.PrivateForkPullRequestWorkflowConditionsValue(observed)),
			Desired:  setting.PrivateForkPullRequestWorkflowConditionsValue(desired),
			Result:   result,
		}
	}
	off := conditions{RunWorkflowsFromForkPullRequests: true}
	cases := []struct {
		name string
		plan SettingPlan
		want []PlannedChangeNote
	}{
		{
			name: "enabling write tokens notes them",
			plan: plan(off, conditions{RunWorkflowsFromForkPullRequests: true, SendWriteTokensToWorkflows: true}, Mismatch),
			want: []PlannedChangeNote{NoteForkWorkflowsGetWriteTokens},
		},
		{
			name: "enabling secrets and variables notes them",
			plan: plan(off, conditions{RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true}, Mismatch),
			want: []PlannedChangeNote{NoteForkWorkflowsGetSecretsAndVariables},
		},
		{
			// GitHub は実行を許可しないあいだ送信を false で返すが、reco はこの関係を取得契約にしないため、効いている値で比べる
			name: "enabling runs with transmissions observed as true but not in effect notes both",
			plan: plan(conditions{SendWriteTokensToWorkflows: true, SendSecretsAndVariables: true}, conditions{RunWorkflowsFromForkPullRequests: true, SendWriteTokensToWorkflows: true, SendSecretsAndVariables: true}, Mismatch),
			want: []PlannedChangeNote{NoteForkWorkflowsGetWriteTokens, NoteForkWorkflowsGetSecretsAndVariables},
		},
		{
			name: "keeping write tokens enabled while changing approval notes nothing",
			plan: plan(conditions{RunWorkflowsFromForkPullRequests: true, SendWriteTokensToWorkflows: true}, conditions{RunWorkflowsFromForkPullRequests: true, SendWriteTokensToWorkflows: true, RequireApprovalForForkPullRequestWorkflows: true}, Mismatch),
		},
		{
			name: "keeping secrets and variables enabled while changing approval notes nothing",
			plan: plan(conditions{RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true}, conditions{RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true, RequireApprovalForForkPullRequestWorkflows: true}, Mismatch),
		},
		{
			name: "disabling secrets and variables notes nothing",
			plan: plan(conditions{RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true}, off, Mismatch),
		},
		{
			name: "enabling runs without sending tokens or secrets notes nothing",
			plan: plan(conditions{}, off, Mismatch),
		},
		{
			name: "not comparable conditions on a public repository note nothing",
			plan: SettingPlan{
				Unit:     setting.Unit{Key: setting.PrivateForkPullRequestWorkflows},
				Observed: github.Unavailable[setting.Value](github.UnavailablePublicRepository),
				Desired:  setting.PrivateForkPullRequestWorkflowConditionsValue(conditions{RunWorkflowsFromForkPullRequests: true, SendSecretsAndVariables: true}),
				Result:   NotComparable,
			},
		},
		{
			name: "setting other than the conditions notes nothing",
			plan: SettingPlan{Unit: setting.Unit{Key: setting.CanApprovePullRequestReviews}, Observed: github.Observed(setting.BoolValue(false)), Desired: setting.BoolValue(true), Result: Mismatch},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.plan.Notes(); !slices.Equal(got, tc.want) {
				t.Errorf("Notes() = %v, want %v", got, tc.want)
			}
		})
	}
}
