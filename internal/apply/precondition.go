package apply

import (
	"fmt"
	"slices"

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
	if len(operation.PreconditionBases) == 0 {
		panic("apply: " + operation.Unit.String() + " has no precondition basis to check before the update")
	}
	if observed.RepositoryID != approvedID {
		return preconditionIdentityMismatch, setting.Unit{}
	}
	for _, basis := range operation.PreconditionBases {
		if !matchesPreconditionBasis(basis, observed, created) {
			return preconditionCurrentValueChanged, basis.Unit
		}
	}
	return preconditionHolds, setting.Unit{}
}

// matchesPreconditionBasis は、変更前提の基準 basis の設定が、更新直前の観測 observed で承認時に予定した値と同じかを返す。
// Environment そのものは、存在に加えて同一性を確かめるため、一覧から一度だけ引いた観測結果で両方を比べる。
func matchesPreconditionBasis(basis changeplan.PreconditionBasis, observed github.ObservationResult, created github.EnvironmentObservation) bool {
	switch basis.Unit.Key {
	case setting.EnvironmentResource:
		return matchesAnticipatedEnvironment(basis, observed.Environments.Lookup(basis.Unit.Environment), created)
	case setting.RulesetResource:
		return matchesAnticipatedRuleset(basis, observed.Rulesets.Lookup(basis.Unit.Ruleset))
	default:
		return matchesAnticipatedValue(basis.AnticipatedValue, observed.Setting(basis.Unit))
	}
}

