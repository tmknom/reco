package setting

// AllowedActionsPolicy は、Actions 許可ポリシーが取り得る値。値は GitHub REST API と YAML 設定仕様の文字列で、表示にもそのまま使う。
type AllowedActionsPolicy string

const (
	AllowedActionsAll       AllowedActionsPolicy = "all"
	AllowedActionsLocalOnly AllowedActionsPolicy = "local_only"
	AllowedActionsSelected  AllowedActionsPolicy = "selected"
)

// ParseAllowedActionsPolicy は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseAllowedActionsPolicy(s string) (AllowedActionsPolicy, bool) {
	switch policy := AllowedActionsPolicy(s); policy {
	case AllowedActionsAll, AllowedActionsLocalOnly, AllowedActionsSelected:
		return policy, true
	default:
		return "", false
	}
}
