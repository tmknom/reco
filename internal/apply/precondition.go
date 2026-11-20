package apply

import (
	"github.com/tmknom/reco/internal/changeplan"
	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
	"github.com/tmknom/reco/internal/setting"
)

// preconditionCheck は、更新直前に変更前提を確かめた結果。
// 未実行の理由はこの値から導く。preconditionNotChecked は先行する異常による停止、preconditionHolds 以外の残りの値は変更前提の不成立に当たる。
// 理由を別のフィールドで持つと、前提は成立したのに理由は停止、のような矛盾した値を作れるため。
type preconditionCheck int

const (
	preconditionNotChecked preconditionCheck = iota
	preconditionUnconfirmed
	preconditionHolds
	preconditionIdentityMismatch
	preconditionCurrentValueChanged
)

// checkPrecondition は、承認した更新操作を更新直前の観測と照らし、更新してよいかを判定する。
// 関係する現在値の変化のときは、変わっていた設定のうち基準の並びで最初のものも返す。
// 並びは changeplan が報告に優先する順に並べている。
// 別のリポジトリの値を承認時に予定した値と比べても意味がないため、同一性を現在値より先に確かめる。
// 基準のない更新操作を成立にすると何も確かめずに更新するため、不変条件の破れとして panic する。
func checkPrecondition(approvedID repository.ID, operation changeplan.UpdateOperation, observed github.ObservationResult) (preconditionCheck, setting.Unit) {
	if len(operation.RelevantSettings) == 0 {
		panic("apply: " + operation.Unit.String() + " has no relevant settings to check before the update")
	}
	if observed.RepositoryID != approvedID {
		return preconditionIdentityMismatch, setting.Unit{}
	}
	for _, relevant := range operation.RelevantSettings {
		if !matchesRelevantSetting(relevant, observed) {
			return preconditionCurrentValueChanged, relevant.Unit
		}
	}
	return preconditionHolds, setting.Unit{}
}

// matchesRelevantSetting は、関係する現在値の設定 relevant が、更新直前の観測 observed で承認時に予定した値と同じかを返す。
// Environment そのものは値を持たないため、宣言の名前で一覧から引いた存在と ID を比べる。
func matchesRelevantSetting(relevant changeplan.RelevantSetting, observed github.ObservationResult) bool {
	if relevant.Unit.Key == setting.EnvironmentResource {
		return matchesEnvironmentAtApproval(relevant.EnvironmentAtApproval, observed.Environments.Lookup(relevant.Unit.Environment))
	}
	return matchesValueAtApproval(relevant.ValueAtApproval, observed.Setting(relevant.Unit))
}

// matchesValueAtApproval は、関係する現在値 current が承認時に予定した値 atApproval と、ドメインモデルの承認と変更前提の意味で同じかを返す。
// 観測不能の理由は観測値に対して読めないため、両方が観測不能のときだけ理由を比べる。
func matchesValueAtApproval(atApproval, current github.SettingObservation[setting.Value]) bool {
	approvedValue, approvedHasValue := atApproval.Value()
	currentValue, currentObserved := current.Value()
	switch {
	case approvedHasValue && currentObserved:
		return approvedValue.Equal(currentValue)
	case !approvedHasValue && !currentObserved:
		return atApproval.UnavailableReason() == current.UnavailableReason()
	default:
		return false
	}
}

// matchesEnvironmentAtApproval は、Environment の現在の観測結果 current が、承認時に予定した存在 atApproval と同じかを返す。
// 承認時に存在しなかったなら、現在も存在しないと確かめたときだけ同じとする。判定できない名前が現れた観測は、存在しないことを意味しないため。
// 承認時に存在したなら、現在も存在し ID が同じときだけ同じとする。同じ名前で作り直された Environment は、承認した変更の相手ではないため。
// 綴りと中の設定は比べない。
// 承認時に観測不能かゼロ値の存在は、承認の前に適用不能として止めるため、不変条件の破れとして panic する。
func matchesEnvironmentAtApproval(atApproval, current github.EnvironmentObservation) bool {
	if atApproval.Absent() {
		return current.Absent()
	}
	approved, ok := atApproval.Existing()
	if !ok {
		panic("apply: environment at approval is neither existing nor absent")
	}
	existing, ok := current.Existing()
	return ok && existing.ID == approved.ID
}

// precedingUpdateNotReflected は、observed が、直前の先行する更新操作の変更前提をそのまま満たすか、
// つまり先行する更新操作の直前の状態のままかを返す。先行する更新操作がなければ偽を返す。
// 先行する更新操作の直前の値でも予定どおりの値でもない値は、反映の遅れでは説明できないため偽になる。
func precedingUpdateNotReflected(approvedID repository.ID, preceding []changeplan.UpdateOperation, observed github.ObservationResult) bool {
	if len(preceding) == 0 {
		return false
	}
	check, _ := checkPrecondition(approvedID, preceding[len(preceding)-1], observed)
	return check == preconditionHolds
}