// matchesAnticipatedValue は、関係する現在値 current が承認時に予定した値 anticipated と、ドメインモデルの承認と変更前提の意味で同じかを返す。
// 観測不能の理由は値に対して読めないため、両方が観測不能のときだけ理由を比べる。
func matchesAnticipatedValue(anticipated changeplan.AnticipatedValue, current github.SettingObservation[setting.Value]) bool {
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

// matchesAnticipatedEnvironment は、Environment そのものの基準 basis が、更新直前に引いた観測結果 current と同じかを返す。
// 存在はリポジトリ設定と同じ判定で比べる。判定できない名前が現れた観測は、存在しないことを意味しないため、存在しないと予定した基準と同じにならない。
// 存在すると予定したときは、さらに同じ実体かを確かめる。同じ名前で作り直された Environment は、承認した変更の相手ではないため、ID が違えば同じとしない。
// 綴りは比べない。中の設定は、削除の基準でだけ、承認の前に示した削除で失われるものとして、存在と同じ観測で比べる (matchesAnticipatedEnvironmentLoss)。
// 削除は中の設定ごと Environment を消し、承認のあとに変わった中身を示さないまま失わせるためである。ほかの基準では比べない。
// 同じ論理的な設定変更が作成した Environment は、作成に続く最初の更新操作では ID の基準がまだなく (created がゼロ値)、存在だけで足りる。
// 承認の時点で ID が定まらないため。それより後の更新操作では、created の ID と比べる。
// 存在するか存在しないかのどちらでもない基準 (観測不能、ゼロ値) は承認の前に適用不能として止め、存在すると予定した基準には changeplan が同一性の基準を必ず添え、
// 削除で失われるものを載せた基準には計画時の ID の基準を必ず添える。
// どれかが破れた形を受け付けると、確かめていない Environment を更新し得るため、更新直前の観測によらず不変条件の破れとして panic する。
func matchesAnticipatedEnvironment(basis changeplan.PreconditionBasis, current github.EnvironmentObservation, created github.EnvironmentObservation) bool {
	anticipated, ok := basis.AnticipatedValue.Value()
	if !ok {
		panic("apply: " + basis.Unit.String() + " is anticipated to be neither present nor absent")
	}
	_, hasID := basis.AnticipatedEnvironmentIdentity.ID()
	if anticipated.Equal(setting.Present()) && !hasID && !basis.AnticipatedEnvironmentIdentity.IsCreatedByPrecedingOperation() {
		panic("apply: " + basis.Unit.String() + " is anticipated to exist without an identity to check")
	}
	if basis.AnticipatedEnvironmentLoss != nil && !hasID {
		panic("apply: " + basis.Unit.String() + " has the loss of its deletion anticipated without an ID to check")
	}
	if !matchesAnticipatedValue(basis.AnticipatedValue, current.Existence()) {
		return false
	}
	existing, exists := current.Existing()
	if !exists {
		return true
	}
	if id, ok := basis.AnticipatedEnvironmentIdentity.ID(); ok {
		if existing.ID != id {
			return false
		}
		return basis.AnticipatedEnvironmentLoss == nil || matchesAnticipatedEnvironmentLoss(*basis.AnticipatedEnvironmentLoss, existing)
	}
	base, hasBase := created.Existing()
	return !hasBase || existing.ID == base.ID
}

// matchesAnticipatedEnvironmentLoss は、承認の前に示した削除で失われるもの anticipated が、更新直前に引いた Environment current と同じかを返す。
// 失われる値は照合と同じ判定で比べる。承認のあとに知らない種類の保護ルールか知らない値が現れて失われる値を示せなくなった Environment は、同じとしない。
// 変数と Secret の一覧は、件数が同じで、予定したどの要素にも同じ要素が現在の一覧にあれば同じとする。並びは GitHub の返す順で、承認の意味を持たないため比べない。
// 変数は名前と値を、Secret は名前と updated_at の文字列を比べる。Secret は値を読めないため、登録し直されたことを updated_at で見分ける。
// 変数か Secret の一覧を読んでいない Environment は、読む範囲の破れとして panic する。読んでいない一覧を空として比べると、承認のあとに足されたものごと消すため。
func matchesAnticipatedEnvironmentLoss(anticipated changeplan.EnvironmentLoss, current github.Environment) bool {
	if current.Variables == nil || current.Secrets == nil {
		panic(fmt.Sprintf("apply: environment %q is checked for its deletion without its variables and secrets read", current.Name))
	}
	deletedValue, ok := current.DeletedValue().Value()
	return ok && deletedValue.Equal(anticipated.DeletedValue) &&
		sameElements(anticipated.Variables, current.Variables.Existing, sameVariable) &&
		sameElements(anticipated.Secrets, current.Secrets.Existing, sameEnvironmentSecret)
}

// sameElements は、anticipated と current が、same で同じとする要素を並びによらず持つかを返す。
// 名前の重複のない一覧どうしを比べるため、件数が同じで、anticipated のどの要素にも同じ要素が current にあれば同じとする。
func sameElements[T any](anticipated, current []T, same func(T, T) bool) bool {
	if len(anticipated) != len(current) {
		return false
	}
	for _, a := range anticipated {
		if !slices.ContainsFunc(current, func(c T) bool { return same(a, c) }) {
			return false
		}
	}
	return true
}

// sameVariable は、変数 a と b の名前と値が同じかを返す。名前は、GitHub が大文字にして保存するため、照合と同じ鍵で比べる。
func sameVariable(a, b github.Variable) bool {
	return setting.FoldVariableName(a.Name) == setting.FoldVariableName(b.Name) && a.Value == b.Value
}

// sameEnvironmentSecret は、Secret a と b の名前と updated_at が同じかを返す。
// 名前は綴りのまま比べる。どちらも GitHub が保存した綴りで、綴りが変わるのは登録し直したときだけのため、大文字小文字を同一視する必要がない。
func sameEnvironmentSecret(a, b github.EnvironmentSecret) bool {
	return a.Name == b.Name && a.UpdatedAt == b.UpdatedAt
}

// matchesAnticipatedRuleset は、Ruleset そのものの基準 basis が、更新直前に名前で引いた観測結果 current と同じかを返す。
// 綴りは比べない。
// 存在するか存在しないかのどちらでもない基準 (観測不能、ゼロ値) は承認の前に適用不能として止め、値を伴う存在の基準には changeplan が ID の基準を必ず添える。
// どちらかが破れた形を受け付けると、確かめていない Ruleset を置き換えるか消し得るため、更新直前の観測によらず不変条件の破れとして panic する。
func matchesAnticipatedRuleset(basis changeplan.PreconditionBasis, current github.RulesetObservation) bool {
	anticipated, ok := basis.AnticipatedValue.Value()
	if !ok {
		panic("apply: " + basis.Unit.String() + " is anticipated to be neither present nor absent")
	}
	id, hasID := basis.AnticipatedRulesetIdentity.ID()
	if !anticipated.Equal(setting.Absent()) && !hasID {
		panic("apply: " + basis.Unit.String() + " is anticipated to exist without an identity to check")
	}
	if !matchesAnticipatedValue(basis.AnticipatedValue, current.SettingObservation()) {
		return false
	}
	existing, exists := current.Existing()
	return !exists || existing.ID == id
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
			for _, basis := range operation.PreconditionBases {
				if basis.AnticipatedEnvironmentIdentity.IsCreatedByPrecedingOperation() {
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
