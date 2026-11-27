package github

import (
	"fmt"

	"github.com/tmknom/reco/internal/setting"
)

// EnvironmentObservation は、同一性を判定できる名前で引いた一つの Environment の関連リソースの観測結果。存在する、存在しない、理由を伴う観測不能のいずれか一つを持つ。
// 状態を取り違えた値を作れないよう、フィールドを公開せず Environments.Lookup でだけ作る。
// どの状態でもないゼロ値は作れるが、理由を読もうとすると panic で検出する。
type EnvironmentObservation struct {
	state       resourceState
	environment Environment
	reason      UnavailableReason
}

// Existing は、存在するときだけ、観測した Environment を GitHub の綴りと ID のまま返し、真を返す。
func (o EnvironmentObservation) Existing() (Environment, bool) {
	if o.state != resourceExisting {
		return Environment{}, false
	}
	return o.environment, true
}

// Absent は、存在しないと確かめたときだけ真を返す。観測不能は存在しないことを意味しない。
func (o EnvironmentObservation) Absent() bool {
	return o.state == resourceAbsent
}

// UnavailableReason は観測不能の理由を返す。観測不能でないとき、およびゼロ値では、不変条件の破れとして panic する。
func (o EnvironmentObservation) UnavailableReason() UnavailableReason {
	if o.state != resourceUnavailable {
		panic("github: environment observation is not unavailable")
	}
	return o.reason
}

// Existence は、その Environment そのものの観測結果を、存在するなら Present、存在しないなら Absent の観測値で返す。
// 照合、変更前提の確認、期待状態の確認が、リポジトリ設定と同じ判定で存在を比べられるようにするため。ID は含めず、Existing で読む。
func (o EnvironmentObservation) Existence() SettingObservation[setting.Value] {
	switch o.state {
	case resourceExisting:
		return Observed(setting.Present())
	case resourceAbsent:
		return Observed(setting.Absent())
	default:
		// 観測不能のときは同じ理由にし、ゼロ値は UnavailableReason の panic で検出する
		return Unavailable[setting.Value](o.UnavailableReason())
	}
}

// DeploymentBranchPolicy は、その Environment の中のデプロイ元制限の設定の観測結果を返す。
// 存在しない Environment の中の設定は値として読めないため、存在しないことを理由にした観測不能にする。
func (o EnvironmentObservation) DeploymentBranchPolicy() SettingObservation[setting.Value] {
	return o.observeSetting(func(e Environment) setting.Value {
		return setting.DeploymentBranchPolicyValue(e.DeploymentBranchPolicy())
	})
}

// RequiredReviewers は、その Environment の中の担当者の集合の設定の観測結果を返す。
// 存在しない Environment の中の設定は値として読めないため、存在しないことを理由にした観測不能にする。
func (o EnvironmentObservation) RequiredReviewers() SettingObservation[setting.Value] {
	return o.observeSetting(func(e Environment) setting.Value {
		return setting.ReviewerSetValue(setting.ReviewerSetOfRule(e.Protection.RequiredReviewers))
	})
}

// PreventSelfReview は、その Environment の中の自己承認の禁止の設定の観測結果を返す。保護ルールがなければ false の観測値になる。
// 存在しない Environment の中の設定は値として読めないため、存在しないことを理由にした観測不能にする。
func (o EnvironmentObservation) PreventSelfReview() SettingObservation[setting.Value] {
	return o.observeSetting(func(e Environment) setting.Value {
		return setting.BoolValue(e.Protection.PreventSelfReview())
	})
}

// WaitTimer は、その Environment の中のデプロイの待機時間の設定の観測結果を返す。保護ルールがなければ 0 分の観測値になる。
// 存在しない Environment の中の設定は値として読めないため、存在しないことを理由にした観測不能にする。
func (o EnvironmentObservation) WaitTimer() SettingObservation[setting.Value] {
	return o.observeSetting(func(e Environment) setting.Value {
		return setting.WaitTimerValue(e.Protection.WaitTimer)
	})
}

// observeSetting は、存在する Environment の設定の値を read で取り出して観測値にする。
// 存在しなければ存在しないことを理由にした観測不能に、観測不能なら同じ理由にする。
func (o EnvironmentObservation) observeSetting(read func(Environment) setting.Value) SettingObservation[setting.Value] {
	switch o.state {
	case resourceExisting:
		return Observed(read(o.environment))
	case resourceAbsent:
		return Unavailable[setting.Value](UnavailableEnvironmentAbsent)
	default:
		// 観測不能のときは同じ理由にし、ゼロ値は UnavailableReason の panic で検出する
		return Unavailable[setting.Value](o.UnavailableReason())
	}
}

// Variable は、その Environment に属する、名前 name の Environment Variable の観測結果を返す。
// 存在しない Environment の変数は値として読めないため、存在しないことを理由にした観測不能にする。
// 存在する Environment の Variables が nil なのは、変数の領域を読まなかった観測か、その Environment を読む範囲に含めなかった観測である。
// どちらも変数が一つもないこととは異なるため、不変条件の破れとして、どの Environment かを示して panic する。
func (o EnvironmentObservation) Variable(name setting.VariableName) SettingObservation[setting.Value] {
	switch o.state {
	case resourceExisting:
		if o.environment.Variables == nil {
			panic(fmt.Sprintf("github: variables of environment %q were not read in this observation", o.environment.Name))
		}
		return o.environment.Variables.Lookup(name)
	case resourceAbsent:
		return Unavailable[setting.Value](UnavailableEnvironmentAbsent)
	default:
		// 観測不能のときは同じ理由にし、ゼロ値は UnavailableReason の panic で検出する
		return Unavailable[setting.Value](o.UnavailableReason())
	}
}
