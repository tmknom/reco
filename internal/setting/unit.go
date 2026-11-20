package setting

import "fmt"

// Unit は管理の単位の識別。リポジトリ設定は Key だけで定まり、Environment に関わる管理の単位は、
// 形を表す Key と Environment の名前の組で定まる (アーキテクチャ設計の package の分け方)。
type Unit struct {
	Key Key
	// Environment は、Key が Environment の形のときだけ持つ。リポジトリ設定ではゼロ値。
	Environment EnvironmentName
}

// String は、CLI 仕様の表示の形で識別を返す。
// Key の形と名前の有無が食い違う識別は、どの管理の単位か決まらないため、不変条件の破れとして panic する。
func (u Unit) String() string {
	named := u.Environment != EnvironmentName{}
	if named != (u.Key.Area() == AreaEnvironments) {
		panic(fmt.Sprintf("setting: unit %q has a mismatched environment name %q", u.Key, u.Environment))
	}
	switch u.Key {
	case EnvironmentResource:
		return "environments." + u.Environment.String()
	case EnvironmentDeploymentBranchPolicy:
		return "environments." + u.Environment.String() + ".deployment_branch_policy"
	default:
		return string(u.Key)
	}
}
