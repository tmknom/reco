package setting

// AccessLevel は、リポジトリ外のワークフローからのアクセスの範囲が取り得る値。値は GitHub REST API の文字列で、表示にもそのまま使う。
// GitHub が返す値をすべて持つ。
type AccessLevel string

const (
	AccessNone         AccessLevel = "none"
	AccessUser         AccessLevel = "user"
	AccessOrganization AccessLevel = "organization"
)

// ParseAccessLevel は s が定義された値に完全一致するときだけ、その値と真を返す。
func ParseAccessLevel(s string) (AccessLevel, bool) {
	switch level := AccessLevel(s); level {
	case AccessNone, AccessUser, AccessOrganization:
		return level, true
	default:
		return "", false
	}
}
