package changeplan

import (
	"strconv"

	"github.com/tmknom/reco/internal/setting"
)

// SettingValue は、一つの設定の期待値または観測値を、照合と表示のために同じ形で持つ。
// 真偽値の設定と列挙値の設定を一つの並びで扱うため、値の種別を持つ。
// ゼロ値は値がないことを表し、比較不能な変更予定の観測値に使う。
// 種別ごとに使わないフィールドをゼロ値に保つため、フィールドを公開せず、コンストラクタでだけ作る。
// 比較は == で行う。
type SettingValue struct {
	kind    valueKind
	boolean bool
	level   setting.PermissionLevel
}

type valueKind int

const (
	kindNone valueKind = iota
	kindBool
	kindPermissionLevel
)

// BoolValue は、真偽値の設定の値を作る。
func BoolValue(b bool) SettingValue {
	return SettingValue{kind: kindBool, boolean: b}
}

// PermissionLevelValue は、ワークフローの GITHUB_TOKEN の既定権限の値を作る。
func PermissionLevelValue(level setting.PermissionLevel) SettingValue {
	return SettingValue{kind: kindPermissionLevel, level: level}
}

// String は、YAML 設定仕様の表記で値を返す。値のないゼロ値を表示しようとするのは不変条件の破れのため panic する。
// fmt は %s に渡した値の String の panic を回復して出力に埋めるため、呼び出し側は String を明示的に呼ぶ。
func (v SettingValue) String() string {
	switch v.kind {
	case kindBool:
		return strconv.FormatBool(v.boolean)
	case kindPermissionLevel:
		return string(v.level)
	case kindNone:
	}
	panic("changeplan: no setting value to format")
}

// Bool は、真偽値の設定の値を返す。真偽値でない値は不変条件の破れのため panic する。
func (v SettingValue) Bool() bool {
	if v.kind != kindBool {
		panic("changeplan: setting value is not a boolean")
	}
	return v.boolean
}

// PermissionLevel は、ワークフローの GITHUB_TOKEN の既定権限の値を返す。それ以外の値は不変条件の破れのため panic する。
func (v SettingValue) PermissionLevel() setting.PermissionLevel {
	if v.kind != kindPermissionLevel {
		panic("changeplan: setting value is not a permission level")
	}
	return v.level
}
