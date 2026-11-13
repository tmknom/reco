package setting

// PermissionLevel は、ワークフローの GITHUB_TOKEN の既定権限が取り得る値。値は GitHub REST API と YAML 設定仕様の文字列で、表示にもそのまま使う。
type PermissionLevel string

const (
	PermissionRead  PermissionLevel = "read"
	PermissionWrite PermissionLevel = "write"
)

// ParsePermissionLevel は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParsePermissionLevel(s string) (PermissionLevel, bool) {
	switch level := PermissionLevel(s); level {
	case PermissionRead, PermissionWrite:
		return level, true
	default:
		return "", false
	}
}
