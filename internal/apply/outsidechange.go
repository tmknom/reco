package apply

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/tmknom/reco/internal/changeplan"
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
// Environment の PUT (作成と方式の変更) は、宛先の Environment の保護設定を、その更新操作の直前と比べる。
// 個別パターンの更新操作は、その更新エンドポイントで reco が観測するのが個別パターンの集合だけのため、比べない。
// 比べる範囲はアーキテクチャ設計の未管理設定を保全する更新方式に従う。
// 未定義の種類は比べる範囲が決まらないため、不変条件の破れとして panic する。定数を足して case を足し忘れたことを lint の exhaustive が検出するよう、default を置かない。
func unexpectedOutsideChanges(change changeplan.LogicalChange, before []github.ObservationResult, after github.ObservationResult) []unexpectedOutsideChange {
	var changes []unexpectedOutsideChange
	var compared []github.UpdateEndpoint
	for i, b := range before {
		operation := change.Operations[i]
		switch operation.Kind {
		case changeplan.SettingUpdate:
			endpoint := github.UpdateEndpointOf(operation.Unit.Key)
			if !slices.Contains(compared, endpoint) {
				compared = append(compared, endpoint)
				changes = append(changes, outsideChangesAt(endpoint, change.ExcludedFromOutsideSettings(), b, after)...)
			}
			continue
		case changeplan.EnvironmentCreation, changeplan.EnvironmentCreationWithMode, changeplan.DeploymentBranchPolicyModeUpdate:
			changes = append(changes, environmentProtectionChanges(operation.Unit.Environment, b, after)...)
			continue
		case changeplan.DeploymentPatternAddition, changeplan.DeploymentPatternRemoval:
			continue
		}
		panic(fmt.Sprintf("apply: undefined kind %d of an update operation", int(operation.Kind)))
	}
	return changes
}

// outsideChangesAt は、更新要求を送った endpoint で観測する設定のうち、変更対象外の設定から除く excluded に含まれないものについて、
// 更新直前と更新後の観測値の差を返す。
// 観測は設定の領域の単位で読むが、同じ設定の領域でも別の更新エンドポイントの設定は更新操作の影響を受けないため比べない。
// 予期しない変化は観測値どうしの差のため、片方でも観測不能な設定は、比べる観測値がなく返さない。
func outsideChangesAt(endpoint github.UpdateEndpoint, excluded []setting.Unit, before, after github.ObservationResult) []unexpectedOutsideChange {
	var changes []unexpectedOutsideChange
	for _, key := range endpoint.ObservedSettings() {
		unit := setting.Unit{Key: key}
		if slices.Contains(excluded, unit) {
			continue
		}
		b, beforeOK := before.Setting(unit).Value()
		a, afterOK := after.Setting(unit).Value()
		if !beforeOK || !afterOK {
			continue
		}
		if !b.Equal(a) {
			changes = append(changes, unexpectedOutsideChange{Setting: unit.String(), Before: b.String(), After: a.String()})
		}
	}
	return changes
}

// environmentProtectionChanges は、宣言の名前 name の Environment の保護設定について、更新直前と更新後の観測の差を CLI 仕様の変更結果の並びで返す。
// 比べる範囲はアーキテクチャ設計の未管理設定を保全する更新方式に従う。
// どちらかの観測で存在しない Environment は、比べる値がないため返さない。
func environmentProtectionChanges(name setting.EnvironmentName, before, after github.ObservationResult) []unexpectedOutsideChange {
	b, beforeOK := before.Environments.Lookup(name).Existing()
	a, afterOK := after.Environments.Lookup(name).Existing()
	if !beforeOK || !afterOK {
		return nil
	}
	prefix := setting.Unit{Key: setting.EnvironmentResource, Environment: name}.String()
	var changes []unexpectedOutsideChange
	if !b.Protection.SameWaitTimer(a.Protection) {
		changes = append(changes, unexpectedOutsideChange{Setting: prefix + ".wait_timer", Before: b.Protection.WaitTimerString(), After: a.Protection.WaitTimerString()})
	}
	if !b.Protection.SameRequiredReviewers(a.Protection) {
		changes = append(changes, unexpectedOutsideChange{
			Setting: prefix + ".required_reviewers", Before: b.Protection.RequiredReviewersString(), After: a.Protection.RequiredReviewersString(),
		})
	}
	if b.Protection.CanAdminsBypass != a.Protection.CanAdminsBypass {
		changes = append(changes, unexpectedOutsideChange{
			Setting: prefix + ".can_admins_bypass", Before: strconv.FormatBool(b.Protection.CanAdminsBypass), After: strconv.FormatBool(a.Protection.CanAdminsBypass),
		})
	}
	return changes
}
