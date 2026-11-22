// Package setting は、どの設定や設定の領域を指すかの識別と、設定の値を持つ。
package setting

import "fmt"

// Key は設定の識別。リポジトリ設定では、値を表示にそのまま使い、管理の単位 (RepositorySettingUnits) では YAML 設定仕様のキーのパスと一致する。
// 関連リソースに関わる管理の単位では、Key は形だけを表し、名前との組 (Unit) で一つに定まる。
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
	// RepositoryVariable は、Repository Variable そのもの (期待値は存在し、値を伴うこと) の形。
	RepositoryVariable        Key = "actions.variables"
	DependabotAlerts          Key = "security.dependabot_alerts"
	DependabotSecurityUpdates Key = "security.dependabot_security_updates"
	SecretScanningEnabled     Key = "security.secret_scanning"
	// RepositoryPushProtection は、リポジトリに設定する push protection で、利用者が自分のアカウントに設定する push protection ではない。
	RepositoryPushProtection             Key = "security.secret_scanning_push_protection"
	PrivateVulnerabilityReportingEnabled Key = "security.private_vulnerability_reporting"
	// EnvironmentResource は、Environment そのもの (期待値は存在すること) の形。
	EnvironmentResource Key = "environments"
	// EnvironmentDeploymentBranchPolicy は、Environment の中のデプロイ元制限の形。
	EnvironmentDeploymentBranchPolicy Key = "environments.deployment_branch_policy"
	// EnvironmentVariable は、Environment に属する Environment Variable そのもの (期待値は存在し、値を伴うこと) の形。
	EnvironmentVariable Key = "environments.variables"
)

// RepositorySettingUnits は、リポジトリ設定の管理の単位 (reco が期待値の宣言を受け付ける設定) を YAML 設定仕様の章と表の並びで返す。
// 設定グループは一つの Key で表す。関連リソースに関わる管理の単位は、宣言の名前ごとに決まるため含めない。
// 呼び出し側が要素を書き換えても共有されないよう、呼び出すたびに新しいスライスを返す。
func RepositorySettingUnits() []Key {
	return []Key{DeleteBranchOnMerge, AllowAutoMerge, ActionsEnabled, AllowedActions, SelectedActions, SHAPinningRequired, DefaultWorkflowPermissions, CanApprovePullRequestReviews, ActionsAccessLevel, DependabotAlerts, DependabotSecurityUpdates, SecretScanningEnabled, RepositoryPushProtection, PrivateVulnerabilityReportingEnabled}
}

// HasEnvironmentName は、k の形の管理の単位が Environment の名前を含むか (Environment そのもの、その中の設定、それに属する関連リソースか) を返す。
func (k Key) HasEnvironmentName() bool {
	area := k.Area()
	return area == AreaEnvironments || area == AreaEnvironmentVariables
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
	case RepositoryVariable:
		return AreaRepositoryVariables
	case EnvironmentResource, EnvironmentDeploymentBranchPolicy:
		return AreaEnvironments
	case EnvironmentVariable:
		return AreaEnvironmentVariables
	case DependabotAlerts, DependabotSecurityUpdates:
		return AreaDependabot
	case SecretScanningEnabled, RepositoryPushProtection:
		return AreaSecretScanning
	case PrivateVulnerabilityReportingEnabled:
		return AreaPrivateVulnerabilityReporting
	}
	panic(fmt.Sprintf("unknown setting %q", k))
}
