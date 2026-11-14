package apply

import (
	"context"
	"fmt"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// updateEndpoint は、更新操作が更新要求を送る GitHub の更新エンドポイント。
type updateEndpoint int

// ゼロ値を定義済みの更新エンドポイントと取り違えないよう 1 から始める。
const (
	repositoryEndpoint updateEndpoint = iota + 1
	workflowPermissionsEndpoint
	actionsPermissionsEndpoint
	selectedActionsEndpoint
)

// observedSettings は、e で reco が観測する設定を返す。
// 宣言を受け付けない設定も更新操作の影響を受け得るため、照合する設定の一覧 (setting.Keys) とは別に持つ。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (e updateEndpoint) observedSettings() []setting.Key {
	switch e {
	case repositoryEndpoint:
		return []setting.Key{setting.DeleteBranchOnMerge, setting.AllowAutoMerge}
	case workflowPermissionsEndpoint:
		return []setting.Key{setting.DefaultWorkflowPermissions, setting.CanApprovePullRequestReviews}
	case actionsPermissionsEndpoint:
		return []setting.Key{setting.AllowedActions, setting.SHAPinningRequired}
	case selectedActionsEndpoint:
		return []setting.Key{setting.SelectedActions}
	}
	panic(fmt.Sprintf("apply: unknown update endpoint %d", e))
}

// updateRequestFunc は、target の設定を期待値 desired に更新する更新要求を一度送る。
type updateRequestFunc func(ctx context.Context, client *github.Client, target repository.Reference, desired setting.Value) (github.UpdateOutcome, error)

// updateOperation は、論理的な設定変更を実現する更新操作。
type updateOperation struct {
	endpoint updateEndpoint
	send     updateRequestFunc
}

// updateOperationOf は、key の論理的な設定変更を実現する更新操作を返す。
// 論理的な設定変更は宣言を受け付ける設定 (setting.Keys) にだけ生じ、どれも更新操作を持つため、
// それ以外の設定は不変条件の破れとして panic する。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
// 送る更新要求と比べる設定の範囲を別々の場所で足し忘れないよう、更新エンドポイントも同じ値で返す。
// 承認の前に引けるよう、client と期待値は引数で受ける関数値で返す。
func updateOperationOf(key setting.Key) updateOperation {
	switch key {
	case setting.DeleteBranchOnMerge:
		return updateOperation{repositoryEndpoint, boolUpdate((*github.Client).UpdateDeleteBranchOnMerge)}
	case setting.AllowAutoMerge:
		return updateOperation{repositoryEndpoint, boolUpdate((*github.Client).UpdateAllowAutoMerge)}
	case setting.AllowedActions:
		return updateOperation{actionsPermissionsEndpoint, func(ctx context.Context, client *github.Client, target repository.Reference, desired setting.Value) (github.UpdateOutcome, error) {
			return client.UpdateAllowedActions(ctx, target, desired.AllowedActionsPolicy())
		}}
	case setting.SelectedActions:
		// Actions 許可ポリシーが selected のまま3項目だけを変えるときも、ポリシーの更新要求は送らない。
		// selected を送り直すと、GitHub が github_owned_allowed を true に変えるため。
		return updateOperation{selectedActionsEndpoint, func(ctx context.Context, client *github.Client, target repository.Reference, desired setting.Value) (github.UpdateOutcome, error) {
			return client.UpdateSelectedActions(ctx, target, desired.SelectedActionsCriteria())
		}}
	case setting.DefaultWorkflowPermissions:
		return updateOperation{workflowPermissionsEndpoint, func(ctx context.Context, client *github.Client, target repository.Reference, desired setting.Value) (github.UpdateOutcome, error) {
			return client.UpdateDefaultWorkflowPermissions(ctx, target, desired.PermissionLevel())
		}}
	case setting.CanApprovePullRequestReviews:
		return updateOperation{workflowPermissionsEndpoint, boolUpdate((*github.Client).UpdateCanApprovePullRequestReviews)}
	case setting.SHAPinningRequired:
		// 宣言を受け付けない設定には論理的な設定変更が生じない
	}
	panic("apply: no update operation for " + string(key))
}

// boolUpdate は、真偽値を受ける更新メソッドを、期待値を受ける更新要求にする。
func boolUpdate(update func(*github.Client, context.Context, repository.Reference, bool) (github.UpdateOutcome, error)) updateRequestFunc {
	return func(ctx context.Context, client *github.Client, target repository.Reference, desired setting.Value) (github.UpdateOutcome, error) {
		return update(client, ctx, target, desired.Bool())
	}
}
