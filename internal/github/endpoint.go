package github

import (
	"fmt"

	"github.com/tmknom/reco/internal/setting"
)

// UpdateEndpoint は、GitHub が更新要求を受け付ける単位である更新エンドポイント。
type UpdateEndpoint int

// ゼロ値を定義済みの更新エンドポイントと取り違えないよう 1 から始める。
const (
	RepositoryEndpoint UpdateEndpoint = iota + 1
	WorkflowPermissionsEndpoint
	ActionsPermissionsEndpoint
	SelectedActionsEndpoint
	ActionsAccessEndpoint
	DependabotAlertsEndpoint
	DependabotSecurityUpdatesEndpoint
)

// ObservedSettings は、e で reco が観測する設定を返す。
// 更新要求を送らない設定も更新操作の影響を受け得るため、更新要求を持つ設定とは別に持つ。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (e UpdateEndpoint) ObservedSettings() []setting.Key {
	switch e {
	case RepositoryEndpoint:
		return []setting.Key{setting.DeleteBranchOnMerge, setting.AllowAutoMerge}
	case WorkflowPermissionsEndpoint:
		return []setting.Key{setting.DefaultWorkflowPermissions, setting.CanApprovePullRequestReviews}
	case ActionsPermissionsEndpoint:
		return []setting.Key{setting.ActionsEnabled, setting.AllowedActions, setting.SHAPinningRequired}
	case SelectedActionsEndpoint:
		return []setting.Key{setting.SelectedActions}
	case ActionsAccessEndpoint:
		return []setting.Key{setting.ActionsAccessLevel}
	case DependabotAlertsEndpoint:
		return []setting.Key{setting.DependabotAlerts}
	case DependabotSecurityUpdatesEndpoint:
		return []setting.Key{setting.DependabotSecurityUpdates}
	}
	panic(fmt.Sprintf("github: unknown update endpoint %d", e))
}

// UpdateEndpointOf は、key の設定を変える更新要求を受け付ける更新エンドポイントを返す。
// 未定義の設定は、不変条件の破れとして panic する。
func UpdateEndpointOf(key setting.Key) UpdateEndpoint {
	return updateRequestOf(key).endpoint
}
