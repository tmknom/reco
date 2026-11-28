package apply

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/changeplandisplay"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// unexpectedOutsideChange は、変更対象外の設定の予期しない変化を、表示の形の名前と値で持つ。
type unexpectedOutsideChange struct {
	Setting, Before, After string
}

// unexpectedOutsideChanges は、change の変更対象外の設定について、更新後の観測 after での予期しない変化を返す。
// before は、開始した更新操作の分だけの直前の観測である。
// ドメインモデルの観測の時点の定めに従い、更新エンドポイントごとに、それへ送った最初の更新操作の直前と比べる。
// Environment の PUT は、どの種類も、宛先の Environment のデプロイ元制限の方式と保護設定を、その更新操作の直前と比べる。同じ更新エンドポイントが受け付ける設定のため。
// 個別パターンの更新操作は、その更新エンドポイントで reco が観測するのが個別パターンの集合だけのため、比べない。
// Repository Variable と Environment Variable の更新操作も、その更新エンドポイントで reco が観測するのがその変数だけのため、比べない。
// Environment Variable の更新操作は Environment を宛先に含むが、Environment の保護設定は別の更新エンドポイントの設定である。
// Ruleset の更新エンドポイントが受け付けるのは Ruleset の値だけのため、ほかの設定は比べない。
// 値の変更は、値が省略したパラメータを送らないため、そのパラメータだけを比べる。作成は更新直前に、削除は更新後に、比べる値がない。
// 比べる範囲はアーキテクチャ設計の未管理設定を保全する更新方式と、設定ごとの設計の更新方式に従う。
// 未定義の種類は比べる範囲が決まらないため、不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func unexpectedOutsideChanges(change changeplan.LogicalChange, before []github.ObservationResult, after github.ObservationResult) []unexpectedOutsideChange {
	var changes []unexpectedOutsideChange
	var compared []github.UpdateEndpoint
	for i, b := range before {
		operation := change.Operations[i]
		switch operation.Kind {
		case changeplan.SettingUpdate, changeplan.MergeMethodsUpdate:
			endpoint, _ := operation.Endpoint()
			if !slices.Contains(compared, endpoint) {
				compared = append(compared, endpoint)
				changes = append(changes, outsideChangesAt(endpoint, change.ExcludedFromOutsideSettings(), b, after)...)
				changes = append(changes, oidcSubjectOmittedItemChanges(operation, b, after)...)
			}
			continue
		case changeplan.EnvironmentCreation, changeplan.EnvironmentCreationWithMode, changeplan.DeploymentBranchPolicyModeUpdate,
			changeplan.ReviewerSetUpdate, changeplan.PreventSelfReviewUpdate, changeplan.WaitTimerUpdate:
			changes = append(changes, environmentPutOutsideChanges(operation.Unit.Environment, change.ExcludedFromOutsideSettings(), b, after)...)
			continue
		case changeplan.RulesetValueUpdate:
			changes = append(changes, rulesetOmittedParameterChanges(operation, b, after)...)
			continue
		case changeplan.DeploymentPatternAddition, changeplan.DeploymentPatternRemoval,
			changeplan.RepositoryVariableCreation, changeplan.RepositoryVariableValueUpdate, changeplan.RepositoryVariableDeletion,
			changeplan.EnvironmentVariableCreation, changeplan.EnvironmentVariableValueUpdate, changeplan.EnvironmentVariableDeletion,
			changeplan.RulesetCreation, changeplan.RulesetDeletion:
			continue
		}
		panic(fmt.Sprintf("apply: undefined kind %d of an update operation", int(operation.Kind)))
	}
	return changes
}

// outsideChangesAt は、更新要求を送った endpoint で観測する設定のうち、変更対象外の設定から除く excluded に含まれないものについて、
// 更新直前と更新後の観測値の差を返す。
// 観測は設定の領域の単位で読むが、同じ設定の領域でも別の更新エンドポイントの設定は更新操作の影響を受けないため比べない。
// 片方でも観測不能な設定の比べ方は observationChange に従う。
func outsideChangesAt(endpoint github.UpdateEndpoint, excluded []setting.Unit, before, after github.ObservationResult) []unexpectedOutsideChange {
	var changes []unexpectedOutsideChange
	for _, key := range endpoint.ObservedSettings() {
		unit := setting.Unit{Key: key}
		if slices.Contains(excluded, unit) {
			continue
		}
		if b, a, changed := observationChange(before.Setting(unit), after.Setting(unit), setting.Value.Equal, setting.Value.String); changed {
			changes = append(changes, unexpectedOutsideChange{Setting: unit.String(), Before: b, After: a})
		}
	}
	return changes
}

