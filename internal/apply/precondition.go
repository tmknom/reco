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
// created は、同じ論理的な設定変更が作成した Environment の ID の基準である。
// 基準のない更新操作を成立にすると何も確かめずに更新するため、不変条件の破れとして panic する。
func checkPrecondition(approvedID repository.ID, operation changeplan.UpdateOperation, observed github.ObservationResult, created github.EnvironmentObservation) (preconditionCheck, setting.Unit) {
	if len(operation.RelevantSettings) == 0 {
		panic("apply: " + operation.Unit.String() + " has no relevant settings to check before the update")
	}
	if observed.RepositoryID != approvedID {
		return preconditionIdentityMismatch, setting.Unit{}
	}
	for _, relevant := range operation.RelevantSettings {
		if !matchesRelevantSetting(relevant, observed, created) {
			return preconditionCurrentValueChanged, relevant.Unit
		}
	}
	return preconditionHolds, setting.Unit{}
}

// matchesRelevantSetting は、関係する現在値の設定 relevant が、更新直前の観測 observed で承認時に予定した値と同じかを返す。
// Environment そのものは値を持たないため、宣言の名前で一覧から引いた存在と ID を比べる。
// 先行する更新操作が作成する Environment は AnticipatedEnvironment を持たないため、真偽値を先に読み、作成した Environment の基準 created と比べる。
func matchesRelevantSetting(relevant changeplan.RelevantSetting, observed github.ObservationResult, created github.EnvironmentObservation) bool {
	if relevant.Unit.Key == setting.EnvironmentResource {
		current := observed.Environments.Lookup(relevant.Unit.Environment)
		if relevant.CreatedByPrecedingOperation {
			return matchesCreatedEnvironment(created, current)
		}
		return matchesAnticipatedEnvironment(relevant.AnticipatedEnvironment, current)
	}
	return matchesAnticipatedValue(relevant.AnticipatedValue, observed.Setting(relevant.Unit))
}

// matchesAnticipatedValue は、関係する現在値 current が承認時に予定した値 anticipated と、ドメインモデルの承認と変更前提の意味で同じかを返す。
// 観測不能の理由は観測値に対して読めないため、両方が観測不能のときだけ理由を比べる。
func matchesAnticipatedValue(anticipated, current github.SettingObservation[setting.Value]) bool {
	anticipatedValue, anticipatedHasValue := anticipated.Value()
	currentValue, currentObserved := current.Value()
	switch {
	case anticipatedHasValue && currentObserved:
		return anticipatedValue.Equal(currentValue)
	case !anticipatedHasValue && !currentObserved:
		return anticipated.UnavailableReason() == current.UnavailableReason()
	default:
		return false
	}
}

// matchesAnticipatedEnvironment は、Environment の現在の観測結果 current が、承認時に予定した存在 anticipated と同じかを返す。
// 承認時に存在しなかったなら、現在も存在しないと確かめたときだけ同じとする。判定できない名前が現れた観測は、存在しないことを意味しないため。
// 承認時に存在したなら、現在も存在し ID が同じときだけ同じとする。同じ名前で作り直された Environment は、承認した変更の相手ではないため。
// 綴りと中の設定は比べない。
// 承認時に観測不能かゼロ値の存在は、承認の前に適用不能として止めるため、不変条件の破れとして panic する。
func matchesAnticipatedEnvironment(anticipated, current github.EnvironmentObservation) bool {
	if anticipated.Absent() {
		return current.Absent()
	}
	anticipatedEnvironment, ok := anticipated.Existing()
	if !ok {
		panic("apply: anticipated environment is neither existing nor absent")
	}
	existing, ok := current.Existing()
	return ok && existing.ID == anticipatedEnvironment.ID
}

// matchesCreatedEnvironment は、同じ論理的な設定変更が作成した Environment の現在の観測結果 current が、予定どおりかを返す。
// 作成に続く最初の更新操作では ID の基準がまだなく (created がゼロ値)、存在することだけを確かめる。承認の時点で ID が定まらないため。
// それより後の更新操作では、存在し、ID が created と同じときだけ予定どおりとする。作成のあとに作り直された Environment は、承認した変更の相手ではないため。
func matchesCreatedEnvironment(created, current github.EnvironmentObservation) bool {
	existing, ok := current.Existing()
	if !ok {
		return false
	}
	base, hasBase := created.Existing()
	return !hasBase || existing.ID == base.ID
}

// environmentCreatedIn は、change が先頭の更新操作で作成する Environment の ID の基準を、更新直前の観測の並び before から引く。
// 基準は、作成に続く最初の更新操作の変更前提が成立した観測 before[1] の、その名前の Environment である。
// 基準がないことはゼロ値で表す。
// before[1] は Environment が存在することを確かめて変更前提が成立した観測のため、存在しなければ不変条件の破れとして panic する。
// 作成でない変更が、先行する更新操作による作成を基準に持つ形も、ID を比べないまま更新することになるため、不変条件の破れとして panic する。
func environmentCreatedIn(change changeplan.LogicalChange, before []github.ObservationResult) github.EnvironmentObservation {
	creation := change.Operations[0]
	if !creation.CreatesEnvironment() {
		for _, operation := range change.Operations {
			for _, relevant := range operation.RelevantSettings {
				if relevant.CreatedByPrecedingOperation {
					panic("apply: " + operation.Unit.String() + " expects an environment created by a preceding update in a change without a creation")
				}
			}
		}
		return github.EnvironmentObservation{}
	}
	if len(before) < 2 {
		return github.EnvironmentObservation{}
	}
	created := before[1].Environments.Lookup(creation.Unit.Environment)
	if _, ok := created.Existing(); !ok {
		panic("apply: " + creation.Unit.String() + " does not exist in the read before the first update after its creation")
	}
	return created
}

// precedingUpdateNotReflected は、observed が、直前の先行する更新操作の変更前提をそのまま満たすか、
// つまり先行する更新操作の直前の状態のままかを返す。先行する更新操作がなければ偽を返す。
// 先行する更新操作の直前の値でも予定どおりの値でもない値は、反映の遅れでは説明できないため偽になる。
// created は、後続の更新操作の変更前提と同じ、作成した Environment の基準である。
func precedingUpdateNotReflected(approvedID repository.ID, preceding []changeplan.UpdateOperation, observed github.ObservationResult, created github.EnvironmentObservation) bool {
	if len(preceding) == 0 {
		return false
	}
	check, _ := checkPrecondition(approvedID, preceding[len(preceding)-1], observed, created)
	return check == preconditionHolds
}
