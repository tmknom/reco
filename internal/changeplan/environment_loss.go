package changeplan

import (
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// EnvironmentLoss は、Environment の削除の変更予定が承認の前に示し、変更前提で計画時のままかを確かめるもの。
// 削除で失われる値と、変更に伴う副作用として一緒に消える Environment Variable と Environment Secret の一覧からなる。
// 計画 (SettingPlan.EnvironmentLoss) と変更前提の基準 (PreconditionBasis.AnticipatedEnvironmentLoss) が同じポインタを持ち、
// 示した内容と比べる内容を同じ値から読む。
type EnvironmentLoss struct {
	DeletedValue setting.EnvironmentDeletedValue
	// Variables は、一緒に消える Environment Variable を、計画時に観測した並びで持つ。
	Variables []github.Variable
	// Secrets は、一緒に消える Environment Secret を、計画時に観測した並びで持つ。
	Secrets []github.EnvironmentSecret
}

// environmentLossOf は、存在しないことを期待値 desired にした Environment そのものの管理の単位 unit について、計画時に存在を観測し、
// 削除で失われる値の全体を読めたときだけ、削除で失われるものを返す。ほかの管理の単位、存在の宣言、存在しない Environment、
// 失われる値を示せない Environment では nil を返す。示せない Environment の削除は適用不能で、承認の対象にならないため (judgeApplicability)。
// 変数か Secret の一覧を読んでいなければ、読む範囲の破れとして panic する。読んでいない一覧を空として示すと、一緒に消えるものを示さずに承認を求めるため。
func environmentLossOf(unit setting.Unit, desired setting.Value, observed github.ObservationResult) *EnvironmentLoss {
	if !declaresEnvironmentAbsent(unit, desired) {
		return nil
	}
	environment, ok := observed.Environments.Lookup(unit.Environment).Existing()
	if !ok {
		return nil
	}
	if environment.Variables == nil || environment.Secrets == nil {
		panic("changeplan: " + unit.String() + " is declared absent without its variables and secrets read")
	}
	deletedValue, ok := environment.DeletedValue().Value()
	if !ok {
		return nil
	}
	return &EnvironmentLoss{DeletedValue: deletedValue, Variables: environment.Variables.Existing, Secrets: environment.Secrets.Existing}
}
