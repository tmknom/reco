package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// Applicability は、変更予定 (不一致または比較不能と照合した設定) を安全に変更できるかの判断。
// ゼロ値の ApplicabilityNotJudged は、一致した設定に判断がないことを表し、三つ目の判断ではない。
type Applicability int

const (
	ApplicabilityNotJudged Applicability = iota
	Applicable
	NotApplicable
)

// NotApplicableReason は、変更予定が適用不能である理由。文言は表示が決める。
// ゼロ値は適用不能でないことを表し、定義済みの理由と取り違えないよう定数は 1 から始める。
type NotApplicableReason int

const (
	// NotApplicableActionsRemainDisabled は、Actions が無効で、同じリポジトリの期待状態が Actions の有効を含まないこと。
	NotApplicableActionsRemainDisabled NotApplicableReason = iota + 1
	// NotApplicablePublicRepository は、リポジトリが公開で、その設定が公開リポジトリに適用されないこと。
	NotApplicablePublicRepository
	// NotApplicableEnvironmentNameUndetermined は、宣言の名前と同じかどうかを判定できない名前の Environment があり、存在するかを確かめられないこと。
	NotApplicableEnvironmentNameUndetermined
)

// judgeApplicability は、管理設定の計画 s の観測結果と、同じリポジトリの管理設定の計画 settings の期待値から、
// 変更予定を安全に適用できるかを判断し、適用不能ならその理由を返す。
// 不一致は、アーキテクチャ設計の未管理設定を保全する更新方式で、追加の条件なしに適用できる。
// 比較不能は、アーキテクチャ設計の遷移表に挙がる観測不能に限って判断する。
// 遷移表にない観測不能を適用可能として黙って通さないよう、不変条件の破れとして panic する。
func judgeApplicability(s SettingPlan, settings []SettingPlan) (Applicability, NotApplicableReason) {
	switch s.Result {
	case Match:
		return ApplicabilityNotJudged, 0
	case Mismatch:
		return Applicable, 0
	case NotComparable:
	}
	reason := s.UnavailableReason()
	switch {
	case s.Key.Area() == setting.AreaEnvironments && reason == github.UnavailableEnvironmentNameUndetermined:
		// 作成すれば、GitHub が同じとみなす既存の Environment を変え得る。同じリポジトリのほかの宣言では解消しない
		return NotApplicable, NotApplicableEnvironmentNameUndetermined
	case s.Key == setting.EnvironmentDeploymentBranchPolicy && reason == github.UnavailableEnvironmentAbsent:
		// Environment の宣言は存在の宣言を兼ねるため、同じ変更計画が先にその Environment を作成する
		if !createsEnvironment(settings, s.Environment) {
			panic(fmt.Sprintf("changeplan: %s is not comparable without the creation of its environment", s.Unit().String()))
		}
		return Applicable, 0
	case s.Key == setting.AllowedActions && reason == github.UnavailableActionsDisabled:
		return Applicable, 0
	case s.Key == setting.SelectedActions && (reason == github.UnavailableNotSelected || reason == github.UnavailableActionsDisabled):
		return Applicable, 0
	case s.Key == setting.SHAPinningRequired && reason == github.UnavailableActionsDisabled:
		if expectsActionsEnabled(settings) {
			return Applicable, 0
		}
		return NotApplicable, NotApplicableActionsRemainDisabled
	case s.Key == setting.ActionsAccessLevel && reason == github.UnavailablePublicRepository:
		// 公開範囲は reco が変える設定ではないため、同じリポジトリのほかの宣言では解消しない
		return NotApplicable, NotApplicablePublicRepository
	}
	panic(fmt.Sprintf("changeplan: %s is not comparable for an unexpected reason %d", s.Unit().String(), int(reason)))
}

// expectsActionsEnabled は、管理設定の計画 settings の期待状態が、Actions が有効であることを含むかを返す。
// Actions 許可ポリシーの期待値は、照合結果によらず Actions が有効であることを含む。
func expectsActionsEnabled(settings []SettingPlan) bool {
	return slices.ContainsFunc(settings, func(s SettingPlan) bool {
		return s.Key == setting.AllowedActions || (s.Key == setting.ActionsEnabled && s.Desired.Bool())
	})
}

// createsEnvironment は、管理設定の計画 settings が、name の Environment の作成を含むかを返す。
func createsEnvironment(settings []SettingPlan, name setting.EnvironmentName) bool {
	return slices.ContainsFunc(settings, func(s SettingPlan) bool {
		return s.IsCreation() && s.Environment.EqualFold(name)
	})
}
