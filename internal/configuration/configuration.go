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
	Reference    repository.Reference
	DesiredState DesiredState
}

// DesiredState は、一つのリポジトリについて管理設定に求める状態。
// YAML の入れ子 (actions.workflow) は写さず、設定の領域ごとに持つ。
type DesiredState struct {
	General                         General
	ActionsPermissions              ActionsPermissions
	WorkflowPermissions             WorkflowPermissions
	ActionsAccess                   ActionsAccess
	ForkPullRequestApproval         ForkPullRequestApproval
	PrivateForkPullRequestWorkflows PrivateForkPullRequestWorkflows
	Retention                       Retention
	OIDCSubject                     OIDCSubject
	// RepositoryVariables は宣言の並びのまま持ち、actions.variables を省略したリポジトリでは nil。
	RepositoryVariables []VariableDeclaration
	// Environments は宣言の並びのまま持ち、environments を省略したリポジトリでは nil。
	Environments []EnvironmentDeclaration
	// Rulesets は宣言の並びのまま持ち、rulesets を省略したリポジトリでは nil。
	Rulesets                      []RulesetDeclaration
	Dependabot                    Dependabot
	SecretScanning                SecretScanning
	PrivateVulnerabilityReporting PrivateVulnerabilityReporting
}

// VariableDeclaration は、一つの変数の宣言。
type VariableDeclaration struct {
	Name setting.VariableName
	// Desired は、存在し値を伴うことの期待値 (setting.VariableValue) か、存在しないことの期待値 (setting.Absent)。
	// setting.Absent を入れてよいのは state: absent を宣言したときだけで、value の省略や null、空文字列では入れない。
	Desired setting.Value
}

// EnvironmentDeclaration は、一つの Environment の宣言。その名前の Environment が存在することの期待値を兼ねる。
type EnvironmentDeclaration struct {
	Name setting.EnvironmentName
	// DeploymentBranchPolicy は nil なら期待値の省略で、デプロイ元制限は管理範囲に含まれない。宣言したときは、方式が selected のときだけ、宣言の順の Patterns を持つ。
	DeploymentBranchPolicy *setting.DeploymentBranchPolicy
	// RequiredReviewers は nil なら期待値の省略で、担当者の集合は管理範囲に含まれない。required_reviewers: [] は空の集合を指す非 nil。
	RequiredReviewers *setting.ReviewerSet
	// PreventSelfReview は nil なら期待値の省略で、自己承認の禁止は管理範囲に含まれない。
	PreventSelfReview *bool
	// WaitTimer は nil なら期待値の省略で、デプロイの待機時間 (分) は管理範囲に含まれない。0 は待機時間がないことの宣言を指す非 nil。
	WaitTimer *int
	// Variables は宣言の並びのまま持ち、variables を省略した Environment では nil。
	Variables []VariableDeclaration
}

// RulesetDeclaration は、一つの Ruleset の宣言。その名前の Ruleset が存在し、値が宣言と同じであること、または存在しないことの期待値を兼ねる。
type RulesetDeclaration struct {
	Name setting.RulesetName
	// Desired は、存在し値を伴うことの期待値 (setting.RulesetValue) か、存在しないことの期待値 (setting.Absent())。
	Desired setting.Value
}

// General はリポジトリ一般設定の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type General struct {
	DeleteBranchOnMerge *bool
	AllowAutoMerge      *bool
	AllowSquashMerge    *bool
	AllowMergeCommit    *bool
	AllowRebaseMerge    *bool
	AllowUpdateBranch   *bool
	// SquashMergeCommit と MergeCommit は設定グループの期待値で、2項目を一体として持つ。
	SquashMergeCommit        *setting.SquashMergeCommitDefault
	MergeCommit              *setting.MergeCommitDefault
	HasIssues                *bool
	HasProjects              *bool
	HasWiki                  *bool
	WebCommitSignoffRequired *bool
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

// ForkPullRequestApproval は、Fork Pull Request ワークフローの承認の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type ForkPullRequestApproval struct {
	ApprovalPolicy *setting.ForkPullRequestApprovalPolicy
}

// PrivateForkPullRequestWorkflows は、非公開リポジトリの Fork Pull Request ワークフローの期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type PrivateForkPullRequestWorkflows struct {
	// Conditions は設定グループの期待値で、4項目を一体として持つ。
	Conditions *setting.PrivateForkPullRequestWorkflowConditions
}

// Retention は、保持期間の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type Retention struct {
	Days *int
}

