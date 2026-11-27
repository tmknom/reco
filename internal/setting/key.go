// Package setting は、どの設定や設定の領域を指すかの識別と、設定の値を持つ。
package setting

import (
	"fmt"
	"strings"
)

// Key は設定の識別。リポジトリ設定では、値を表示にそのまま使い、管理の単位 (RepositorySettingUnits) では YAML 設定仕様のキーのパスと一致する。
// 関連リソースに関わる管理の単位では、Key は形だけを表し、名前との組 (Unit) で一つに定まる。
type Key string

const (
	DeleteBranchOnMerge Key = "general.delete_branch_on_merge"
	AllowAutoMerge      Key = "general.allow_auto_merge"
	AllowSquashMerge    Key = "general.allow_squash_merge"
	AllowMergeCommit    Key = "general.allow_merge_commit"
	AllowRebaseMerge    Key = "general.allow_rebase_merge"
	AllowUpdateBranch   Key = "general.allow_update_branch"
	// SquashMergeCommit は、squash merge のマージメッセージ。タイトルと本文の設定グループを一つの Key で表す。
	SquashMergeCommit Key = "general.squash_merge_commit"
	// MergeCommit は、merge commit のマージメッセージ。タイトルと本文の設定グループを一つの Key で表す。マージ方式の AllowMergeCommit とは別の設定である。
	MergeCommit                  Key = "general.merge_commit"
	ActionsEnabled               Key = "actions.permissions.enabled"
	AllowedActions               Key = "actions.permissions.allowed_actions"
	SelectedActions              Key = "actions.permissions.selected_actions"
	SHAPinningRequired           Key = "actions.permissions.sha_pinning_required"
	DefaultWorkflowPermissions   Key = "actions.workflow.default_workflow_permissions"
	CanApprovePullRequestReviews Key = "actions.workflow.can_approve_pull_request_reviews"
	ActionsAccessLevel           Key = "actions.access.access_level"
	// ForkPullRequestApproval は、Fork Pull Request ワークフローの承認ポリシー。
	ForkPullRequestApproval Key = "actions.fork_pull_requests.approval_policy"
	// PrivateForkPullRequestWorkflows は、非公開リポジトリの Fork Pull Request ワークフローの実行条件。4項目の設定グループを一つの Key で表す。
	PrivateForkPullRequestWorkflows Key = "actions.fork_pull_requests.private_repositories"
	// RetentionDays は、チェック、ワークフローの実行、コミットのステータス、Artifact、Log の保持日数。上限は設定ではなく、Key を持たない。
	RetentionDays Key = "actions.retention.days"
	// OIDCSubject は、OIDC トークンの subject クレームのカスタマイズ。3項目の設定グループを一つの Key で表す。
	OIDCSubject Key = "actions.oidc_subject"
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
	// EnvironmentRequiredReviewers は、Environment の中の担当者の集合の形。
	EnvironmentRequiredReviewers Key = "environments.required_reviewers"
	// EnvironmentPreventSelfReview は、Environment の中の自己承認の禁止の形。
	EnvironmentPreventSelfReview Key = "environments.prevent_self_review"
	// EnvironmentWaitTimer は、Environment の中のデプロイの待機時間の形。
	EnvironmentWaitTimer Key = "environments.wait_timer"
	// EnvironmentVariable は、Environment に属する Environment Variable そのもの (期待値は存在し、値を伴うこと) の形。
	EnvironmentVariable Key = "environments.variables"
	// RulesetResource は、Ruleset そのもの (期待値は存在し、値を伴うこと) の形。
	RulesetResource Key = "rulesets"
)

// RepositorySettingUnits は、リポジトリ設定の管理の単位 (reco が期待値の宣言を受け付ける設定) を YAML 設定仕様の章と表の並びで返す。
// 設定グループは一つの Key で表す。関連リソースに関わる管理の単位は、宣言の名前ごとに決まるため含めない。
// 呼び出し側が要素を書き換えても共有されないよう、呼び出すたびに新しいスライスを返す。
func RepositorySettingUnits() []Key {
	return []Key{DeleteBranchOnMerge, AllowAutoMerge, AllowSquashMerge, AllowMergeCommit, AllowRebaseMerge, AllowUpdateBranch, SquashMergeCommit, MergeCommit, ActionsEnabled, AllowedActions, SelectedActions, SHAPinningRequired, DefaultWorkflowPermissions, CanApprovePullRequestReviews, ActionsAccessLevel, ForkPullRequestApproval, PrivateForkPullRequestWorkflows, RetentionDays, OIDCSubject, DependabotAlerts, DependabotSecurityUpdates, SecretScanningEnabled, RepositoryPushProtection, PrivateVulnerabilityReportingEnabled}
}

// MergeMethods は、GitHub がすべてを無効にすることを受け付けないマージ方式の管理の単位を、YAML 設定仕様の表の並びで返す。
// 呼び出し側が要素を書き換えても共有されないよう、呼び出すたびに新しいスライスを返す。
func MergeMethods() []Key {
	return []Key{AllowSquashMerge, AllowMergeCommit, AllowRebaseMerge}
}

// CorrespondingMergeMethod は、k がマージメッセージのとき、対応するマージ方式と真を返す。
// GitHub は、対応するマージ方式が無効のあいだのマージメッセージの更新を拒否する (General の設計)。
func (k Key) CorrespondingMergeMethod() (Key, bool) {
	switch k {
	case SquashMergeCommit:
		return AllowSquashMerge, true
	case MergeCommit:
		return AllowMergeCommit, true
	default:
		return "", false
	}
}

// DeclaredKeyName は、k の管理の単位を宣言する、設定ファイルの最も内側のキーを返す。
// Key の値は YAML 設定仕様のキーのパスを表すため、パスの最後の要素がそのキーになる。関連リソースの形では、その種類を並べるキー (rulesets など) を返す。
func (k Key) DeclaredKeyName() string {
	path := string(k)
	return path[strings.LastIndex(path, ".")+1:]
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
	case DeleteBranchOnMerge, AllowAutoMerge, AllowSquashMerge, AllowMergeCommit, AllowRebaseMerge, AllowUpdateBranch, SquashMergeCommit, MergeCommit:
		return AreaGeneral
	case ActionsEnabled, AllowedActions, SelectedActions, SHAPinningRequired:
		return AreaActionsPermissions
	case DefaultWorkflowPermissions, CanApprovePullRequestReviews:
		return AreaWorkflowPermissions
	case ActionsAccessLevel:
		return AreaActionsAccess
	case ForkPullRequestApproval:
		return AreaForkPullRequestApproval
	case PrivateForkPullRequestWorkflows:
		return AreaPrivateForkPullRequestWorkflows
	case RetentionDays:
		return AreaRetention
	case OIDCSubject:
		return AreaOIDCSubject
	case RepositoryVariable:
		return AreaRepositoryVariables
	case EnvironmentResource, EnvironmentDeploymentBranchPolicy, EnvironmentRequiredReviewers, EnvironmentPreventSelfReview, EnvironmentWaitTimer:
		return AreaEnvironments
	case EnvironmentVariable:
		return AreaEnvironmentVariables
	case DependabotAlerts, DependabotSecurityUpdates:
		return AreaDependabot
	case SecretScanningEnabled, RepositoryPushProtection:
		return AreaSecretScanning
	case PrivateVulnerabilityReportingEnabled:
		return AreaPrivateVulnerabilityReporting
	case RulesetResource:
		return AreaRulesets
	}
	panic(fmt.Sprintf("unknown setting %q", k))
}
