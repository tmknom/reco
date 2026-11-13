// Package configuration は、YAML 設定ファイルを解析・検証し、設定宣言に変換する。
//
// YAML の構文要素 (タグ、アンカー) は受理条件の判定に使うため、構造体へ直接デコードせず
// yaml.Node の木と、各ノードの位置にあるソースの文字を走査して検証する。公開する型は YAML を知らない。
package configuration

import (
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
	WorkflowPermissions WorkflowPermissions
}

// General はリポジトリ一般設定の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type General struct {
	DeleteBranchOnMerge *bool
	AllowAutoMerge      *bool
}

// WorkflowPermissions はワークフロー権限の期待値。nil は期待値の省略で、その設定は管理範囲に含まれない。
type WorkflowPermissions struct {
	DefaultWorkflowPermissions   *setting.PermissionLevel
	CanApprovePullRequestReviews *bool
}
