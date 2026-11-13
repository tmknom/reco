// Package setting は、どの設定を指すかの識別を持つ。
package setting

// Key は設定の識別。値は YAML 設定仕様のキーのパスで、表示にもそのまま使う。
type Key string

const (
	DeleteBranchOnMerge Key = "general.delete_branch_on_merge"
	AllowAutoMerge      Key = "general.allow_auto_merge"
)

// GeneralKeys は、リポジトリ一般設定の設定を YAML 設定仕様の表の並びで返す。
// 呼び出し側が要素を書き換えても共有されないよう、呼び出すたびに新しいスライスを返す。
func GeneralKeys() []Key {
	return []Key{DeleteBranchOnMerge, AllowAutoMerge}
}
