// Package configuration は、YAML 設定ファイルを解析・検証し、設定宣言に変換する。
//
// YAML の構文要素 (タグ、アンカー) は受理条件の判定に使うため、構造体へ直接デコードせず
// yaml.Node の木と、各ノードの位置にあるソースの文字を走査して検証する。公開する型は YAML を知らない。
package configuration

import (
	"fmt"

	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// Declaration は設定宣言。
type Declaration struct {
	Repositories []RepositoryDeclaration
}

// RepositoryDeclaration はリポジトリごとの宣言。
type RepositoryDeclaration struct {
	Reference repository.Reference
	Desired   DesiredState
}

// DesiredState は、一つのリポジトリについて管理設定に求める状態。
// YAML の入れ子 (actions.workflow) は写さず、設定の領域ごとに持つ。
type DesiredState struct {
	General             General
	ActionsPermissions  ActionsPermissions
	WorkflowPermissions WorkflowPermissions
	ActionsAccess       ActionsAccess
	// RepositoryVariables は宣言の並びのまま持ち、actions.variables を省略したリポジトリでは nil。
	RepositoryVariables []VariableDeclaration
	// Environments は宣言の並びのまま持ち、environments を省略したリポジトリでは nil。
	Environments                  []EnvironmentDeclaration
	Dependabot                    Dependabot
	SecretScanning                SecretScanning
	PrivateVulnerabilityReporting PrivateVulnerabilityReporting
}

// VariableDeclaration は、一つの変数の宣言。
type VariableDeclaration struct {
	Name setting.VariableName
	// Desired は、存在し値を伴うことの期待値 (setting.VariableValue)。
	Desired setting.Value
}

// EnvironmentDeclaration は、一つの Environment の宣言。その名前の Environment が存在することの期待値を兼ねる。
type EnvironmentDeclaration struct {
	Name setting.EnvironmentName
	// DeploymentBranchPolicy は nil なら期待値の省略で、デプロイ元制限は管理範囲に含まれない。宣言したときは、方式が selected のときだけ、宣言の順の Patterns を持つ。
	DeploymentBranchPolicy *setting.DeploymentBranchPolicy
	// Variables は宣言の並びのまま持ち、variables を省略した Environment では nil。
	Variables []VariableDeclaration
}

// General はリポジトリ一般設定の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type General struct {
	DeleteBranchOnMerge *bool
	AllowAutoMerge      *bool
}

// ActionsPermissions は Actions の実行許可の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type ActionsPermissions struct {
	// Enabled は、AllowedActions を宣言したときは nil。有効であることの期待値は AllowedActions が担う (YAML 設定仕様 4章)。
	Enabled        *bool
	AllowedActions *setting.AllowedActionsPolicy
	// SelectedActions は設定グループの期待値で、3項目を一体として持つ。
	SelectedActions    *setting.SelectedActionsCriteria
	SHAPinningRequired *bool
}

// WorkflowPermissions はワークフロー権限の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type WorkflowPermissions struct {
	DefaultWorkflowPermissions   *setting.PermissionLevel
	CanApprovePullRequestReviews *bool
}

// ActionsAccess は、リポジトリ外のワークフローからのアクセスの期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type ActionsAccess struct {
	// AccessLevel は none か user。organization は宣言で受け付けない (YAML 設定仕様の Actions access)。
	AccessLevel *setting.AccessLevel
}

// Dependabot は、Dependabot の領域の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
// 2つの期待値は互いの状態を含まない (YAML 設定仕様の Security)。
type Dependabot struct {
	Alerts          *bool
	SecurityUpdates *bool
}

// SecretScanning は、Secret scanning の領域の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
// 2つの期待値は互いの状態を含まない (YAML 設定仕様の Security)。
type SecretScanning struct {
	Enabled        *bool
	PushProtection *bool
}

// PrivateVulnerabilityReporting は、Private vulnerability reporting の領域の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type PrivateVulnerabilityReporting struct {
	Enabled *bool
}

// Value は、key の設定の期待値を返す。期待値が省略されていれば ok は偽になる。
// 未知の設定は不変条件の破れとして panic する。
// 設定を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (d DesiredState) Value(key setting.Key) (value setting.Value, ok bool) {
	switch key {
	case setting.DeleteBranchOnMerge:
		return optionalBoolValue(d.General.DeleteBranchOnMerge)
	case setting.AllowAutoMerge:
		return optionalBoolValue(d.General.AllowAutoMerge)
	case setting.ActionsEnabled:
		return optionalBoolValue(d.ActionsPermissions.Enabled)
	case setting.AllowedActions:
		if policy := d.ActionsPermissions.AllowedActions; policy != nil {
			return setting.AllowedActionsPolicyValue(*policy), true
		}
		return setting.Value{}, false
	case setting.SelectedActions:
		if criteria := d.ActionsPermissions.SelectedActions; criteria != nil {
			return setting.SelectedActionsCriteriaValue(*criteria), true
		}
		return setting.Value{}, false
	case setting.DefaultWorkflowPermissions:
		if level := d.WorkflowPermissions.DefaultWorkflowPermissions; level != nil {
			return setting.PermissionLevelValue(*level), true
		}
		return setting.Value{}, false
	case setting.CanApprovePullRequestReviews:
		return optionalBoolValue(d.WorkflowPermissions.CanApprovePullRequestReviews)
	case setting.SHAPinningRequired:
		return optionalBoolValue(d.ActionsPermissions.SHAPinningRequired)
	case setting.ActionsAccessLevel:
		if level := d.ActionsAccess.AccessLevel; level != nil {
			return setting.AccessLevelValue(*level), true
		}
		return setting.Value{}, false
	case setting.DependabotAlerts:
		return optionalBoolValue(d.Dependabot.Alerts)
	case setting.DependabotSecurityUpdates:
		if enabled := d.Dependabot.SecurityUpdates; enabled != nil {
			return setting.DependabotSecurityUpdatesValue(setting.DependabotSecurityUpdatesStateOf(*enabled)), true
		}
		return setting.Value{}, false
	case setting.SecretScanningEnabled:
		return optionalBoolValue(d.SecretScanning.Enabled)
	case setting.RepositoryPushProtection:
		return optionalBoolValue(d.SecretScanning.PushProtection)
	case setting.PrivateVulnerabilityReportingEnabled:
		return optionalBoolValue(d.PrivateVulnerabilityReporting.Enabled)
	case setting.RepositoryVariable, setting.EnvironmentResource, setting.EnvironmentDeploymentBranchPolicy, setting.EnvironmentVariable:
		// 関連リソースに関わる管理の単位は名前との組で識別するため、この関数では引けない。
	}
	panic(fmt.Sprintf("unknown setting %q", key))
}

func optionalBoolValue(b *bool) (setting.Value, bool) {
	if b == nil {
		return setting.Value{}, false
	}
	return setting.BoolValue(*b), true
}
