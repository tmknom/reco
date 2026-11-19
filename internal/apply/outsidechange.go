package apply

import (
	"slices"

	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/setting"
)

// unexpectedOutsideChange は、変更対象外の設定の予期しない変化。
type unexpectedOutsideChange struct {
	Key           setting.Key
	Before, After setting.Value
}

// unexpectedOutsideChanges は、change の変更対象外の設定について、更新後の観測 after での予期しない変化を返す。
// before は、開始した更新操作の分だけの直前の観測である。
// ドメインモデルの観測の時点の定めに従い、更新エンドポイントごとに、それへ送った最初の更新操作の直前と比べる。
func unexpectedOutsideChanges(change changeplan.LogicalChange, before []github.ObservationResult, after github.ObservationResult) []unexpectedOutsideChange {
	var changes []unexpectedOutsideChange
	var compared []github.UpdateEndpoint
	for i, b := range before {
		endpoint := github.UpdateEndpointOf(change.Operations[i].Key)
		if slices.Contains(compared, endpoint) {
			continue
		}
		compared = append(compared, endpoint)
		changes = append(changes, outsideChangesAt(endpoint, change.SettingsToChange(), b, after)...)
	}
	return changes
}

// outsideChangesAt は、更新要求を送った endpoint で観測する設定のうち、論理的な設定変更が変える changed に含まれないものについて、
// 更新直前と更新後の観測値の差を返す。
// 観測は設定の領域の単位で読むが、同じ設定の領域でも別の更新エンドポイントの設定は更新操作の影響を受けないため比べない。
// 予期しない変化は観測値どうしの差のため、片方でも観測不能な設定は、比べる観測値がなく返さない。
func outsideChangesAt(endpoint github.UpdateEndpoint, changed []setting.Key, before, after github.ObservationResult) []unexpectedOutsideChange {
	var changes []unexpectedOutsideChange
	for _, key := range endpoint.ObservedSettings() {
		if slices.Contains(changed, key) {
			continue
		}
		b, beforeOK := before.Setting(key).Value()
		a, afterOK := after.Setting(key).Value()
		if !beforeOK || !afterOK {
			continue
		}
		if !b.Equal(a) {
			changes = append(changes, unexpectedOutsideChange{Key: key, Before: b, After: a})
		}
	}
	return changes
}
