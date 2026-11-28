package setting

import "fmt"

// Unit は管理の単位の識別。リポジトリ設定は Key だけで定まり、関連リソースに関わる管理の単位は、
// 形を表す Key と関連リソースの名前の組で定まる (アーキテクチャ設計の package の分け方)。
// Environment か Ruleset の名前を持つ単位どうしでは、== は綴りの違う同じ関連リソースを別の単位とするため、同じ管理の単位かは Same で判定する。
type Unit struct {
	Key Key
	// Environment は、Key が Environment か、Environment に属する Environment Variable の形のときだけ持つ。ほかの形ではゼロ値。
	Environment EnvironmentName
	// Variable は、Key が Repository Variable か Environment Variable の形のときだけ持つ。ほかの形ではゼロ値。
	Variable VariableName
	// Ruleset は、Key が Ruleset の形のときだけ持つ。ほかの形ではゼロ値。
	Ruleset RulesetName
}

// Same は、u と other が同じ管理の単位かを返す。
// Environment と Ruleset の名前は、GitHub が同じ関連リソースとみなす名前どうしを同じ単位にするため、EqualFold で比べる。
// 変数の名前は英小文字を含まないため、== で比べれば足りる。
func (u Unit) Same(other Unit) bool {
	return u.Key == other.Key && u.Environment.EqualFold(other.Environment) && u.Variable == other.Variable &&
		u.Ruleset.EqualFold(other.Ruleset)
}

// String は、CLI 仕様の表示の形で識別を返す。
// Key の形と名前の有無が食い違う識別は、どの管理の単位か決まらないため、不変条件の破れとして panic する。
func (u Unit) String() string {
	area := u.Key.Area()
	hasVariable := area == AreaRepositoryVariables || area == AreaEnvironmentVariables
	if (u.Environment != EnvironmentName{}) != u.Key.HasEnvironmentName() || (u.Variable != VariableName{}) != hasVariable || (u.Ruleset != RulesetName{}) != (u.Key == RulesetResource) {
		panic(fmt.Sprintf("setting: unit %q has a mismatched name (environment %q, variable %q, ruleset %q)", u.Key, u.Environment, u.Variable, u.Ruleset))
	}
	switch u.Key {
	case EnvironmentResource:
		return "environments." + u.Environment.String()
	case EnvironmentDeploymentBranchPolicy:
		return "environments." + u.Environment.String() + ".deployment_branch_policy"
	case EnvironmentReviewerSet:
		return "environments." + u.Environment.String() + ".reviewers"
	case EnvironmentPreventSelfReview:
		return "environments." + u.Environment.String() + ".prevent_self_review"
	case EnvironmentWaitTimer:
		return "environments." + u.Environment.String() + ".wait_timer"
	case EnvironmentVariable:
		return "environments." + u.Environment.String() + ".variables." + u.Variable.String()
	case RepositoryVariable:
		return "actions.variables." + u.Variable.String()
	case RulesetResource:
		return "rulesets." + u.Ruleset.String()
	default:
		return string(u.Key)
	}
}