// oidcSubjectOmittedItemChanges は、更新操作の値が省略した subject のカスタマイズの項目 (use_immutable_subject) について、更新直前と更新後の観測値の差を、CLI 仕様の変更対象外の設定の名前と値の表記で返す。
// 省略した項目は更新要求で送らず GitHub が保持する値に任せるため、期待状態の確認は比べない。そのため、更新操作の影響を受けていない保証をここで与える。
// 値が項目を宣言しているときは、期待状態の確認が比べるため返さない。
func oidcSubjectOmittedItemChanges(operation changeplan.UpdateOperation, before, after github.ObservationResult) []unexpectedOutsideChange {
	if operation.Unit.Key != setting.OIDCSubject || operation.Value.OIDCSubjectCustomization().UseImmutableSubject != nil {
		return nil
	}
	b := *before.OIDCSubject.Customization.UseImmutableSubject
	a := *after.OIDCSubject.Customization.UseImmutableSubject
	if b == a {
		return nil
	}
	return []unexpectedOutsideChange{{
		Setting: string(setting.OIDCSubject) + ".use_immutable_subject", Before: strconv.FormatBool(b), After: strconv.FormatBool(a),
	}}
}

// rulesetOmittedParameterChanges は、Ruleset の値の変更の更新操作の値が省略したパラメータについて、更新直前と更新後の観測値の差を、
// CLI 仕様の変更対象外の設定の名前と値の表記で返す。
// 省略したパラメータは送らず、期待状態の確認も比べないため、GitHub が決める値のまま保たれたかの保証をここで与える。
// 更新直前の観測は、値の変更の変更前提が成立した観測で、宛先の Ruleset の値を必ず持つ。
// 更新後の観測で値を読めなければ、比べる値がなく返さない。期待状態の確認が、不一致か確認失敗にする。
func rulesetOmittedParameterChanges(operation changeplan.UpdateOperation, before, after github.ObservationResult) []unexpectedOutsideChange {
	b, _ := before.Setting(operation.Unit).Value()
	a, afterOK := after.Setting(operation.Unit).Value()
	if !afterOK || a.Equal(setting.Absent()) {
		return nil
	}
	var changes []unexpectedOutsideChange
	for _, c := range b.Ruleset().OmittedParameterChangesTo(a.Ruleset(), operation.Value.Ruleset()) {
		changes = append(changes, unexpectedOutsideChange{Setting: operation.Unit.String() + "." + string(c.Rule) + "." + c.Name, Before: c.Before, After: c.After})
	}
	return changes
}

// environmentPutOutsideChanges は、宣言の名前 name の Environment のデプロイ元制限の方式と保護設定のうち、変更対象外の設定から除く excluded に含まれないものについて、
// 更新直前と更新後の観測の差を、CLI 仕様の変更対象外の設定の名前と値の表記で、方式、保護設定の項目の順に返す。
// 方式は方式の名前で示し、個別パターンは別の更新エンドポイントの設定のため比べない。
// 比べる範囲はアーキテクチャ設計の未管理設定を保全する更新方式と、設定ごとの設計の更新方式に従う。
// どちらかの観測で存在しない Environment は、比べる値がないため返さない。作成の直前がこれに当たる。
func environmentPutOutsideChanges(name setting.EnvironmentName, excluded []setting.Unit, before, after github.ObservationResult) []unexpectedOutsideChange {
	b, beforeOK := before.Environments.Lookup(name).Existing()
	a, afterOK := after.Environments.Lookup(name).Existing()
	if !beforeOK || !afterOK {
		return nil
	}
	var changes []unexpectedOutsideChange
	policy := setting.Unit{Key: setting.EnvironmentDeploymentBranchPolicy, Environment: name}
	if !slices.Contains(excluded, policy) && b.DeploymentBranchPolicyMode != a.DeploymentBranchPolicyMode {
		changes = append(changes, unexpectedOutsideChange{Setting: policy.String(), Before: string(b.DeploymentBranchPolicyMode), After: string(a.DeploymentBranchPolicyMode)})
	}
	return append(changes, environmentProtectionChanges(name, excluded, b.Protection, a.Protection)...)
}

