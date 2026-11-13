// Package setting は、どの設定を指すかの識別を持つ。
package setting

// Key は設定の識別。値は YAML 設定仕様のキーのパスで、表示にもそのまま使う。
// 宣言を受け付けない未管理設定も、同じ形で指す。
type Key string

const (
	DeleteBranchOnMerge Key = "general.delete_branch_on_merge"
	AllowAutoMerge      Key = "general.allow_auto_merge"
)
