package changeplan

import (
	"fmt"
	"slices"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// Applicability は、変更予定 (不一致または比較不能と照合した設定) を安全に変更できるかの判断。
// ゼロ値は、一致した設定に判断がないことを表し、三つ目の判断ではない。
// フィールドを公開しないのは、適用不能だけが理由を伴い、理由に添える未管理設定や上限は伴う理由のときだけ持つ状態を、
// Applicable、NotApplicable、NotApplicableAboveMaximum を通してしか作れなくするため。
type Applicability struct {
	applicable        bool
	reason            NotApplicableReason
	unmanagedSettings []setting.Key
	maximumAllowed    int
}

// NotApplicableReason は、変更予定が適用不能である理由。文言は表示が決める。
// ゼロ値は適用不能でないことを表し、定義済みの理由と取り違えないよう定数は 1 から始める。
type NotApplicableReason int

const (
	// NotApplicableActionsRemainDisabled は、Actions が無効で、同じリポジトリの期待状態が Actions の有効を含まないこと。
	NotApplicableActionsRemainDisabled NotApplicableReason = iota + 1
	// NotApplicablePublicRepository は、リポジトリが公開で、その設定が公開リポジトリに適用されないこと。
	NotApplicablePublicRepository
	// NotApplicableNameUndetermined は、宣言の名前と同じかどうかを判定できない名前のリポジトリリソースがあり、宣言の名前のリポジトリリソースが存在するかを確かめられないこと。
	NotApplicableNameUndetermined
	// NotApplicableDependabotAlertsRemainDisabled は、Dependabot alerts が無効で、同じリポジトリの期待状態が Dependabot alerts の有効を含まないこと。
	NotApplicableDependabotAlertsRemainDisabled
	// NotApplicablePrivateRepository は、リポジトリが非公開で、その設定を非公開リポジトリでは変えられないこと。
	// 設定が非公開リポジトリに適用されないのか、reco が非公開リポジトリでは扱わないのかは、表示が設定から決める。
	NotApplicablePrivateRepository
	// NotApplicableSecretScanningRemainsDisabled は、Secret scanning が無効で、同じリポジトリの期待状態が Secret scanning の有効を含まないこと。
	NotApplicableSecretScanningRemainsDisabled
	// NotApplicableNoMergeMethodEnabled は、更新後に有効なマージ方式が一つも残らないこと。無効のまま残るマージ方式を伴う。
	NotApplicableNoMergeMethodEnabled
	// NotApplicableUninterpretedParts は、存在するリポジトリリソースの値が reco の解釈しない部分を含むこと。
	NotApplicableUninterpretedParts
	// NotApplicableAboveMaximumAllowed は、期待値が、計画時に観測した設定できる値の範囲の上限を超えること。上限を伴う。
	// この理由を持つのは保持日数の変更予定だけである。
	NotApplicableAboveMaximumAllowed
	// NotApplicableNoReviewers は、更新時点に Environment の担当者がいない状態になる見込みで、自己承認の禁止を送らずには期待値にならないこと。
	NotApplicableNoReviewers
	// NotApplicableMergeMethodRemainsDisabled は、マージメッセージに対応するマージ方式が無効で、同じリポジトリの期待状態がその有効を含まないこと。
	NotApplicableMergeMethodRemainsDisabled
	// NotApplicableUnknownValue は、現在の値が reco の知らない値であること。
	NotApplicableUnknownValue
	// NotApplicableNotReturned は、GitHub が現在の値を返さなかったこと。
	NotApplicableNotReturned
	// NotApplicableSecretScanningUnknown は、Secret scanning の有効状態が reco の知らない値で、同じリポジトリの期待状態が Secret scanning の有効を含まず、有効と確かめられないこと。
	// 無効であるとは確かめていないため、NotApplicableSecretScanningRemainsDisabled と分ける。
	NotApplicableSecretScanningUnknown
)

// Applicable は、適用可能の判断を返す。
func Applicable() Applicability {
	return Applicability{applicable: true}
}

// NotApplicable は、理由 reason による適用不能の判断を返す。unmanagedSettings は、理由に添える、成り立っていない条件に関わる未管理設定。
// 未管理設定を伴う理由は NotApplicableNoMergeMethodEnabled (無効のまま残るマージ方式) だけで、そのときは一つ以上を伴う。
// 理由がないか、理由と未管理設定の有無が食い違う判断と、上限を伴う理由は、表示が対処を示せないため不変条件の破れとして panic する。
// 上限を伴う理由は NotApplicableAboveMaximum で作る。
func NotApplicable(reason NotApplicableReason, unmanagedSettings ...setting.Key) Applicability {
	if reason == 0 || reason == NotApplicableAboveMaximumAllowed ||
		(reason == NotApplicableNoMergeMethodEnabled) != (len(unmanagedSettings) > 0) {
		panic(fmt.Sprintf("changeplan: not-applicable reason %d with %d unmanaged settings", int(reason), len(unmanagedSettings)))
	}
	return Applicability{reason: reason, unmanagedSettings: unmanagedSettings}
}

// NotApplicableAboveMaximum は、期待値が計画時に観測した上限 maximum を超えることによる適用不能の判断を返す。
func NotApplicableAboveMaximum(maximum int) Applicability {
	return Applicability{reason: NotApplicableAboveMaximumAllowed, maximumAllowed: maximum}
}

// IsApplicable は、適用可能と判断したかを返す。
func (a Applicability) IsApplicable() bool {
	return a.applicable
}

// IsNotApplicable は、適用不能と判断したかを返す。
func (a Applicability) IsNotApplicable() bool {
	return a.reason != 0
}

// Reason は、適用不能の理由を返す。適用不能でなければゼロ値を返す。
func (a Applicability) Reason() NotApplicableReason {
	return a.reason
}

// UnmanagedSettings は、適用不能の理由に添える未管理設定を返す。伴わなければ nil を返す。
func (a Applicability) UnmanagedSettings() []setting.Key {
	return a.unmanagedSettings
}

// MaximumAllowed は、適用不能の理由に添える、計画時に観測した上限を返す。伴わなければ 0 を返す。
func (a Applicability) MaximumAllowed() int {
	return a.maximumAllowed
}

// judgeApplicability は、管理設定の計画 s の観測結果と、同じリポジトリの管理設定の計画 settings の期待値と、
// 計画時の観測結果 observed (同じ設定の領域の未管理設定の値) から、変更予定を安全に適用できるかを判断する。
// 不一致は、設定ごとの設計の更新方式で、前提の設定を持つ依存する設定を除いて追加の条件なしに適用できる。
// 比較不能は、知らない値の観測不能を設定によらず適用不能とし (ドメインモデルの照合結果と適用可能性)、ほかは設定ごとの設計の遷移表に挙がる観測不能に限って判断する。
// 遷移表にない観測不能を適用可能として黙って通さないよう、不変条件の破れとして panic する。
func judgeApplicability(s SettingPlan, settings []SettingPlan, observed github.ObservationResult) Applicability {
	switch s.Result {
	case Match:
		return Applicability{}
	case Mismatch:
		// GitHub は Dependabot alerts が無効のあいだ、Dependabot security updates の更新を向きによらず拒否する
		if s.Unit.Key == setting.DependabotSecurityUpdates && !enabledAtUpdate(setting.DependabotAlerts, settings, observed) {
			return NotApplicable(NotApplicableDependabotAlertsRemainDisabled)
		}
		// GitHub は Secret scanning が無効のあいだ、push protection を有効にする更新に成功を応答して値を変えない
		if s.Unit.Key == setting.RepositoryPushProtection && s.Desired.Bool() && !enabledAtUpdate(setting.SecretScanningEnabled, settings, observed) {
			if observed.Setting(setting.Unit{Key: setting.SecretScanningEnabled}).HasUnknownValue() {
				return NotApplicable(NotApplicableSecretScanningUnknown)
			}
			return NotApplicable(NotApplicableSecretScanningRemainsDisabled)
		}
		// GitHub は対応するマージ方式が無効のあいだ、マージメッセージの更新を拒否する。reco はマージ方式を宣言なしに有効にしない
		if method, ok := s.Unit.Key.CorrespondingMergeMethod(); ok && !enabledAtUpdate(method, settings, observed) {
			return NotApplicable(NotApplicableMergeMethodRemainsDisabled)
		}
		// GitHub は上限を超える値の更新を拒否し、値を変えない
		if s.Unit.Key == setting.RetentionDays && s.Desired.RetentionDays() > observed.Retention.MaximumAllowedDays {
			return NotApplicableAboveMaximum(observed.Retention.MaximumAllowedDays)
		}
		if left := mergeMethodsLeftDisabled(s, settings, observed); len(left) > 0 {
			return NotApplicable(NotApplicableNoMergeMethodEnabled, left...)
		}
		if s.Unit.Key == setting.EnvironmentPreventSelfReview && !preventSelfReviewReachable(s, settings, observed) {
			return NotApplicable(NotApplicableNoReviewers)
		}
		return Applicable()
	case NotComparable:
	}
	reason := s.Observed.UnavailableReason()
	switch {
	case (s.Unit.Key.HasEnvironmentName() || s.Unit.Key == setting.RulesetResource) && reason == github.UnavailableNameUndetermined:
		// 作成すれば、GitHub が同じとみなす既存のリポジトリリソースを変え得る。存在しないことの宣言は、存在するかを確かめられず、存在するなら消える値を承認の前に示せない。同じリポジトリのほかの宣言では解消しない
		return NotApplicable(NotApplicableNameUndetermined)
	case s.Unit.Key == setting.RulesetResource && reason == github.UnavailableUninterpretedParts:
		// 期待値に変える更新は解釈しない部分も置き換え、削除は解釈しない部分も消し、reco が承認の前に示せない値を変えるか失わせる。同じリポジトリのほかの宣言では解消しない
		return NotApplicable(NotApplicableUninterpretedParts)
	case reason == github.UnavailableUnknownValue:
		// 置き換えで失われる値の意味を、reco が承認の前に示せない。設定によらず、同じリポジトリのほかの宣言では解消しない
		return NotApplicable(NotApplicableUnknownValue)
	case s.Unit.Key == setting.WebCommitSignoffRequired && reason == github.UnavailableNotReturned:
		// 更新のあとの観測でも値が返らなければ、期待状態に一致したかを確かめられない。同じリポジトリのほかの宣言では解消しない
		return NotApplicable(NotApplicableNotReturned)
	case s.Unit.Key.HasEnvironmentName() && s.Unit.Key != setting.EnvironmentResource && reason == github.UnavailableEnvironmentAbsent:
		// Environment の宣言は、方式や変数だけを書いても存在の宣言を兼ねるため、同じ変更計画が先にその Environment を作成する
		if !createsEnvironment(settings, s.Unit.Environment) {
			panic(fmt.Sprintf("changeplan: %s is not comparable without the creation of its environment", s.Unit.String()))
		}
		if s.Unit.Key == setting.EnvironmentPreventSelfReview && !preventSelfReviewReachable(s, settings, observed) {
			return NotApplicable(NotApplicableNoReviewers)
		}
		return Applicable()
	case s.Unit.Key == setting.AllowedActions && reason == github.UnavailableActionsDisabled:
		return Applicable()
	case s.Unit.Key == setting.SelectedActions && (reason == github.UnavailableNotSelected || reason == github.UnavailableActionsDisabled):
		return Applicable()
	case s.Unit.Key == setting.SHAPinningRequired && reason == github.UnavailableActionsDisabled:
		if expectsActionsEnabled(settings) {
			return Applicable()
		}
		return NotApplicable(NotApplicableActionsRemainDisabled)
	case (s.Unit.Key == setting.ActionsAccessLevel || s.Unit.Key == setting.PrivateForkPullRequestWorkflows) && reason == github.UnavailablePublicRepository:
		// 公開範囲は reco が変える設定ではないため、同じリポジトリのほかの宣言では解消しない
		return NotApplicable(NotApplicablePublicRepository)
	case reason == github.UnavailablePrivateRepository &&
		(s.Unit.Key.Area() == setting.AreaSecretScanning || s.Unit.Key == setting.PrivateVulnerabilityReportingEnabled || s.Unit.Key == setting.RulesetResource ||
			s.Unit.Key == setting.ForkPullRequestApproval):
		// 公開範囲は reco が変える設定ではないため、同じリポジトリのほかの宣言では解消しない
		return NotApplicable(NotApplicablePrivateRepository)
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

// preventSelfReviewReachable は、自己承認の禁止の変更予定 s の期待値に、更新時点で到達できる見込みかを返す。
// GitHub は担当者のいない状態への prevent_self_review の更新を、値によらず拒むため、更新時点に担当者がいれば到達でき、
// いなければ、担当者をすべて外す変更か作成が送らずに偽にする場合に限って到達できる (Environments の設計の適用可能性と副作用)。
// 更新時点の担当者は、担当者の集合を宣言していればその期待値、宣言していなければ計画時の観測値で決める。宣言していない担当者の集合は reco が変えないため。
// 呼び出し元は、s を存在する Environment の不一致か、同じ変更計画が作成する Environment の比較不能に限る。
// 宣言していない担当者の集合が知らない値で観測不能なら、担当者がいる。観測不能になるのは、知らない種類の担当者が一人以上いるときに限るため。
// ほかに担当者の集合を観測できないのは作成する Environment だけで、そのときの担当者は作成直後の値 (担当者がいない) とする。
func preventSelfReviewReachable(s SettingPlan, settings []SettingPlan, observed github.ObservationResult) bool {
	reviewersUnit := setting.Unit{Key: setting.EnvironmentReviewerSet, Environment: s.Unit.Environment}
	reviewers, declared := RepositoryPlan{Settings: settings}.settingPlanOf(reviewersUnit)
	atUpdate := reviewers.Desired
	if !declared {
		observedReviewers := observed.Setting(reviewersUnit)
		if observedReviewers.HasUnknownValue() {
			return true
		}
		current, ok := observedReviewers.Value()
		if !ok {
			current = createdValueOf(setting.EnvironmentReviewerSet)
		}
		atUpdate = current
	}
	if !atUpdate.ReviewerSet().HasNoReviewers() {
		return true
	}
	return !s.Desired.Bool() && (declared || createsEnvironment(settings, s.Unit.Environment))
}

// createsEnvironment は、管理設定の計画 settings が、name の Environment の作成を含むかを返す。
func createsEnvironment(settings []SettingPlan, name setting.EnvironmentName) bool {
	return slices.ContainsFunc(settings, func(s SettingPlan) bool {
		return s.IsEnvironmentCreation() && s.Unit.Environment.EqualFold(name)
	})
}

// enabledAtUpdate は、ある設定を更新する時点で、その更新に有効でなければならない設定 required (前提の設定か、対応するマージ方式) が有効と見込めるかを返す。
// 計画時に有効か、同じ変更計画が required を有効にするときに限る。計画時に有効で同じ変更計画が無効にするときも真を返す。
// そのときは実行順で、無効にする変更より先に更新するため。期待値どうしは互いの状態を含まないため、更新する設定の期待値からは決めない。
func enabledAtUpdate(required setting.Key, settings []SettingPlan, observed github.ObservationResult) bool {
	if v, ok := observed.Setting(setting.Unit{Key: required}).Value(); ok && v.Bool() {
		return true
	}
	return slices.ContainsFunc(settings, func(s SettingPlan) bool {
		return s.Unit.Key == required && s.Desired.Bool()
	})
}