// environmentProtectionChanges は、宣言の名前 name の Environment の保護設定の項目のうち、変更対象外の設定から除く excluded に含まれないものについて、
// 更新直前 before と更新後 after の差を、CLI 仕様の変更対象外の設定の名前と値の表記で、待機時間、担当者の集合、自己承認の禁止、can_admins_bypass の順に返す。
// 担当者の集合は、管理設定の照合と同じ判定で比べ、login を示さない。片方でも観測不能なときの比べ方は observationChange に従う。
// can_admins_bypass は宣言を受け付けない項目で、管理の単位に当たらないため、excluded によらず比べる。
func environmentProtectionChanges(name setting.EnvironmentName, excluded []setting.Unit, before, after github.EnvironmentProtection) []unexpectedOutsideChange {
	prefix := setting.Unit{Key: setting.EnvironmentResource, Environment: name}.String() + "."
	compared := func(key setting.Key) bool {
		return !slices.Contains(excluded, setting.Unit{Key: key, Environment: name})
	}
	var changes []unexpectedOutsideChange
	if compared(setting.EnvironmentWaitTimer) && before.WaitTimer != after.WaitTimer {
		changes = append(changes, unexpectedOutsideChange{
			Setting: prefix + setting.EnvironmentWaitTimer.DeclaredKeyName(),
			Before:  setting.WaitTimerValue(before.WaitTimer).String(), After: setting.WaitTimerValue(after.WaitTimer).String(),
		})
	}
	if compared(setting.EnvironmentReviewerSet) {
		if b, a, changed := observationChange(before.ReviewerSet, after.ReviewerSet, sameReviewerSet, reviewerSetText); changed {
			changes = append(changes, unexpectedOutsideChange{Setting: prefix + setting.EnvironmentReviewerSet.DeclaredKeyName(), Before: b, After: a})
		}
	}
	if compared(setting.EnvironmentPreventSelfReview) && before.PreventSelfReview != after.PreventSelfReview {
		changes = append(changes, unexpectedOutsideChange{
			Setting: prefix + setting.EnvironmentPreventSelfReview.DeclaredKeyName(),
			Before:  strconv.FormatBool(before.PreventSelfReview), After: strconv.FormatBool(after.PreventSelfReview),
		})
	}
	if before.CanAdminsBypass != after.CanAdminsBypass {
		changes = append(changes, unexpectedOutsideChange{
			Setting: prefix + "can_admins_bypass", Before: strconv.FormatBool(before.CanAdminsBypass), After: strconv.FormatBool(after.CanAdminsBypass),
		})
	}
	return changes
}

// observationChange は、変更対象外の設定の更新直前 before と更新後 after の観測結果が予期しない変化かを判定し、変化なら両側の表記と真を返す。
// 両方が観測値なら equal で比べ、text で示す。片方が知らない値で、もう片方が観測値なら、知らない値は reco の知るどの値とも異なり、値が変わったことは確かなため変化とし、
// 知らない値の側を変更予定の現在値と同じ表記で示す。上位の設定から理由を引き継いだ観測不能は、その設定の値が知らない値とは限らないため、比べる設定に含めない。
// それ以外の観測不能を含む組は、比べる値がなく変化としない。知らない値どうしも、同じ値かを判断できないため比べない。
func observationChange[T any](before, after github.SettingObservation[T], equal func(T, T) bool, text func(T) string) (string, string, bool) {
	b, beforeOK := before.Value()
	a, afterOK := after.Value()
	switch {
	case beforeOK && afterOK:
		return text(b), text(a), !equal(b, a)
	case beforeOK && after.HasUnknownValue():
		return text(b), changeplandisplay.UnknownValueText, true
	case afterOK && before.HasUnknownValue():
		return changeplandisplay.UnknownValueText, text(a), true
	}
	return "", "", false
}

// sameReviewerSet は、担当者の集合 a と b が、管理設定の照合と同じ判定で同じかを返す。
func sameReviewerSet(a, b setting.ReviewerSet) bool {
	return setting.ReviewerSetValue(a).Equal(setting.ReviewerSetValue(b))
}

// reviewerSetText は、担当者の集合を、CLI 仕様が変更対象外の設定に示す login のない表記で返す。
func reviewerSetText(s setting.ReviewerSet) string {
	return s.WithoutLogins().String()
}
