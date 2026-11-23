package changeplan

import (
	"fmt"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// variableOperationOf は、Variable の変更予定 s の更新操作を、承認時に予定した変数の値 anticipated を基準にして返す。
// 期待値が存在しないことなら削除で、送る値を持たない。期待値が値なら、anticipated が存在しないことなら作成、値なら値の変更である。
// 種類の組は宛先の違う Repository Variable と Environment Variable で分ける。
// 基準が観測不能なら値がゼロ値になり、Equal が不変条件の破れとして panic する。どの種類かを決められないため。
// 存在しない変数への削除も、照合が一致にして変更予定を作らないため、不変条件の破れとして panic する。
func variableOperationOf(s SettingPlan, anticipated github.SettingObservation[setting.Value]) UpdateOperation {
	var creation, valueUpdate, deletion UpdateOperationKind
	switch s.Unit.Key {
	case setting.RepositoryVariable:
		creation, valueUpdate, deletion = RepositoryVariableCreation, RepositoryVariableValueUpdate, RepositoryVariableDeletion
	case setting.EnvironmentVariable:
		creation, valueUpdate, deletion = EnvironmentVariableCreation, EnvironmentVariableValueUpdate, EnvironmentVariableDeletion
	default:
		panic(fmt.Sprintf("changeplan: %s is not a variable", s.Unit.Key))
	}
	value, _ := anticipated.Value()
	relevant := []RelevantSetting{{Unit: s.Unit, AnticipatedValue: anticipated}}
	if s.Desired.Equal(setting.Absent()) {
		if value.Equal(setting.Absent()) {
			panic(fmt.Sprintf("changeplan: deletion of %s is planned while it is absent", s.Unit.String()))
		}
		return UpdateOperation{Kind: deletion, Unit: s.Unit, RelevantSettings: relevant}
	}
	kind := valueUpdate
	if value.Equal(setting.Absent()) {
		kind = creation
	}
	return UpdateOperation{Kind: kind, Unit: s.Unit, Value: s.Desired, RelevantSettings: relevant}
}
