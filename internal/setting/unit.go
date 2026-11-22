package setting

import "fmt"

// Unit は管理の単位の識別。リポジトリ設定は Key だけで定まり、関連リソースに関わる管理の単位は、
// 形を表す Key と関連リソースの名前の組で定まる (アーキテクチャ設計の package の分け方)。
type Unit struct {
	Key Key
	// Environment は、Key が Environment か、Environment に属する Environment Variable の形のときだけ持つ。ほかの形ではゼロ値。
	Environment EnvironmentName
	// Variable は、Key が Repository Variable か Environment Variable の形のときだけ持つ。ほかの形ではゼロ値。
	Variable VariableName
}

// String は、CLI 仕様の表示の形で識別を返す。
// Key の形と名前の有無が食い違う識別は、どの管理の単位か決まらないため、不変条件の破れとして panic する。
func (u Unit) String() string {
	area := u.Key.Area()
	hasVariable := area == AreaRepositoryVariables || area == AreaEnvironmentVariables
	if (u.Environment != EnvironmentName{}) != u.Key.HasEnvironmentName() || (u.Variable != VariableName{}) != hasVariable {
		panic(fmt.Sprintf("setting: unit %q has a mismatched name (environment %q, variable %q)", u.Key, u.Environment, u.Variable))
	}
	switch u.Key {
	case EnvironmentResource:
		return "environments." + u.Environment.String()
	case EnvironmentDeploymentBranchPolicy:
		return "environments." + u.Environment.String() + ".deployment_branch_policy"
	case EnvironmentVariable:
		return "environments." + u.Environment.String() + ".variables." + u.Variable.String()
	case RepositoryVariable:
		return "actions.variables." + u.Variable.String()
	default:
		return string(u.Key)
	}
}
