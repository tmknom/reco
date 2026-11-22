package changeplan

import (
	"fmt"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// variableOperationOf は、Variable の変更予定 s の更新操作を、承認時に予定した変数の値 anticipated を基準にして返す。
// anticipated が存在しないことなら作成、値なら値の変更で、種類の組は宛先の違う Repository Variable と Environment Variable で分ける。
// 基準が観測不能なら値がゼロ値になり、Equal が不変条件の破れとして panic する。作成か値の変更かを決められないため。
func variableOperationOf(s SettingPlan, anticipated github.SettingObservation[setting.Value]) UpdateOperation {
	var creation, valueUpdate UpdateOperationKind
	switch s.Unit.Key {
	case setting.RepositoryVariable:
		creation, valueUpdate = RepositoryVariableCreation, RepositoryVariableValueUpdate
	case setting.EnvironmentVariable:
		creation, valueUpdate = EnvironmentVariableCreation, EnvironmentVariableValueUpdate
	default:
		panic(fmt.Sprintf("changeplan: %s is not a variable", s.Unit.Key))
	}
	value, _ := anticipated.Value()
	kind := valueUpdate
	if value.Equal(setting.AbsentVariable()) {
		kind = creation
	}
	return UpdateOperation{Kind: kind, Unit: s.Unit, Value: s.Desired, RelevantSettings: []RelevantSetting{{Unit: s.Unit, AnticipatedValue: anticipated}}}
}
