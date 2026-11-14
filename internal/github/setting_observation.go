package github

// UnavailableReason は、取得条件による観測不能の理由。
type UnavailableReason int

// ゼロ値を定義済みの理由と取り違えないよう 1 から始める。
const (
	// UnavailableActionsDisabled は、GitHub Actions が無効であることを表す。
	UnavailableActionsDisabled UnavailableReason = iota + 1
	// UnavailableNotSelected は、Actions 許可ポリシーが selected でないことを表す。
	UnavailableNotSelected
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

// Unavailable は、取得条件により観測不能だった設定の観測結果を作る。
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
