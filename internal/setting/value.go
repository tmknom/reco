package setting

import (
	"slices"
	"strconv"
)

// Value は、一つの設定の値。期待値と観測値のどちらも、同じ設定についてはこの値で表す。
// 種別の違う設定の値を一つの並びで扱うため、値の種別を持つ。
// ゼロ値は値がないことを表す。
// 種別ごとに使わないフィールドをゼロ値に保つため、フィールドを公開せず、コンストラクタでだけ作る。
// 二つの値が同じかどうかは Equal で判定する。許可パターンのスライスを持つため、== はコンパイルエラーになる。
type Value struct {
	kind     valueKind
	boolean  bool
	level    PermissionLevel
	policy   AllowedActionsPolicy
	criteria SelectedActionsCriteria
}

type valueKind int

const (
	kindNone valueKind = iota
	kindBool
	kindPermissionLevel
	kindAllowedActionsPolicy
	kindSelectedActionsCriteria
)

// BoolValue は、真偽値の設定の値を作る。
func BoolValue(b bool) Value {
	return Value{kind: kindBool, boolean: b}
}

// PermissionLevelValue は、ワークフローの GITHUB_TOKEN の既定権限の値を作る。
func PermissionLevelValue(level PermissionLevel) Value {
	return Value{kind: kindPermissionLevel, level: level}
}

// AllowedActionsPolicyValue は、Actions 許可ポリシーの値を作る。
func AllowedActionsPolicyValue(policy AllowedActionsPolicy) Value {
	return Value{kind: kindAllowedActionsPolicy, policy: policy}
}

// SelectedActionsCriteriaValue は、選択方式の Actions 許可条件の値を作る。
// 呼び出し側があとで書き換えても値が変わらないように、許可パターンを複製して持つ。
func SelectedActionsCriteriaValue(criteria SelectedActionsCriteria) Value {
	criteria.PatternsAllowed = slices.Clone(criteria.PatternsAllowed)
	return Value{kind: kindSelectedActionsCriteria, criteria: criteria}
}

// Equal は、二つの値が設定の意味で同じ値かどうかを返す。
// 値のないゼロ値は設定の値ではなく、比べようとするのは不変条件の破れのため panic する。
// 同じ値として扱うと、確かめていない値が一致や変更前提の成立として黙って通るため。
// 種別ごとに使わないフィールドはゼロ値のため、種別で分岐せずに全フィールドを比べる。
func (v Value) Equal(other Value) bool {
	if v.kind == kindNone || other.kind == kindNone {
		panic("setting: no value to compare")
	}
	return v.kind == other.kind &&
		v.boolean == other.boolean &&
		v.level == other.level &&
		v.policy == other.policy &&
		v.criteria.equal(other.criteria)
}

// String は、表示に使う表記で値を返す。値のないゼロ値を表示しようとするのは不変条件の破れのため panic する。
// fmt は %s に渡した値の String の panic を回復して出力に埋めるため、呼び出し側は String を明示的に呼ぶ。
func (v Value) String() string {
	switch v.kind {
	case kindBool:
		return strconv.FormatBool(v.boolean)
	case kindPermissionLevel:
		return string(v.level)
	case kindAllowedActionsPolicy:
		return string(v.policy)
	case kindSelectedActionsCriteria:
		return "github_owned_allowed: " + strconv.FormatBool(v.criteria.GitHubOwnedAllowed) +
			", verified_allowed: " + strconv.FormatBool(v.criteria.VerifiedAllowed) +
			", patterns_allowed: " + v.criteria.PatternsAllowedString()
	case kindNone:
	}
	panic("setting: no value to format")
}

// Bool は、真偽値の設定の値を返す。真偽値でない値は不変条件の破れのため panic する。
func (v Value) Bool() bool {
	if v.kind != kindBool {
		panic("setting: value is not a boolean")
	}
	return v.boolean
}

// PermissionLevel は、ワークフローの GITHUB_TOKEN の既定権限の値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) PermissionLevel() PermissionLevel {
	if v.kind != kindPermissionLevel {
		panic("setting: value is not a permission level")
	}
	return v.level
}

// AllowedActionsPolicy は、Actions 許可ポリシーの値を返す。それ以外の値は不変条件の破れのため panic する。
func (v Value) AllowedActionsPolicy() AllowedActionsPolicy {
	if v.kind != kindAllowedActionsPolicy {
		panic("setting: value is not an allowed actions policy")
	}
	return v.policy
}

// SelectedActionsCriteria は、選択方式の Actions 許可条件の値を返す。それ以外の値は不変条件の破れのため panic する。
// 戻り値の書き換えが値に及ばないように、許可パターンは複製を返す。
func (v Value) SelectedActionsCriteria() SelectedActionsCriteria {
	if v.kind != kindSelectedActionsCriteria {
		panic("setting: value is not selected actions criteria")
	}
	criteria := v.criteria
	criteria.PatternsAllowed = slices.Clone(criteria.PatternsAllowed)
	return criteria
}
