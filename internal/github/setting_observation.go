package github

// UnavailableReason は、観測不能の理由。
type UnavailableReason int

// ゼロ値を定義済みの理由と取り違えないよう 1 から始める。
const (
	// UnavailableActionsDisabled は、GitHub Actions が無効であることを表す。
	UnavailableActionsDisabled UnavailableReason = iota + 1
	// UnavailableNotSelected は、Actions 許可ポリシーが selected でないことを表す。
	UnavailableNotSelected
	// UnavailablePublicRepository は、リポジトリが公開であることを表す。
	UnavailablePublicRepository
	// UnavailableEnvironmentAbsent は、属する先の Environment が存在しないことを表す。
	UnavailableEnvironmentAbsent
	// UnavailableNameUndetermined は、引いた名前と同じ名前のリポジトリリソースはないが、同じかどうかを判定できない名前のリポジトリリソースがあることを表す。
	UnavailableNameUndetermined
	// UnavailablePrivateRepository は、リポジトリが非公開であることを表す。
	UnavailablePrivateRepository
	// UnavailableUninterpretedParts は、リポジトリリソースの値か、Environment の削除で失われる中の設定が、reco の解釈しない部分を含むことを表す。
	// 値の中の列挙の項目に GitHub が reco の知らない値を返したことも、解釈しない部分に数えてこの理由にする。
	UnavailableUninterpretedParts
	// UnavailableUnknownValue は、GitHub が値を足し得る列挙の設定に、GitHub が reco の知らない値を返したことを表す。
	// 値が要素の集合なら、どれか一つの要素の列挙の項目が知らない値であることを含む。
	// 上位の設定が知らない値のために取得条件を判断できない設定も、上位の設定と同じくこの理由にする。
	UnavailableUnknownValue
	// UnavailableNotReturned は、公式の仕様が省略を許す値を、GitHub が応答に含めなかったことを表す。
	UnavailableNotReturned
)

// SettingObservation は、一つの設定の観測結果で、観測値か、理由を伴う観測不能のどちらか一方を持つ。
// 値と理由を両方持つ状態を作れないよう、フィールドを公開せずコンストラクタでだけ作る。
// どちらも持たないゼロ値は作れるが、観測値としては読めず、理由を読もうとすると panic で検出する。
type SettingObservation[T any] struct {
	value    T
	observed bool
	reason   UnavailableReason
}

// Observed は、観測値を持つ設定の観測結果を作る。
func Observed[T any](value T) SettingObservation[T] {
	return SettingObservation[T]{value: value, observed: true}
}

// Unavailable は、理由 reason により観測不能だった設定の観測結果を作る。
func Unavailable[T any](reason UnavailableReason) SettingObservation[T] {
	return SettingObservation[T]{reason: reason}
}

// Value は観測値と真を返す。観測不能のときは T のゼロ値と偽を返し、呼び出し側に値として読む前の確認を強いる。
func (o SettingObservation[T]) Value() (T, bool) {
	return o.value, o.observed
}

// UnavailableReason は観測不能の理由を返す。観測値を持つとき、およびゼロ値では、不変条件の破れとして panic する。
// Observed は理由を持たないため、理由がないことだけで両方を検出できる。
func (o SettingObservation[T]) UnavailableReason() UnavailableReason {
	if o.reason == 0 {
		panic("github: setting observation is not unavailable")
	}
	return o.reason
}

// HasUnknownValue は、GitHub が reco の知らない値を返したことを理由とする観測不能かを返す。観測値なら偽を返す。
// 値を読む前に Value で確かめる手順を利用者に負わせず、ゼロ値は UnavailableReason と同じく panic で検出する。
func (o SettingObservation[T]) HasUnknownValue() bool {
	if o.observed {
		return false
	}
	return o.UnavailableReason() == UnavailableUnknownValue
}
