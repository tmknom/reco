// Package setting は、どの設定や設定の領域を指すかの識別と、設定の値を持つ。
package setting

import "fmt"

// Key は設定の識別。値は YAML 設定仕様のキーのパスで、表示にもそのまま使う。
type Key string

const (
	DeleteBranchOnMerge          Key = "general.delete_branch_on_merge"
	AllowAutoMerge               Key = "general.allow_auto_merge"
	DefaultWorkflowPermissions   Key = "actions.workflow.default_workflow_permissions"
	CanApprovePullRequestReviews Key = "actions.workflow.can_approve_pull_request_reviews"
)

// Keys は、reco が宣言を受け付ける設定を YAML 設定仕様の章と表の並びで返す。
// 呼び出し側が要素を書き換えても共有されないよう、呼び出すたびに新しいスライスを返す。
func Keys() []Key {
	return []Key{DeleteBranchOnMerge, AllowAutoMerge, DefaultWorkflowPermissions, CanApprovePullRequestReviews}
}

// KeysIn は、Keys のうち area に属する設定を Keys の並びで返す。
func KeysIn(area Area) []Key {
	var keys []Key
	for _, key := range Keys() {
		if key.Area() == area {
			keys = append(keys, key)
		}
	}
	return keys
}

// Area は、k が属する設定の領域を返す。未知の設定は不変条件の破れとして panic する。
// 定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func (k Key) Area() Area {
	switch k {
	case DeleteBranchOnMerge, AllowAutoMerge:
		return AreaGeneral
	case DefaultWorkflowPermissions, CanApprovePullRequestReviews:
		return AreaWorkflowPermissions
	}
	panic(fmt.Sprintf("unknown setting %q", k))
}
