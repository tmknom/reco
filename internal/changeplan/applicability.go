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
	// NotApplicableDependabotAlertsRemainDisabled は、Dependabot alerts が無効で、同じリポジトリの期待状態が Dependabot alerts の有効を含まないこと。
	NotApplicableDependabotAlertsRemainDisabled
	// NotApplicablePrivateRepository は、リポジトリが非公開で、その設定が非公開リポジトリでは利用できないこと。
	NotApplicablePrivateRepository
	// NotApplicableSecretScanningRemainsDisabled は、Secret scanning が無効で、同じリポジトリの期待状態が Secret scanning の有効を含まないこと。
	NotApplicableSecretScanningRemainsDisabled
)

// judgeApplicability は、管理設定の計画 s の観測結果と、同じリポジトリの管理設定の計画 settings の期待値と、
// 計画時の観測結果 observed (同じ設定の領域の未管理設定の値) から、変更予定を安全に適用できるかを判断し、適用不能ならその理由を返す。
// 不一致は、設定ごとの設計の更新方式で、前提の設定を持つ依存する設定を除いて追加の条件なしに適用できる。
// 比較不能は、設定ごとの設計の遷移表に挙がる観測不能に限って判断する。
// 遷移表にない観測不能を適用可能として黙って通さないよう、不変条件の破れとして panic する。
func judgeApplicability(s SettingPlan, settings []SettingPlan, observed github.ObservationResult) (Applicability, NotApplicableReason) {
	switch s.Result {
	case Match:
		return ApplicabilityNotJudged, 0
	case Mismatch:
		// GitHub は Dependabot alerts が無効のあいだ、Dependabot security updates の更新を向きによらず拒否する
		if s.Unit.Key == setting.DependabotSecurityUpdates && !prerequisiteAvailable(setting.DependabotAlerts, settings, observed) {
			return NotApplicable, NotApplicableDependabotAlertsRemainDisabled
		}
		// GitHub は Secret scanning が無効のあいだ、push protection を有効にする更新に成功を応答して値を変えない
		if s.Unit.Key == setting.RepositoryPushProtection && s.Desired.Bool() && !prerequisiteAvailable(setting.SecretScanningEnabled, settings, observed) {
			return NotApplicable, NotApplicableSecretScanningRemainsDisabled
		}
		return Applicable, 0
	case NotComparable:
	}
	reason := s.UnavailableReason()
	switch {
	case s.Unit.Key.HasEnvironmentName() && reason == github.UnavailableEnvironmentNameUndetermined:
		// 作成すれば、GitHub が同じとみなす既存の Environment を変え得る。同じリポジトリのほかの宣言では解消しない
		return NotApplicable, NotApplicableEnvironmentNameUndetermined
	case s.Unit.Key.HasEnvironmentName() && s.Unit.Key != setting.EnvironmentResource && reason == github.UnavailableEnvironmentAbsent:
		// Environment の宣言は、方式や変数だけを書いても存在の宣言を兼ねるため、同じ変更計画が先にその Environment を作成する
		if !createsEnvironment(settings, s.Unit.Environment) {
			panic(fmt.Sprintf("changeplan: %s is not comparable without the creation of its environment", s.Unit.String()))
		}
		return Applicable, 0
	case s.Unit.Key == setting.AllowedActions && reason == github.UnavailableActionsDisabled:
		return Applicable, 0
	case s.Unit.Key == setting.SelectedActions && (reason == github.UnavailableNotSelected || reason == github.UnavailableActionsDisabled):
		return Applicable, 0
	case s.Unit.Key == setting.SHAPinningRequired && reason == github.UnavailableActionsDisabled:
		if expectsActionsEnabled(settings) {
			return Applicable, 0
		}
		return NotApplicable, NotApplicableActionsRemainDisabled
	case s.Unit.Key == setting.ActionsAccessLevel && reason == github.UnavailablePublicRepository:
		// 公開範囲は reco が変える設定ではないため、同じリポジトリのほかの宣言では解消しない
		return NotApplicable, NotApplicablePublicRepository
	case reason == github.UnavailablePrivateRepository &&
		(s.Unit.Key.Area() == setting.AreaSecretScanning || s.Unit.Key == setting.PrivateVulnerabilityReportingEnabled):
		// 公開範囲は reco が変える設定ではないため、同じリポジトリのほかの宣言では解消しない
		return NotApplicable, NotApplicablePrivateRepository
	}
	panic(fmt.Sprintf("changeplan: %s is not comparable for an unexpected reason %d", s.Unit.String(), int(reason)))
}

// expectsActionsEnabled は、管理設定の計画 settings の期待状態が、Actions が有効であることを含むかを返す。
// Actions 許可ポリシーの期待値は、照合結果によらず Actions が有効であることを含む。
func expectsActionsEnabled(settings []SettingPlan) bool {
	return slices.ContainsFunc(settings, func(s SettingPlan) bool {
		return s.Unit.Key == setting.AllowedActions || (s.Unit.Key == setting.ActionsEnabled && s.Desired.Bool())
	})
}

// createsEnvironment は、管理設定の計画 settings が、name の Environment の作成を含むかを返す。
func createsEnvironment(settings []SettingPlan, name setting.EnvironmentName) bool {
	return slices.ContainsFunc(settings, func(s SettingPlan) bool {
		return s.IsEnvironmentCreation() && s.Unit.Environment.EqualFold(name)
	})
}

// prerequisiteAvailable は、依存する設定を更新する時点で、前提の設定 prerequisite が有効と見込めるかを返す。
// 計画時に有効か、同じ変更計画が前提の設定を有効にするときに限る。期待値どうしは互いの状態を含まないため、
// 依存する設定の期待値からは決めない。
func prerequisiteAvailable(prerequisite setting.Key, settings []SettingPlan, observed github.ObservationResult) bool {
	if v, ok := observed.Setting(setting.Unit{Key: prerequisite}).Value(); ok && v.Bool() {
		return true
	}
	return slices.ContainsFunc(settings, func(s SettingPlan) bool {
		return s.Unit.Key == prerequisite && s.Desired.Bool()
	})
}
