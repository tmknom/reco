package setting

import "strconv"

// Value は、一つの設定の値。期待値と観測値のどちらも、同じ設定についてはこの値で表し、== で比べる。
// 真偽値の設定と列挙値の設定を一つの並びで扱うため、値の種別を持つ。
// ゼロ値は値がないことを表す。
// 種別ごとに使わないフィールドをゼロ値に保つため、フィールドを公開せず、コンストラクタでだけ作る。
type Value struct {
	kind    valueKind
	boolean bool
	level   PermissionLevel
}

type valueKind int

const (
	kindNone valueKind = iota
	kindBool
	kindPermissionLevel
)

// BoolValue は、真偽値の設定の値を作る。
func BoolValue(b bool) Value {
	return Value{kind: kindBool, boolean: b}
}

// PermissionLevelValue は、ワークフローの GITHUB_TOKEN の既定権限の値を作る。
func PermissionLevelValue(level PermissionLevel) Value {
	return Value{kind: kindPermissionLevel, level: level}
}

// String は、YAML 設定仕様の表記で値を返す。値のないゼロ値を表示しようとするのは不変条件の破れのため panic する。
// fmt は %s に渡した値の String の panic を回復して出力に埋めるため、呼び出し側は String を明示的に呼ぶ。
func (v Value) String() string {
	switch v.kind {
	case kindBool:
		return strconv.FormatBool(v.boolean)
	case kindPermissionLevel:
		return string(v.level)
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
