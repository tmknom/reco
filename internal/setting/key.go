// Package setting は、どの設定や設定の領域を指すかの識別と、設定の値を持つ。
package setting

import "fmt"

// Key は設定の識別。リポジトリ設定では、値を表示にそのまま使い、管理の単位 (ManagementUnits) では YAML 設定仕様のキーのパスと一致する。
// Environment に関わる管理の単位では、Key は形だけを表し、名前との組 (Unit) で一つに定まる。
type Key string

const (
	DeleteBranchOnMerge          Key = "general.delete_branch_on_merge"
	AllowAutoMerge               Key = "general.allow_auto_merge"
	ActionsEnabled               Key = "actions.permissions.enabled"
	AllowedActions               Key = "actions.permissions.allowed_actions"
	SelectedActions              Key = "actions.permissions.selected_actions"
	SHAPinningRequired           Key = "actions.permissions.sha_pinning_required"
	DefaultWorkflowPermissions   Key = "actions.workflow.default_workflow_permissions"
	CanApprovePullRequestReviews Key = "actions.workflow.can_approve_pull_request_reviews"
	ActionsAccessLevel           Key = "actions.access.access_level"
	DependabotAlerts             Key = "security.dependabot_alerts"
	DependabotSecurityUpdates    Key = "security.dependabot_security_updates"
	// EnvironmentResource は、Environment そのもの (期待値は存在すること) の形。
	EnvironmentResource Key = "environments"
	// EnvironmentDeploymentBranchPolicy は、Environment の中のデプロイ元制限の形。
	EnvironmentDeploymentBranchPolicy Key = "environments.deployment_branch_policy"
)

// ManagementUnits は、リポジトリ設定の管理の単位 (reco が期待値の宣言を受け付ける設定) を YAML 設定仕様の章と表の並びで返す。
// 設定グループは一つの Key で表す。Environment に関わる管理の単位は、宣言の名前ごとに決まるため含めない。
// 呼び出し側が要素を書き換えても共有されないよう、呼び出すたびに新しいスライスを返す。
func ManagementUnits() []Key {
	return []Key{DeleteBranchOnMerge, AllowAutoMerge, ActionsEnabled, AllowedActions, SelectedActions, SHAPinningRequired, DefaultWorkflowPermissions, CanApprovePullRequestReviews, ActionsAccessLevel, DependabotAlerts, DependabotSecurityUpdates}
}

// Area は、k が属する設定の領域を返す。未知の設定は不変条件の破れとして panic する。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (k Key) Area() Area {
	switch k {
	case DeleteBranchOnMerge, AllowAutoMerge:
		return AreaGeneral
	case ActionsEnabled, AllowedActions, SelectedActions, SHAPinningRequired:
		return AreaActionsPermissions
	case DefaultWorkflowPermissions, CanApprovePullRequestReviews:
		return AreaWorkflowPermissions
	case ActionsAccessLevel:
		return AreaActionsAccess
	case EnvironmentResource, EnvironmentDeploymentBranchPolicy:
		return AreaEnvironments
	case DependabotAlerts, DependabotSecurityUpdates:
		return AreaDependabot
	}
	panic(fmt.Sprintf("unknown setting %q", k))
}