// OIDCSubject は、OIDC トークンの subject クレームのカスタマイズの期待値。
type OIDCSubject struct {
	// Customization は設定グループの期待値で、3項目を一体として持つ。nil は期待値の省略で、その設定は管理範囲に含まれない。
	Customization *setting.OIDCSubjectCustomization
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

// ManagedSetting は、管理設定 (期待値を明示した管理の単位) と、その期待値。
type ManagedSetting struct {
	Unit    setting.Unit
	Desired setting.Value
}

// ManagedSettings は、管理範囲の管理設定を期待値とともに返す。何も宣言していなければ nil を返す。
// 並びは、リポジトリ設定 (setting.RepositorySettingUnits の並び)、Repository Variable (宣言の並び)、Environment ごと (宣言の並び) に存在、デプロイ元制限、担当者の集合、変数 (宣言の並び)、Ruleset (宣言の並び) である。
// Environment の宣言は存在することの期待値を兼ねるため、名前だけの宣言も存在の管理設定になる。
// Ruleset の宣言は、存在し値が宣言と同じことの期待値を一つの管理設定にする。
// 管理範囲を読む処理ごとに宣言を走査すると管理設定が食い違うため、ここでだけ列挙する。
func (d DesiredState) ManagedSettings() []ManagedSetting {
	var managed []ManagedSetting
	for _, key := range setting.RepositorySettingUnits() {
		if desired, ok := d.value(key); ok {
			managed = append(managed, ManagedSetting{Unit: setting.Unit{Key: key}, Desired: desired})
		}
	}
	for _, v := range d.RepositoryVariables {
		managed = append(managed, ManagedSetting{Unit: setting.Unit{Key: setting.RepositoryVariable, Variable: v.Name}, Desired: v.Desired})
	}
	for _, e := range d.Environments {
		managed = append(managed, ManagedSetting{Unit: setting.Unit{Key: setting.EnvironmentResource, Environment: e.Name}, Desired: setting.Present()})
		if e.DeploymentBranchPolicy != nil {
			unit := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: e.Name}
			managed = append(managed, ManagedSetting{Unit: unit, Desired: setting.DeploymentBranchPolicyValue(*e.DeploymentBranchPolicy)})
		}
		if e.RequiredReviewers != nil {
			unit := setting.Unit{Key: setting.EnvironmentRequiredReviewers, Environment: e.Name}
			managed = append(managed, ManagedSetting{Unit: unit, Desired: setting.ReviewerSetValue(*e.RequiredReviewers)})
		}
		if e.PreventSelfReview != nil {
			unit := setting.Unit{Key: setting.EnvironmentPreventSelfReview, Environment: e.Name}
			managed = append(managed, ManagedSetting{Unit: unit, Desired: setting.BoolValue(*e.PreventSelfReview)})
		}
		if e.WaitTimer != nil {
			unit := setting.Unit{Key: setting.EnvironmentWaitTimer, Environment: e.Name}
			managed = append(managed, ManagedSetting{Unit: unit, Desired: setting.WaitTimerValue(*e.WaitTimer)})
		}
		for _, v := range e.Variables {
			managed = append(managed, ManagedSetting{Unit: setting.Unit{Key: setting.EnvironmentVariable, Environment: e.Name, Variable: v.Name}, Desired: v.Desired})
		}
	}
	for _, r := range d.Rulesets {
		managed = append(managed, ManagedSetting{Unit: setting.Unit{Key: setting.RulesetResource, Ruleset: r.Name}, Desired: r.Desired})
	}
	return managed
}

// value は、key の設定の期待値を返す。期待値が省略されていれば ok は偽になる。
// 未知の設定は不変条件の破れとして panic する。
// 設定を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (d DesiredState) value(key setting.Key) (value setting.Value, ok bool) {
	switch key {
	case setting.DeleteBranchOnMerge:
		return optionalBoolValue(d.General.DeleteBranchOnMerge)
	case setting.AllowAutoMerge:
		return optionalBoolValue(d.General.AllowAutoMerge)
	case setting.AllowSquashMerge:
		return optionalBoolValue(d.General.AllowSquashMerge)
	case setting.AllowMergeCommit:
		return optionalBoolValue(d.General.AllowMergeCommit)
	case setting.AllowRebaseMerge:
		return optionalBoolValue(d.General.AllowRebaseMerge)
	case setting.AllowUpdateBranch:
		return optionalBoolValue(d.General.AllowUpdateBranch)
	case setting.SquashMergeCommit:
		if v := d.General.SquashMergeCommit; v != nil {
			return setting.SquashMergeCommitDefaultValue(*v), true
		}
		return setting.Value{}, false
	case setting.MergeCommit:
		if v := d.General.MergeCommit; v != nil {
			return setting.MergeCommitDefaultValue(*v), true
		}
		return setting.Value{}, false
	case setting.HasIssues:
		return optionalBoolValue(d.General.HasIssues)
	case setting.HasProjects:
		return optionalBoolValue(d.General.HasProjects)
	case setting.HasWiki:
		return optionalBoolValue(d.General.HasWiki)
	case setting.WebCommitSignoffRequired:
		return optionalBoolValue(d.General.WebCommitSignoffRequired)
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
	case setting.ForkPullRequestApproval:
		if policy := d.ForkPullRequestApproval.ApprovalPolicy; policy != nil {
			return setting.ForkPullRequestApprovalPolicyValue(*policy), true
		}
		return setting.Value{}, false
	case setting.PrivateForkPullRequestWorkflows:
		if conditions := d.PrivateForkPullRequestWorkflows.Conditions; conditions != nil {
			return setting.PrivateForkPullRequestWorkflowConditionsValue(*conditions), true
		}
		return setting.Value{}, false
	case setting.RetentionDays:
		if days := d.Retention.Days; days != nil {
			return setting.RetentionDaysValue(*days), true
		}
		return setting.Value{}, false
	case setting.OIDCSubject:
		if customization := d.OIDCSubject.Customization; customization != nil {
			return setting.OIDCSubjectCustomizationValue(*customization), true
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
	case setting.RepositoryVariable, setting.EnvironmentResource, setting.EnvironmentDeploymentBranchPolicy, setting.EnvironmentRequiredReviewers, setting.EnvironmentPreventSelfReview, setting.EnvironmentWaitTimer, setting.EnvironmentVariable, setting.RulesetResource:
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
