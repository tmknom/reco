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
func checkPrecondition(approvedID repository.ID, operation changeplan.UpdateOperation, observed github.ObservationResult) (preconditionCheck, setting.Key) {
	if len(operation.RelevantSettings) == 0 {
		panic("apply: " + string(operation.Key) + " has no relevant settings to check before the update")
	}
	if observed.RepositoryID != approvedID {
		return preconditionIdentityMismatch, ""
	}
	for _, relevant := range operation.RelevantSettings {
		if !matchesValueAtApproval(relevant.AtApproval, observed.Setting(relevant.Key)) {
			return preconditionCurrentValueChanged, relevant.Key
		}
	}
	return preconditionHolds, ""
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
